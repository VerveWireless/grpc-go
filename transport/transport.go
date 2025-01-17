/*
 *
 * Copyright 2014, Google Inc.
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are
 * met:
 *
 *     * Redistributions of source code must retain the above copyright
 * notice, this list of conditions and the following disclaimer.
 *     * Redistributions in binary form must reproduce the above
 * copyright notice, this list of conditions and the following disclaimer
 * in the documentation and/or other materials provided with the
 * distribution.
 *     * Neither the name of Google Inc. nor the names of its
 * contributors may be used to endorse or promote products derived from
 * this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 * A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 * OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 * SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 *
 */

/*
Package transport defines and implements message oriented communication channel
to complete various transactions (e.g., an RPC).
*/
package transport // import "github.com/VerveWireless/grpc-go/transport"

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/trace"
	"github.com/VerveWireless/grpc-go/codes"
	"github.com/VerveWireless/grpc-go/credentials"
	"github.com/VerveWireless/grpc-go/metadata"
)

// recvMsg represents the received msg from the transport. All transport
// protocol specific info has been removed.
type recvMsg struct {
	data []byte
	// nil: received some data
	// io.EOF: stream is completed. data is nil.
	// other non-nil error: transport failure. data is nil.
	err error
}

func (recvMsg) isItem() bool {
	return true
}

// All items in an out of a recvBuffer should be the same type.
type item interface {
	isItem() bool
}

// recvBuffer is an unbounded channel of item.
type recvBuffer struct {
	c       chan item
	mu      sync.Mutex
	backlog []item
}

func newRecvBuffer() *recvBuffer {
	b := &recvBuffer{
		c: make(chan item, 1),
	}
	return b
}

func (b *recvBuffer) put(r item) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.backlog = append(b.backlog, r)
	select {
	case b.c <- b.backlog[0]:
		b.backlog = b.backlog[1:]
	default:
	}
}

func (b *recvBuffer) load() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.backlog) > 0 {
		select {
		case b.c <- b.backlog[0]:
			b.backlog = b.backlog[1:]
		default:
		}
	}
}

// get returns the channel that receives an item in the buffer.
//
// Upon receipt of an item, the caller should call load to send another
// item onto the channel if there is any.
func (b *recvBuffer) get() <-chan item {
	return b.c
}

// recvBufferReader implements io.Reader interface to read the data from
// recvBuffer.
type recvBufferReader struct {
	ctx  context.Context
	recv *recvBuffer
	last *bytes.Reader // Stores the remaining data in the previous calls.
	err  error
}

// Read reads the next len(p) bytes from last. If last is drained, it tries to
// read additional data from recv. It blocks if there no additional data available
// in recv. If Read returns any non-nil error, it will continue to return that error.
func (r *recvBufferReader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	defer func() { r.err = err }()
	if r.last != nil && r.last.Len() > 0 {
		// Read remaining data left in last call.
		return r.last.Read(p)
	}
	select {
	case <-r.ctx.Done():
		return 0, ContextErr(r.ctx.Err())
	case i := <-r.recv.get():
		r.recv.load()
		m := i.(*recvMsg)
		if m.err != nil {
			return 0, m.err
		}
		r.last = bytes.NewReader(m.data)
		return r.last.Read(p)
	}
}

type streamState uint8

const (
	streamActive    streamState = iota
	streamWriteDone             // EndStream sent
	streamReadDone              // EndStream received
	streamDone                  // sendDone and recvDone or RSTStreamFrame is sent or received.
)

// Stream represents an RPC in the transport layer.
type Stream struct {
	id uint32
	// nil for client side Stream.
	st ServerTransport
	// ctx is the associated context of the stream.
	ctx    context.Context
	cancel context.CancelFunc
	// method records the associated RPC method of the stream.
	method       string
	recvCompress string
	sendCompress string
	buf          *recvBuffer
	dec          io.Reader
	fc           *inFlow
	recvQuota    uint32
	// The accumulated inbound quota pending for window update.
	updateQuota uint32
	// The handler to control the window update procedure for both this
	// particular stream and the associated transport.
	windowHandler func(int)

	sendQuotaPool *quotaPool
	// Close headerChan to indicate the end of reception of header metadata.
	headerChan chan struct{}
	// header caches the received header metadata.
	header metadata.MD
	// The key-value map of trailer metadata.
	trailer metadata.MD

	mu sync.RWMutex // guard the following
	// headerOK becomes true from the first header is about to send.
	headerOk bool
	state    streamState
	// true iff headerChan is closed. Used to avoid closing headerChan
	// multiple times.
	headerDone bool
	// the status received from the server.
	statusCode codes.Code
	statusDesc string
}

// RecvCompress returns the compression algorithm applied to the inbound
// message. It is empty string if there is no compression applied.
func (s *Stream) RecvCompress() string {
	return s.recvCompress
}

// SetSendCompress sets the compression algorithm to the stream.
func (s *Stream) SetSendCompress(str string) {
	s.sendCompress = str
}

// Header acquires the key-value pairs of header metadata once it
// is available. It blocks until i) the metadata is ready or ii) there is no
// header metadata or iii) the stream is cancelled/expired.
func (s *Stream) Header() (metadata.MD, error) {
	select {
	case <-s.ctx.Done():
		return nil, ContextErr(s.ctx.Err())
	case <-s.headerChan:
		return s.header.Copy(), nil
	}
}

// Trailer returns the cached trailer metedata. Note that if it is not called
// after the entire stream is done, it could return an empty MD. Client
// side only.
func (s *Stream) Trailer() metadata.MD {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.trailer.Copy()
}

// ServerTransport returns the underlying ServerTransport for the stream.
// The client side stream always returns nil.
func (s *Stream) ServerTransport() ServerTransport {
	return s.st
}

// Context returns the context of the stream.
func (s *Stream) Context() context.Context {
	return s.ctx
}

// TraceContext recreates the context of s with a trace.Trace.
func (s *Stream) TraceContext(tr trace.Trace) {
	s.ctx = trace.NewContext(s.ctx, tr)
}

// Method returns the method for the stream.
func (s *Stream) Method() string {
	return s.method
}

// StatusCode returns statusCode received from the server.
func (s *Stream) StatusCode() codes.Code {
	return s.statusCode
}

// StatusDesc returns statusDesc received from the server.
func (s *Stream) StatusDesc() string {
	return s.statusDesc
}

// ErrIllegalTrailerSet indicates that the trailer has already been set or it
// is too late to do so.
var ErrIllegalTrailerSet = errors.New("transport: trailer has been set")

// SetTrailer sets the trailer metadata which will be sent with the RPC status
// by the server. This can only be called at most once. Server side only.
func (s *Stream) SetTrailer(md metadata.MD) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.trailer != nil {
		return ErrIllegalTrailerSet
	}
	s.trailer = md.Copy()
	return nil
}

func (s *Stream) write(m recvMsg) {
	s.buf.put(&m)
}

// Read reads all the data available for this Stream from the transport and
// passes them into the decoder, which converts them into a gRPC message stream.
// The error is io.EOF when the stream is done or another non-nil error if
// the stream broke.
func (s *Stream) Read(p []byte) (n int, err error) {
	n, err = s.dec.Read(p)
	if err != nil {
		return
	}
	s.windowHandler(n)
	return
}

// The key to save transport.Stream in the context.
type streamKey struct{}

// newContextWithStream creates a new context from ctx and attaches stream
// to it.
func newContextWithStream(ctx context.Context, stream *Stream) context.Context {
	return context.WithValue(ctx, streamKey{}, stream)
}

// StreamFromContext returns the stream saved in ctx.
func StreamFromContext(ctx context.Context) (s *Stream, ok bool) {
	s, ok = ctx.Value(streamKey{}).(*Stream)
	return
}

// state of transport
type transportState int

const (
	reachable transportState = iota
	unreachable
	closing
)

// NewServerTransport creates a ServerTransport with conn or non-nil error
// if it fails.
func NewServerTransport(protocol string, conn net.Conn, maxStreams uint32, authInfo credentials.AuthInfo) (ServerTransport, error) {
	return newHTTP2Server(conn, maxStreams, authInfo)
}

// ConnectOptions covers all relevant options for dialing a server.
type ConnectOptions struct {
	// UserAgent is the application user agent.
	UserAgent string
	// Dialer specifies how to dial a network address.
	Dialer func(string, time.Duration) (net.Conn, error)
	// AuthOptions stores the credentials required to setup a client connection and/or issue RPCs.
	AuthOptions []credentials.Credentials
	// Timeout specifies the timeout for dialing a client connection.
	Timeout time.Duration
}

// NewClientTransport establishes the transport with the required ConnectOptions
// and returns it to the caller.
func NewClientTransport(target string, opts *ConnectOptions) (ClientTransport, error) {
	return newHTTP2Client(target, opts)
}

// Options provides additional hints and information for message
// transmission.
type Options struct {
	// Last indicates whether this write is the last piece for
	// this stream.
	Last bool

	// Delay is a hint to the transport implementation for whether
	// the data could be buffered for a batching write. The
	// Transport implementation may ignore the hint.
	Delay bool
}

// CallHdr carries the information of a particular RPC.
type CallHdr struct {
	// Host specifies the peer's host.
	Host string

	// Method specifies the operation to perform.
	Method string

	// RecvCompress specifies the compression algorithm applied on
	// inbound messages.
	RecvCompress string

	// SendCompress specifies the compression algorithm applied on
	// outbound message.
	SendCompress string

	// Flush indicates whether a new stream command should be sent
	// to the peer without waiting for the first data. This is
	// only a hint. The transport may modify the flush decision
	// for performance purposes.
	Flush bool
}

// ClientTransport is the common interface for all gRPC client-side transport
// implementations.
type ClientTransport interface {
	// Close tears down this transport. Once it returns, the transport
	// should not be accessed any more. The caller must make sure this
	// is called only once.
	Close() error

	// Write sends the data for the given stream. A nil stream indicates
	// the write is to be performed on the transport as a whole.
	Write(s *Stream, data []byte, opts *Options) error

	// NewStream creates a Stream for an RPC.
	NewStream(ctx context.Context, callHdr *CallHdr) (*Stream, error)

	// CloseStream clears the footprint of a stream when the stream is
	// not needed any more. The err indicates the error incurred when
	// CloseStream is called. Must be called when a stream is finished
	// unless the associated transport is closing.
	CloseStream(stream *Stream, err error)

	// Error returns a channel that is closed when some I/O error
	// happens. Typically the caller should have a goroutine to monitor
	// this in order to take action (e.g., close the current transport
	// and create a new one) in error case. It should not return nil
	// once the transport is initiated.
	Error() <-chan struct{}
}

// ServerTransport is the common interface for all gRPC server-side transport
// implementations.
//
// Methods may be called concurrently from multiple goroutines, but
// Write methods for a given Stream will be called serially.
type ServerTransport interface {
	// HandleStreams receives incoming streams using the given handler.
	HandleStreams(func(*Stream))

	// WriteHeader sends the header metadata for the given stream.
	// WriteHeader may not be called on all streams.
	WriteHeader(s *Stream, md metadata.MD) error

	// Write sends the data for the given stream.
	// Write may not be called on all streams.
	Write(s *Stream, data []byte, opts *Options) error

	// WriteStatus sends the status of a stream to the client.
	// WriteStatus is the final call made on a stream and always
	// occurs.
	WriteStatus(s *Stream, statusCode codes.Code, statusDesc string) error

	// Close tears down the transport. Once it is called, the transport
	// should not be accessed any more. All the pending streams and their
	// handlers will be terminated asynchronously.
	Close() error

	// RemoteAddr returns the remote network address.
	RemoteAddr() net.Addr
}

// StreamErrorf creates an StreamError with the specified error code and description.
func StreamErrorf(c codes.Code, format string, a ...interface{}) StreamError {
	return StreamError{
		Code: c,
		Desc: fmt.Sprintf(format, a...),
	}
}

// ConnectionErrorf creates an ConnectionError with the specified error description.
func ConnectionErrorf(format string, a ...interface{}) ConnectionError {
	return ConnectionError{
		Desc: fmt.Sprintf(format, a...),
	}
}

// ConnectionError is an error that results in the termination of the
// entire connection and the retry of all the active streams.
type ConnectionError struct {
	Desc string
}

func (e ConnectionError) Error() string {
	return fmt.Sprintf("connection error: desc = %q", e.Desc)
}

// Define some common ConnectionErrors.
var ErrConnClosing = ConnectionError{Desc: "transport is closing"}

// StreamError is an error that only affects one stream within a connection.
type StreamError struct {
	Code codes.Code
	Desc string
}

func (e StreamError) Error() string {
	return fmt.Sprintf("stream error: code = %d desc = %q", e.Code, e.Desc)
}

// ContextErr converts the error from context package into a StreamError.
func ContextErr(err error) StreamError {
	switch err {
	case context.DeadlineExceeded:
		return StreamErrorf(codes.DeadlineExceeded, "%v", err)
	case context.Canceled:
		return StreamErrorf(codes.Canceled, "%v", err)
	}
	panic(fmt.Sprintf("Unexpected error from context packet: %v", err))
}

// wait blocks until it can receive from ctx.Done, closing, or proceed.
// If it receives from ctx.Done, it returns 0, the StreamError for ctx.Err.
// If it receives from closing, it returns 0, ErrConnClosing.
// If it receives from proceed, it returns the received integer, nil.
func wait(ctx context.Context, closing <-chan struct{}, proceed <-chan int) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ContextErr(ctx.Err())
	case <-closing:
		return 0, ErrConnClosing
	case i := <-proceed:
		return i, nil
	}
}
