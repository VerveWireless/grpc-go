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

package grpc_test

import (
	"bytes"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"golang.org/x/net/http2"
	"github.com/VerveWireless/grpc-go"
	"github.com/VerveWireless/grpc-go/codes"
	"github.com/VerveWireless/grpc-go/credentials"
	"github.com/VerveWireless/grpc-go/grpclog"
	"github.com/VerveWireless/grpc-go/health"
	healthpb "github.com/VerveWireless/grpc-go/health/grpc_health_v1"
	"github.com/VerveWireless/grpc-go/internal"
	"github.com/VerveWireless/grpc-go/metadata"
	"github.com/VerveWireless/grpc-go/peer"
	testpb "github.com/VerveWireless/grpc-go/test/grpc_testing"
	"github.com/VerveWireless/grpc-go/transport"
)

var (
	// For headers:
	testMetadata = metadata.MD{
		"key1": []string{"value1"},
		"key2": []string{"value2"},
	}
	// For trailers:
	testTrailerMetadata = metadata.MD{
		"tkey1": []string{"trailerValue1"},
		"tkey2": []string{"trailerValue2"},
	}
	testAppUA = "myApp1/1.0 myApp2/0.9"
)

var raceMode bool // set by race_test.go in race mode

type testServer struct {
	security string // indicate the authentication protocol used by this server.
}

func (s *testServer) EmptyCall(ctx context.Context, in *testpb.Empty) (*testpb.Empty, error) {
	if md, ok := metadata.FromContext(ctx); ok {
		// For testing purpose, returns an error if there is attached metadata other than
		// the user agent set by the client application.
		if _, ok := md["user-agent"]; !ok {
			return nil, grpc.Errorf(codes.DataLoss, "missing expected user-agent")
		}
		var str []string
		for _, entry := range md["user-agent"] {
			str = append(str, "ua", entry)
		}
		grpc.SendHeader(ctx, metadata.Pairs(str...))
	}
	return new(testpb.Empty), nil
}

func newPayload(t testpb.PayloadType, size int32) (*testpb.Payload, error) {
	if size < 0 {
		return nil, fmt.Errorf("Requested a response with invalid length %d", size)
	}
	body := make([]byte, size)
	switch t {
	case testpb.PayloadType_COMPRESSABLE:
	case testpb.PayloadType_UNCOMPRESSABLE:
		return nil, fmt.Errorf("PayloadType UNCOMPRESSABLE is not supported")
	default:
		return nil, fmt.Errorf("Unsupported payload type: %d", t)
	}
	return &testpb.Payload{
		Type: t.Enum(),
		Body: body,
	}, nil
}

func (s *testServer) UnaryCall(ctx context.Context, in *testpb.SimpleRequest) (*testpb.SimpleResponse, error) {
	md, ok := metadata.FromContext(ctx)
	if ok {
		if err := grpc.SendHeader(ctx, md); err != nil {
			return nil, fmt.Errorf("grpc.SendHeader(%v, %v) = %v, want %v", ctx, md, err, nil)
		}
		grpc.SetTrailer(ctx, testTrailerMetadata)
	}
	pr, ok := peer.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("failed to get peer from ctx")
	}
	if pr.Addr == net.Addr(nil) {
		return nil, fmt.Errorf("failed to get peer address")
	}
	if s.security != "" {
		// Check Auth info
		var authType, serverName string
		switch info := pr.AuthInfo.(type) {
		case credentials.TLSInfo:
			authType = info.AuthType()
			serverName = info.State.ServerName
		default:
			return nil, fmt.Errorf("Unknown AuthInfo type")
		}
		if authType != s.security {
			return nil, fmt.Errorf("Wrong auth type: got %q, want %q", authType, s.security)
		}
		if serverName != "x.test.youtube.com" {
			return nil, fmt.Errorf("Unknown server name %q", serverName)
		}
	}

	// Simulate some service delay.
	time.Sleep(time.Second)

	payload, err := newPayload(in.GetResponseType(), in.GetResponseSize())
	if err != nil {
		return nil, err
	}
	return &testpb.SimpleResponse{
		Payload: payload,
	}, nil
}

func (s *testServer) StreamingOutputCall(args *testpb.StreamingOutputCallRequest, stream testpb.TestService_StreamingOutputCallServer) error {
	if md, ok := metadata.FromContext(stream.Context()); ok {
		// For testing purpose, returns an error if there is attached metadata.
		if len(md) > 0 {
			return grpc.Errorf(codes.DataLoss, "got extra metadata")
		}
	}
	cs := args.GetResponseParameters()
	for _, c := range cs {
		if us := c.GetIntervalUs(); us > 0 {
			time.Sleep(time.Duration(us) * time.Microsecond)
		}

		payload, err := newPayload(args.GetResponseType(), c.GetSize())
		if err != nil {
			return err
		}

		if err := stream.Send(&testpb.StreamingOutputCallResponse{
			Payload: payload,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *testServer) StreamingInputCall(stream testpb.TestService_StreamingInputCallServer) error {
	var sum int
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&testpb.StreamingInputCallResponse{
				AggregatedPayloadSize: proto.Int32(int32(sum)),
			})
		}
		if err != nil {
			return err
		}
		p := in.GetPayload().GetBody()
		sum += len(p)
	}
}

func (s *testServer) FullDuplexCall(stream testpb.TestService_FullDuplexCallServer) error {
	md, ok := metadata.FromContext(stream.Context())
	if ok {
		if err := stream.SendHeader(md); err != nil {
			return fmt.Errorf("%v.SendHeader(%v) = %v, want %v", stream, md, err, nil)
		}
		stream.SetTrailer(md)
	}
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			// read done.
			return nil
		}
		if err != nil {
			return err
		}
		cs := in.GetResponseParameters()
		for _, c := range cs {
			if us := c.GetIntervalUs(); us > 0 {
				time.Sleep(time.Duration(us) * time.Microsecond)
			}

			payload, err := newPayload(in.GetResponseType(), c.GetSize())
			if err != nil {
				return err
			}

			if err := stream.Send(&testpb.StreamingOutputCallResponse{
				Payload: payload,
			}); err != nil {
				return err
			}
		}
	}
}

func (s *testServer) HalfDuplexCall(stream testpb.TestService_HalfDuplexCallServer) error {
	var msgBuf []*testpb.StreamingOutputCallRequest
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			// read done.
			break
		}
		if err != nil {
			return err
		}
		msgBuf = append(msgBuf, in)
	}
	for _, m := range msgBuf {
		cs := m.GetResponseParameters()
		for _, c := range cs {
			if us := c.GetIntervalUs(); us > 0 {
				time.Sleep(time.Duration(us) * time.Microsecond)
			}

			payload, err := newPayload(m.GetResponseType(), c.GetSize())
			if err != nil {
				return err
			}

			if err := stream.Send(&testpb.StreamingOutputCallResponse{
				Payload: payload,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

const tlsDir = "testdata/"

func TestReconnectTimeout(t *testing.T) {
	defer leakCheck(t)()
	restore := declareLogNoise(t,
		"transport: http2Client.notifyError got notified that the client transport was broken",
		"grpc: Conn.resetTransport failed to create client transport: connection error: desc = \"transport",
		"grpc: Conn.transportMonitor exits due to: grpc: timed out trying to connect",
	)
	defer restore()

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	_, port, err := net.SplitHostPort(lis.Addr().String())
	if err != nil {
		t.Fatalf("Failed to parse listener address: %v", err)
	}
	addr := "localhost:" + port
	conn, err := grpc.Dial(addr, grpc.WithTimeout(5*time.Second), grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial to the server %q: %v", addr, err)
	}
	// Close unaccepted connection (i.e., conn).
	lis.Close()
	tc := testpb.NewTestServiceClient(conn)
	waitC := make(chan struct{})
	go func() {
		defer close(waitC)
		const argSize = 271828
		const respSize = 314159

		payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
		if err != nil {
			t.Error(err)
			return
		}

		req := &testpb.SimpleRequest{
			ResponseType: testpb.PayloadType_COMPRESSABLE.Enum(),
			ResponseSize: proto.Int32(respSize),
			Payload:      payload,
		}
		if _, err := tc.UnaryCall(context.Background(), req); err == nil {
			t.Errorf("TestService/UnaryCall(_, _) = _, <nil>, want _, non-nil")
			return
		}
	}()
	// Block until reconnect times out.
	<-waitC
	if err := conn.Close(); err != grpc.ErrClientConnClosing {
		t.Fatalf("%v.Close() = %v, want %v", conn, err, grpc.ErrClientConnClosing)
	}
}

func unixDialer(addr string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", addr, timeout)
}

type env struct {
	name        string
	network     string // The type of network such as tcp, unix, etc.
	dialer      func(addr string, timeout time.Duration) (net.Conn, error)
	security    string // The security protocol such as TLS, SSH, etc.
	httpHandler bool   // whether to use the http.Handler ServerTransport; requires TLS
}

func (e env) runnable() bool {
	if runtime.GOOS == "windows" && strings.HasPrefix(e.name, "unix-") {
		return false
	}
	return true
}

func (e env) getDialer() func(addr string, timeout time.Duration) (net.Conn, error) {
	if e.dialer != nil {
		return e.dialer
	}
	return func(addr string, timeout time.Duration) (net.Conn, error) {
		return net.DialTimeout("tcp", addr, timeout)
	}
}

var (
	tcpClearEnv  = env{name: "tcp-clear", network: "tcp"}
	tcpTLSEnv    = env{name: "tcp-tls", network: "tcp", security: "tls"}
	unixClearEnv = env{name: "unix-clear", network: "unix", dialer: unixDialer}
	unixTLSEnv   = env{name: "unix-tls", network: "unix", dialer: unixDialer, security: "tls"}
	handlerEnv   = env{name: "handler-tls", network: "tcp", security: "tls", httpHandler: true}
	allEnv       = []env{tcpClearEnv, tcpTLSEnv, unixClearEnv, unixTLSEnv, handlerEnv}
)

var onlyEnv = flag.String("only_env", "", "If non-empty, one of 'tcp-clear', 'tcp-tls', 'unix-clear', 'unix-tls', or 'handler-tls' to only run the tests for that environment. Empty means all.")

func listTestEnv() (envs []env) {
	if *onlyEnv != "" {
		for _, e := range allEnv {
			if e.name == *onlyEnv {
				if !e.runnable() {
					panic(fmt.Sprintf("--only_env environment %q does not run on %s", *onlyEnv, runtime.GOOS))
				}
				return []env{e}
			}
		}
		panic(fmt.Sprintf("invalid --only_env value %q", *onlyEnv))
	}
	for _, e := range allEnv {
		if e.runnable() {
			envs = append(envs, e)
		}
	}
	return envs
}

// test is an end-to-end test. It should be created with the newTest
// func, modified as needed, and then started with its startServer method.
// It should be cleaned up with the tearDown method.
type test struct {
	t *testing.T
	e env

	ctx    context.Context // valid for life of test, before tearDown
	cancel context.CancelFunc

	// Configurable knobs, after newTest returns:
	testServer        testpb.TestServiceServer // nil means none
	healthServer      *health.HealthServer     // nil means disabled
	maxStream         uint32
	userAgent         string
	clientCompression bool
	serverCompression bool

	// srv and srvAddr are set once startServer is called.
	srv     *grpc.Server
	srvAddr string

	cc          *grpc.ClientConn // nil until requested via clientConn
	restoreLogs func()           // nil unless declareLogNoise is used
}

func (te *test) tearDown() {
	if te.cancel != nil {
		te.cancel()
	}
	te.srv.Stop()
	if te.cc != nil {
		te.cc.Close()
	}
	if te.restoreLogs != nil {
		te.restoreLogs()
	}
}

// newTest returns a new test using the provided testing.T and
// environment.  It is returned with default values. Tests should
// modify it before calling its startServer and clientConn methods.
func newTest(t *testing.T, e env) *test {
	te := &test{
		t:          t,
		e:          e,
		testServer: &testServer{security: e.security},
		maxStream:  math.MaxUint32,
	}
	te.ctx, te.cancel = context.WithCancel(context.Background())
	return te
}

// startServer starts a gRPC server listening. Callers should defer a
// call to te.tearDown to clean up.
func (te *test) startServer() {
	e := te.e
	te.t.Logf("Running test in %s environment...", e.name)
	sopts := []grpc.ServerOption{grpc.MaxConcurrentStreams(te.maxStream)}
	if te.serverCompression {
		sopts = append(sopts,
			grpc.RPCCompressor(grpc.NewGZIPCompressor()),
			grpc.RPCDecompressor(grpc.NewGZIPDecompressor()),
		)
	}

	la := "localhost:0"
	switch e.network {
	case "unix":
		la = "/tmp/testsock" + fmt.Sprintf("%d", time.Now())
		syscall.Unlink(la)
	}
	lis, err := net.Listen(e.network, la)
	if err != nil {
		te.t.Fatalf("Failed to listen: %v", err)
	}
	if te.e.security == "tls" {
		creds, err := credentials.NewServerTLSFromFile(tlsDir+"server1.pem", tlsDir+"server1.key")
		if err != nil {
			te.t.Fatalf("Failed to generate credentials %v", err)
		}
		sopts = append(sopts, grpc.Creds(creds))
	}
	s := grpc.NewServer(sopts...)
	te.srv = s
	if e.httpHandler {
		internal.TestingUseHandlerImpl(s)
	}
	if te.healthServer != nil {
		healthpb.RegisterHealthServer(s, te.healthServer)
	}
	if te.testServer != nil {
		testpb.RegisterTestServiceServer(s, te.testServer)
	}
	addr := la
	switch e.network {
	case "unix":
	default:
		_, port, err := net.SplitHostPort(lis.Addr().String())
		if err != nil {
			te.t.Fatalf("Failed to parse listener address: %v", err)
		}
		addr = "localhost:" + port
	}

	go s.Serve(lis)
	te.srvAddr = addr
}

func (te *test) clientConn() *grpc.ClientConn {
	if te.cc != nil {
		return te.cc
	}
	opts := []grpc.DialOption{
		grpc.WithDialer(te.e.dialer),
		grpc.WithUserAgent(te.userAgent),
	}

	if te.clientCompression {
		opts = append(opts,
			grpc.WithCompressor(grpc.NewGZIPCompressor()),
			grpc.WithDecompressor(grpc.NewGZIPDecompressor()),
		)
	}
	if te.e.security == "tls" {
		creds, err := credentials.NewClientTLSFromFile(tlsDir+"ca.pem", "x.test.youtube.com")
		if err != nil {
			te.t.Fatalf("Failed to load credentials: %v", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}
	var err error
	te.cc, err = grpc.Dial(te.srvAddr, opts...)
	if err != nil {
		te.t.Fatalf("Dial(%q) = %v", te.srvAddr, err)
	}
	return te.cc
}

func (te *test) declareLogNoise(phrases ...string) {
	te.restoreLogs = declareLogNoise(te.t, phrases...)
}

func (te *test) withServerTester(fn func(st *serverTester)) {
	var c net.Conn
	var err error
	c, err = te.e.getDialer()(te.srvAddr, 10*time.Second)
	if err != nil {
		te.t.Fatal(err)
	}
	defer c.Close()
	if te.e.security == "tls" {
		c = tls.Client(c, &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{http2.NextProtoTLS},
		})
	}
	st := newServerTesterFromConn(te.t, c)
	st.greet()
	fn(st)
}

func TestTimeoutOnDeadServer(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testTimeoutOnDeadServer(t, e)
	}
}

func testTimeoutOnDeadServer(t *testing.T, e env) {
	te := newTest(t, e)
	te.declareLogNoise(
		"transport: http2Client.notifyError got notified that the client transport was broken EOF",
		"grpc: Conn.transportMonitor exits due to: grpc: the client connection is closing",
		"grpc: Conn.resetTransport failed to create client transport: connection error",
		"grpc: Conn.resetTransport failed to create client transport: connection error: desc = \"transport: dial unix",
	)
	te.startServer()
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)
	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	if _, err := cc.WaitForStateChange(ctx, grpc.Idle); err != nil {
		t.Fatalf("cc.WaitForStateChange(_, %s) = _, %v, want _, <nil>", grpc.Idle, err)
	}
	ctx, _ = context.WithTimeout(context.Background(), time.Second)
	if _, err := cc.WaitForStateChange(ctx, grpc.Connecting); err != nil {
		t.Fatalf("cc.WaitForStateChange(_, %s) = _, %v, want _, <nil>", grpc.Connecting, err)
	}
	if state, err := cc.State(); err != nil || state != grpc.Ready {
		t.Fatalf("cc.State() = %s, %v, want %s, <nil>", state, err, grpc.Ready)
	}
	ctx, _ = context.WithTimeout(context.Background(), time.Second)
	if _, err := cc.WaitForStateChange(ctx, grpc.Ready); err != context.DeadlineExceeded {
		t.Fatalf("cc.WaitForStateChange(_, %s) = _, %v, want _, %v", grpc.Ready, err, context.DeadlineExceeded)
	}
	te.srv.Stop()
	// Set -1 as the timeout to make sure if transportMonitor gets error
	// notification in time the failure path of the 1st invoke of
	// ClientConn.wait hits the deadline exceeded error.
	ctx, _ = context.WithTimeout(context.Background(), -1)
	if _, err := tc.EmptyCall(ctx, &testpb.Empty{}); grpc.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/EmptyCall(%v, _) = _, error %v, want _, error code: %d", ctx, err, codes.DeadlineExceeded)
	}
	ctx, _ = context.WithTimeout(context.Background(), time.Second)
	if _, err := cc.WaitForStateChange(ctx, grpc.Ready); err != nil {
		t.Fatalf("cc.WaitForStateChange(_, %s) = _, %v, want _, <nil>", grpc.Ready, err)
	}
	if state, err := cc.State(); err != nil || (state != grpc.Connecting && state != grpc.TransientFailure) {
		t.Fatalf("cc.State() = %s, %v, want %s or %s, <nil>", state, err, grpc.Connecting, grpc.TransientFailure)
	}
	cc.Close()
	awaitNewConnLogOutput()
}

func healthCheck(d time.Duration, cc *grpc.ClientConn, serviceName string) (*healthpb.HealthCheckResponse, error) {
	ctx, _ := context.WithTimeout(context.Background(), d)
	hc := healthpb.NewHealthClient(cc)
	req := &healthpb.HealthCheckRequest{
		Service: serviceName,
	}
	return hc.Check(ctx, req)
}

func TestHealthCheckOnSuccess(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testHealthCheckOnSuccess(t, e)
	}
}

func testHealthCheckOnSuccess(t *testing.T, e env) {
	te := newTest(t, e)
	hs := health.NewHealthServer()
	hs.SetServingStatus("grpc.health.v1.Health", 1)
	te.healthServer = hs
	te.startServer()
	defer te.tearDown()

	cc := te.clientConn()
	if _, err := healthCheck(1*time.Second, cc, "grpc.health.v1.Health"); err != nil {
		t.Fatalf("Health/Check(_, _) = _, %v, want _, <nil>", err)
	}
}

func TestHealthCheckOnFailure(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testHealthCheckOnFailure(t, e)
	}
}

func testHealthCheckOnFailure(t *testing.T, e env) {
	defer leakCheck(t)()
	te := newTest(t, e)
	te.declareLogNoise(
		"Failed to dial ",
		"grpc: the client connection is closing; please retry",
	)
	hs := health.NewHealthServer()
	hs.SetServingStatus("grpc.health.v1.HealthCheck", 1)
	te.healthServer = hs
	te.startServer()
	defer te.tearDown()

	cc := te.clientConn()
	if _, err := healthCheck(0*time.Second, cc, "grpc.health.v1.Health"); err != grpc.Errorf(codes.DeadlineExceeded, "context deadline exceeded") {
		t.Fatalf("Health/Check(_, _) = _, %v, want _, error code %d", err, codes.DeadlineExceeded)
	}
	awaitNewConnLogOutput()
}

func TestHealthCheckOff(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testHealthCheckOff(t, e)
	}
}

func testHealthCheckOff(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer()
	defer te.tearDown()
	want := grpc.Errorf(codes.Unimplemented, "unknown service grpc.health.v1.Health")
	if _, err := healthCheck(1*time.Second, te.clientConn(), ""); err != want {
		t.Fatalf("Health/Check(_, _) = _, %v, want _, error %v", err, want)
	}
}

func TestHealthCheckServingStatus(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testHealthCheckServingStatus(t, e)
	}
}

func testHealthCheckServingStatus(t *testing.T, e env) {
	te := newTest(t, e)
	hs := health.NewHealthServer()
	te.healthServer = hs
	te.startServer()
	defer te.tearDown()

	cc := te.clientConn()
	out, err := healthCheck(1*time.Second, cc, "")
	if err != nil {
		t.Fatalf("Health/Check(_, _) = _, %v, want _, <nil>", err)
	}
	if out.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("Got the serving status %v, want SERVING", out.Status)
	}
	if _, err := healthCheck(1*time.Second, cc, "grpc.health.v1.Health"); err != grpc.Errorf(codes.NotFound, "unknown service") {
		t.Fatalf("Health/Check(_, _) = _, %v, want _, error code %d", err, codes.NotFound)
	}
	hs.SetServingStatus("grpc.health.v1.Health", healthpb.HealthCheckResponse_SERVING)
	out, err = healthCheck(1*time.Second, cc, "grpc.health.v1.Health")
	if err != nil {
		t.Fatalf("Health/Check(_, _) = _, %v, want _, <nil>", err)
	}
	if out.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("Got the serving status %v, want SERVING", out.Status)
	}
	hs.SetServingStatus("grpc.health.v1.Health", healthpb.HealthCheckResponse_NOT_SERVING)
	out, err = healthCheck(1*time.Second, cc, "grpc.health.v1.Health")
	if err != nil {
		t.Fatalf("Health/Check(_, _) = _, %v, want _, <nil>", err)
	}
	if out.Status != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("Got the serving status %v, want NOT_SERVING", out.Status)
	}

}

func TestEmptyUnaryWithUserAgent(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testEmptyUnaryWithUserAgent(t, e)
	}
}

func testEmptyUnaryWithUserAgent(t *testing.T, e env) {
	te := newTest(t, e)
	te.userAgent = testAppUA
	te.startServer()
	defer te.tearDown()

	cc := te.clientConn()

	// Wait until cc is connected.
	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	if _, err := cc.WaitForStateChange(ctx, grpc.Idle); err != nil {
		t.Fatalf("cc.WaitForStateChange(_, %s) = _, %v, want _, <nil>", grpc.Idle, err)
	}
	ctx, _ = context.WithTimeout(context.Background(), time.Second)
	if _, err := cc.WaitForStateChange(ctx, grpc.Connecting); err != nil {
		t.Fatalf("cc.WaitForStateChange(_, %s) = _, %v, want _, <nil>", grpc.Connecting, err)
	}
	if state, err := cc.State(); err != nil || state != grpc.Ready {
		t.Fatalf("cc.State() = %s, %v, want %s, <nil>", state, err, grpc.Ready)
	}
	ctx, _ = context.WithTimeout(context.Background(), time.Second)
	if _, err := cc.WaitForStateChange(ctx, grpc.Ready); err == nil {
		t.Fatalf("cc.WaitForStateChange(_, %s) = _, <nil>, want _, %v", grpc.Ready, context.DeadlineExceeded)
	}
	tc := testpb.NewTestServiceClient(cc)
	var header metadata.MD
	reply, err := tc.EmptyCall(context.Background(), &testpb.Empty{}, grpc.Header(&header))
	if err != nil || !proto.Equal(&testpb.Empty{}, reply) {
		t.Fatalf("TestService/EmptyCall(_, _) = %v, %v, want %v, <nil>", reply, err, &testpb.Empty{})
	}
	if v, ok := header["ua"]; !ok || v[0] != testAppUA {
		t.Fatalf("header[\"ua\"] = %q, %t, want %q, true", v, ok, testAppUA)
	}

	te.srv.Stop()
	cc.Close()

	ctx, _ = context.WithTimeout(context.Background(), 5*time.Second)
	if _, err := cc.WaitForStateChange(ctx, grpc.Ready); err != nil {
		t.Fatalf("cc.WaitForStateChange(_, %s) = _, %v, want _, <nil>", grpc.Ready, err)
	}
	if state, err := cc.State(); err != nil || state != grpc.Shutdown {
		t.Fatalf("cc.State() = %s, %v, want %s, <nil>", state, err, grpc.Shutdown)
	}
}

func TestFailedEmptyUnary(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testFailedEmptyUnary(t, e)
	}
}

func testFailedEmptyUnary(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer()
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	ctx := metadata.NewContext(context.Background(), testMetadata)
	wantErr := grpc.Errorf(codes.DataLoss, "missing expected user-agent")
	if _, err := tc.EmptyCall(ctx, &testpb.Empty{}); err != wantErr {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %v", err, wantErr)
	}
}

func TestLargeUnary(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testLargeUnary(t, e)
	}
}

func testLargeUnary(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer()
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const argSize = 271828
	const respSize = 314159

	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseSize: proto.Int32(respSize),
		Payload:      payload,
	}
	reply, err := tc.UnaryCall(context.Background(), req)
	if err != nil {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, <nil>", err)
	}
	pt := reply.GetPayload().GetType()
	ps := len(reply.GetPayload().GetBody())
	if pt != testpb.PayloadType_COMPRESSABLE || ps != respSize {
		t.Fatalf("Got the reply with type %d len %d; want %d, %d", pt, ps, testpb.PayloadType_COMPRESSABLE, respSize)
	}
}

func TestMetadataUnaryRPC(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testMetadataUnaryRPC(t, e)
	}
}

func testMetadataUnaryRPC(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer()
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const argSize = 2718
	const respSize = 314

	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseSize: proto.Int32(respSize),
		Payload:      payload,
	}
	var header, trailer metadata.MD
	ctx := metadata.NewContext(context.Background(), testMetadata)
	if _, err := tc.UnaryCall(ctx, req, grpc.Header(&header), grpc.Trailer(&trailer)); err != nil {
		t.Fatalf("TestService.UnaryCall(%v, _, _, _) = _, %v; want _, <nil>", ctx, err)
	}
	// Ignore optional response headers that Servers may set:
	if header != nil {
		delete(header, "trailer") // RFC 2616 says server SHOULD (but optional) declare trailers
		delete(header, "date")    // the Date header is also optional
	}
	if !reflect.DeepEqual(header, testMetadata) {
		t.Fatalf("Received header metadata %v, want %v", header, testMetadata)
	}
	if !reflect.DeepEqual(trailer, testTrailerMetadata) {
		t.Fatalf("Received trailer metadata %v, want %v", trailer, testTrailerMetadata)
	}
}

func performOneRPC(t *testing.T, tc testpb.TestServiceClient, wg *sync.WaitGroup) {
	defer wg.Done()
	const argSize = 2718
	const respSize = 314

	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Error(err)
		return
	}

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseSize: proto.Int32(respSize),
		Payload:      payload,
	}
	reply, err := tc.UnaryCall(context.Background(), req)
	if err != nil {
		t.Errorf("TestService/UnaryCall(_, _) = _, %v, want _, <nil>", err)
		return
	}
	pt := reply.GetPayload().GetType()
	ps := len(reply.GetPayload().GetBody())
	if pt != testpb.PayloadType_COMPRESSABLE || ps != respSize {
		t.Errorf("Got reply with type %d len %d; want %d, %d", pt, ps, testpb.PayloadType_COMPRESSABLE, respSize)
		return
	}
}

func TestRetry(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testRetry(t, e)
	}
}

// This test mimics a user who sends 1000 RPCs concurrently on a faulty transport.
// TODO(zhaoq): Refactor to make this clearer and add more cases to test racy
// and error-prone paths.
func testRetry(t *testing.T, e env) {
	te := newTest(t, e)
	te.declareLogNoise("transport: http2Client.notifyError got notified that the client transport was broken")
	te.startServer()
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)

	var wg sync.WaitGroup

	numRPC := 1000
	rpcSpacing := 2 * time.Millisecond
	if raceMode {
		// The race detector has a limit on how many goroutines it can track.
		// This test is near the upper limit, and goes over the limit
		// depending on the environment (the http.Handler environment uses
		// more goroutines)
		t.Logf("Shortening test in race mode.")
		numRPC /= 2
		rpcSpacing *= 2
	}

	wg.Add(1)
	go func() {
		// Halfway through starting RPCs, kill all connections:
		time.Sleep(time.Duration(numRPC/2) * rpcSpacing)

		// The server shuts down the network connection to make a
		// transport error which will be detected by the client side
		// code.
		internal.TestingCloseConns(te.srv)
		wg.Done()
	}()
	// All these RPCs should succeed eventually.
	for i := 0; i < numRPC; i++ {
		time.Sleep(rpcSpacing)
		wg.Add(1)
		go performOneRPC(t, tc, &wg)
	}
	wg.Wait()
}

func TestRPCTimeout(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testRPCTimeout(t, e)
	}
}

// TODO(zhaoq): Have a better test coverage of timeout and cancellation mechanism.
func testRPCTimeout(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer()
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)

	const argSize = 2718
	const respSize = 314

	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseSize: proto.Int32(respSize),
		Payload:      payload,
	}
	for i := -1; i <= 10; i++ {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(i)*time.Millisecond)
		reply, err := tc.UnaryCall(ctx, req)
		if grpc.Code(err) != codes.DeadlineExceeded {
			t.Fatalf(`TestService/UnaryCallv(_, _) = %v, %v; want <nil>, error code: %d`, reply, err, codes.DeadlineExceeded)
		}
	}
}

func TestCancel(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testCancel(t, e)
	}
}

func testCancel(t *testing.T, e env) {
	te := newTest(t, e)
	te.declareLogNoise("grpc: the client connection is closing; please retry")
	te.startServer()
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)

	const argSize = 2718
	const respSize = 314

	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}

	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseSize: proto.Int32(respSize),
		Payload:      payload,
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(1*time.Millisecond, cancel)
	reply, err := tc.UnaryCall(ctx, req)
	if grpc.Code(err) != codes.Canceled {
		t.Fatalf(`TestService/UnaryCall(_, _) = %v, %v; want <nil>, error code: %d`, reply, err, codes.Canceled)
	}
	cc.Close()

	awaitNewConnLogOutput()
}

func TestCancelNoIO(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testCancelNoIO(t, e)
	}
}

func testCancelNoIO(t *testing.T, e env) {
	te := newTest(t, e)
	te.declareLogNoise("http2Client.notifyError got notified that the client transport was broken")
	te.maxStream = 1 // Only allows 1 live stream per server transport.
	te.startServer()
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)

	// Start one blocked RPC for which we'll never send streaming
	// input. This will consume the 1 maximum concurrent streams,
	// causing future RPCs to hang.
	ctx, cancelFirst := context.WithCancel(context.Background())
	_, err := tc.StreamingInputCall(ctx)
	if err != nil {
		t.Fatalf("%v.StreamingInputCall(_) = _, %v, want _, <nil>", tc, err)
	}

	// Loop until the ClientConn receives the initial settings
	// frame from the server, notifying it about the maximum
	// concurrent streams. We know when it's received it because
	// an RPC will fail with codes.DeadlineExceeded instead of
	// succeeding.
	// TODO(bradfitz): add internal test hook for this (Issue 534)
	for {
		ctx, cancelSecond := context.WithTimeout(context.Background(), 250*time.Millisecond)
		_, err := tc.StreamingInputCall(ctx)
		cancelSecond()
		if err == nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if grpc.Code(err) == codes.DeadlineExceeded {
			break
		}
		t.Fatalf("%v.StreamingInputCall(_) = _, %v, want _, %d", tc, err, codes.DeadlineExceeded)
	}
	// If there are any RPCs in flight before the client receives
	// the max streams setting, let them be expired.
	// TODO(bradfitz): add internal test hook for this (Issue 534)
	time.Sleep(500 * time.Millisecond)

	ch := make(chan struct{})
	go func() {
		defer close(ch)

		// This should be blocked until the 1st is canceled.
		ctx, cancelThird := context.WithTimeout(context.Background(), 2*time.Second)
		if _, err := tc.StreamingInputCall(ctx); err != nil {
			t.Errorf("%v.StreamingInputCall(_) = _, %v, want _, <nil>", tc, err)
		}
		cancelThird()
	}()
	cancelFirst()
	<-ch
}

// The following tests the gRPC streaming RPC implementations.
// TODO(zhaoq): Have better coverage on error cases.
var (
	reqSizes  = []int{27182, 8, 1828, 45904}
	respSizes = []int{31415, 9, 2653, 58979}
)

func TestNoService(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testNoService(t, e)
	}
}

func testNoService(t *testing.T, e env) {
	te := newTest(t, e)
	te.testServer = nil // register nothing
	te.startServer()
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)

	// Make sure setting ack has been sent.
	time.Sleep(20 * time.Millisecond)
	stream, err := tc.FullDuplexCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	if _, err := stream.Recv(); grpc.Code(err) != codes.Unimplemented {
		t.Fatalf("stream.Recv() = _, %v, want _, error code %d", err, codes.Unimplemented)
	}
}

func TestPingPong(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testPingPong(t, e)
	}
}

func testPingPong(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer()
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	stream, err := tc.FullDuplexCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	var index int
	for index < len(reqSizes) {
		respParam := []*testpb.ResponseParameters{
			{
				Size: proto.Int32(int32(respSizes[index])),
			},
		}

		payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, int32(reqSizes[index]))
		if err != nil {
			t.Fatal(err)
		}

		req := &testpb.StreamingOutputCallRequest{
			ResponseType:       testpb.PayloadType_COMPRESSABLE.Enum(),
			ResponseParameters: respParam,
			Payload:            payload,
		}
		if err := stream.Send(req); err != nil {
			t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, req, err)
		}
		reply, err := stream.Recv()
		if err != nil {
			t.Fatalf("%v.Recv() = %v, want <nil>", stream, err)
		}
		pt := reply.GetPayload().GetType()
		if pt != testpb.PayloadType_COMPRESSABLE {
			t.Fatalf("Got the reply of type %d, want %d", pt, testpb.PayloadType_COMPRESSABLE)
		}
		size := len(reply.GetPayload().GetBody())
		if size != int(respSizes[index]) {
			t.Fatalf("Got reply body of length %d, want %d", size, respSizes[index])
		}
		index++
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("%v.CloseSend() got %v, want %v", stream, err, nil)
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("%v failed to complele the ping pong test: %v", stream, err)
	}
}

func TestMetadataStreamingRPC(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testMetadataStreamingRPC(t, e)
	}
}

func testMetadataStreamingRPC(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer()
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	ctx := metadata.NewContext(te.ctx, testMetadata)
	stream, err := tc.FullDuplexCall(ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	go func() {
		headerMD, err := stream.Header()
		if e.security == "tls" {
			delete(headerMD, "transport_security_type")
		}
		delete(headerMD, "trailer") // ignore if present
		if err != nil || !reflect.DeepEqual(testMetadata, headerMD) {
			t.Errorf("#1 %v.Header() = %v, %v, want %v, <nil>", stream, headerMD, err, testMetadata)
		}
		// test the cached value.
		headerMD, err = stream.Header()
		delete(headerMD, "trailer") // ignore if present
		if err != nil || !reflect.DeepEqual(testMetadata, headerMD) {
			t.Errorf("#2 %v.Header() = %v, %v, want %v, <nil>", stream, headerMD, err, testMetadata)
		}
		var index int
		for index < len(reqSizes) {
			respParam := []*testpb.ResponseParameters{
				{
					Size: proto.Int32(int32(respSizes[index])),
				},
			}

			payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, int32(reqSizes[index]))
			if err != nil {
				t.Fatal(err)
			}

			req := &testpb.StreamingOutputCallRequest{
				ResponseType:       testpb.PayloadType_COMPRESSABLE.Enum(),
				ResponseParameters: respParam,
				Payload:            payload,
			}
			if err := stream.Send(req); err != nil {
				t.Errorf("%v.Send(%v) = %v, want <nil>", stream, req, err)
				return
			}
			index++
		}
		// Tell the server we're done sending args.
		stream.CloseSend()
	}()
	for {
		if _, err := stream.Recv(); err != nil {
			break
		}
	}
	trailerMD := stream.Trailer()
	if !reflect.DeepEqual(testMetadata, trailerMD) {
		t.Fatalf("%v.Trailer() = %v, want %v", stream, trailerMD, testMetadata)
	}
}

func TestServerStreaming(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testServerStreaming(t, e)
	}
}

func testServerStreaming(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer()
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	respParam := make([]*testpb.ResponseParameters, len(respSizes))
	for i, s := range respSizes {
		respParam[i] = &testpb.ResponseParameters{
			Size: proto.Int32(int32(s)),
		}
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseParameters: respParam,
	}
	stream, err := tc.StreamingOutputCall(context.Background(), req)
	if err != nil {
		t.Fatalf("%v.StreamingOutputCall(_) = _, %v, want <nil>", tc, err)
	}
	var rpcStatus error
	var respCnt int
	var index int
	for {
		reply, err := stream.Recv()
		if err != nil {
			rpcStatus = err
			break
		}
		pt := reply.GetPayload().GetType()
		if pt != testpb.PayloadType_COMPRESSABLE {
			t.Fatalf("Got the reply of type %d, want %d", pt, testpb.PayloadType_COMPRESSABLE)
		}
		size := len(reply.GetPayload().GetBody())
		if size != int(respSizes[index]) {
			t.Fatalf("Got reply body of length %d, want %d", size, respSizes[index])
		}
		index++
		respCnt++
	}
	if rpcStatus != io.EOF {
		t.Fatalf("Failed to finish the server streaming rpc: %v, want <EOF>", rpcStatus)
	}
	if respCnt != len(respSizes) {
		t.Fatalf("Got %d reply, want %d", len(respSizes), respCnt)
	}
}

func TestFailedServerStreaming(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testFailedServerStreaming(t, e)
	}
}

func testFailedServerStreaming(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer()
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	respParam := make([]*testpb.ResponseParameters, len(respSizes))
	for i, s := range respSizes {
		respParam[i] = &testpb.ResponseParameters{
			Size: proto.Int32(int32(s)),
		}
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseParameters: respParam,
	}
	ctx := metadata.NewContext(te.ctx, testMetadata)
	stream, err := tc.StreamingOutputCall(ctx, req)
	if err != nil {
		t.Fatalf("%v.StreamingOutputCall(_) = _, %v, want <nil>", tc, err)
	}
	if _, err := stream.Recv(); err != grpc.Errorf(codes.DataLoss, "got extra metadata") {
		t.Fatalf("%v.Recv() = _, %v, want _, %v", stream, err, grpc.Errorf(codes.DataLoss, "got extra metadata"))
	}
}

// concurrentSendServer is a TestServiceServer whose
// StreamingOutputCall makes ten serial Send calls, sending payloads
// "0".."9", inclusive.  TestServerStreaming_Concurrent verifies they
// were received in the correct order, and that there were no races.
//
// All other TestServiceServer methods crash if called.
type concurrentSendServer struct {
	testpb.TestServiceServer
}

func (s concurrentSendServer) StreamingOutputCall(args *testpb.StreamingOutputCallRequest, stream testpb.TestService_StreamingOutputCallServer) error {
	for i := 0; i < 10; i++ {
		stream.Send(&testpb.StreamingOutputCallResponse{
			Payload: &testpb.Payload{
				Body: []byte{'0' + uint8(i)},
			},
		})
	}
	return nil
}

// Tests doing a bunch of concurrent streaming output calls.
func TestServerStreaming_Concurrent(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testServerStreaming_Concurrent(t, e)
	}
}

func testServerStreaming_Concurrent(t *testing.T, e env) {
	te := newTest(t, e)
	te.testServer = concurrentSendServer{}
	te.startServer()
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)

	doStreamingCall := func() {
		req := &testpb.StreamingOutputCallRequest{}
		stream, err := tc.StreamingOutputCall(context.Background(), req)
		if err != nil {
			t.Errorf("%v.StreamingOutputCall(_) = _, %v, want <nil>", tc, err)
			return
		}
		var ngot int
		var buf bytes.Buffer
		for {
			reply, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			ngot++
			if buf.Len() > 0 {
				buf.WriteByte(',')
			}
			buf.Write(reply.GetPayload().GetBody())
		}
		if want := 10; ngot != want {
			t.Errorf("Got %d replies, want %d", ngot, want)
		}
		if got, want := buf.String(), "0,1,2,3,4,5,6,7,8,9"; got != want {
			t.Errorf("Got replies %q; want %q", got, want)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			doStreamingCall()
		}()
	}
	wg.Wait()

}

func TestClientStreaming(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testClientStreaming(t, e)
	}
}

func testClientStreaming(t *testing.T, e env) {
	te := newTest(t, e)
	te.startServer()
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	stream, err := tc.StreamingInputCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.StreamingInputCall(_) = _, %v, want <nil>", tc, err)
	}

	var sum int
	for _, s := range reqSizes {
		payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, int32(s))
		if err != nil {
			t.Fatal(err)
		}

		req := &testpb.StreamingInputCallRequest{
			Payload: payload,
		}
		if err := stream.Send(req); err != nil {
			t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, req, err)
		}
		sum += s
	}
	reply, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("%v.CloseAndRecv() got error %v, want %v", stream, err, nil)
	}
	if reply.GetAggregatedPayloadSize() != int32(sum) {
		t.Fatalf("%v.CloseAndRecv().GetAggregatePayloadSize() = %v; want %v", stream, reply.GetAggregatedPayloadSize(), sum)
	}
}

func TestExceedMaxStreamsLimit(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testExceedMaxStreamsLimit(t, e)
	}
}

func testExceedMaxStreamsLimit(t *testing.T, e env) {
	te := newTest(t, e)
	te.declareLogNoise(
		"http2Client.notifyError got notified that the client transport was broken",
		"Conn.resetTransport failed to create client transport",
		"grpc: the client connection is closing",
	)
	te.maxStream = 1 // Only allows 1 live stream per server transport.
	te.startServer()
	defer te.tearDown()

	cc := te.clientConn()
	tc := testpb.NewTestServiceClient(cc)

	_, err := tc.StreamingInputCall(te.ctx)
	if err != nil {
		t.Fatalf("%v.StreamingInputCall(_) = _, %v, want _, <nil>", tc, err)
	}
	// Loop until receiving the new max stream setting from the server.
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, err := tc.StreamingInputCall(ctx)
		if err == nil {
			time.Sleep(time.Second)
			continue
		}
		if grpc.Code(err) == codes.DeadlineExceeded {
			break
		}
		t.Fatalf("%v.StreamingInputCall(_) = _, %v, want _, %d", tc, err, codes.DeadlineExceeded)
	}
}

func TestCompressServerHasNoSupport(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testCompressServerHasNoSupport(t, e)
	}
}

func testCompressServerHasNoSupport(t *testing.T, e env) {
	te := newTest(t, e)
	te.serverCompression = false
	te.clientCompression = true
	te.startServer()
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	const argSize = 271828
	const respSize = 314159
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseSize: proto.Int32(respSize),
		Payload:      payload,
	}
	if _, err := tc.UnaryCall(context.Background(), req); err == nil || grpc.Code(err) != codes.InvalidArgument {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, error code %d", err, codes.InvalidArgument)
	}
	// Streaming RPC
	stream, err := tc.FullDuplexCall(context.Background())
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	respParam := []*testpb.ResponseParameters{
		{
			Size: proto.Int32(31415),
		},
	}
	payload, err = newPayload(testpb.PayloadType_COMPRESSABLE, int32(31415))
	if err != nil {
		t.Fatal(err)
	}
	sreq := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseParameters: respParam,
		Payload:            payload,
	}
	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err := stream.Recv(); err == nil || grpc.Code(err) != codes.InvalidArgument {
		t.Fatalf("%v.Recv() = %v, want error code %d", stream, err, codes.InvalidArgument)
	}
}

func TestCompressOK(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testCompressOK(t, e)
	}
}

func testCompressOK(t *testing.T, e env) {
	te := newTest(t, e)
	te.serverCompression = true
	te.clientCompression = true
	te.startServer()
	defer te.tearDown()
	tc := testpb.NewTestServiceClient(te.clientConn())

	// Unary call
	const argSize = 271828
	const respSize = 314159
	payload, err := newPayload(testpb.PayloadType_COMPRESSABLE, argSize)
	if err != nil {
		t.Fatal(err)
	}
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseSize: proto.Int32(respSize),
		Payload:      payload,
	}
	if _, err := tc.UnaryCall(context.Background(), req); err != nil {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, <nil>", err)
	}
	// Streaming RPC
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := tc.FullDuplexCall(ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	respParam := []*testpb.ResponseParameters{
		{
			Size: proto.Int32(31415),
		},
	}
	payload, err = newPayload(testpb.PayloadType_COMPRESSABLE, int32(31415))
	if err != nil {
		t.Fatal(err)
	}
	sreq := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseParameters: respParam,
		Payload:            payload,
	}
	if err := stream.Send(sreq); err != nil {
		t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, sreq, err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("%v.Recv() = %v, want <nil>", stream, err)
	}
}

// funcServer implements methods of TestServiceServer using funcs,
// similar to an http.HandlerFunc.
// Any unimplemented method will crash. Tests implement the method(s)
// they need.
type funcServer struct {
	testpb.TestServiceServer
	unaryCall          func(ctx context.Context, in *testpb.SimpleRequest) (*testpb.SimpleResponse, error)
	streamingInputCall func(stream testpb.TestService_StreamingInputCallServer) error
}

func (s *funcServer) UnaryCall(ctx context.Context, in *testpb.SimpleRequest) (*testpb.SimpleResponse, error) {
	return s.unaryCall(ctx, in)
}

func (s *funcServer) StreamingInputCall(stream testpb.TestService_StreamingInputCallServer) error {
	return s.streamingInputCall(stream)
}

func TestClientRequestBodyError_UnexpectedEOF(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testClientRequestBodyError_UnexpectedEOF(t, e)
	}
}

func testClientRequestBodyError_UnexpectedEOF(t *testing.T, e env) {
	te := newTest(t, e)
	te.testServer = &funcServer{unaryCall: func(ctx context.Context, in *testpb.SimpleRequest) (*testpb.SimpleResponse, error) {
		errUnexpectedCall := errors.New("unexpected call func server method")
		t.Error(errUnexpectedCall)
		return nil, errUnexpectedCall
	}}
	te.startServer()
	defer te.tearDown()
	te.withServerTester(func(st *serverTester) {
		st.writeHeadersGRPC(1, "/grpc.testing.TestService/UnaryCall")
		// Say we have 5 bytes coming, but set END_STREAM flag:
		st.writeData(1, true, []byte{0, 0, 0, 0, 5})
		st.wantAnyFrame() // wait for server to crash (it used to crash)
	})
}

func TestClientRequestBodyError_CloseAfterLength(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testClientRequestBodyError_CloseAfterLength(t, e)
	}
}

func testClientRequestBodyError_CloseAfterLength(t *testing.T, e env) {
	te := newTest(t, e)
	te.declareLogNoise("Server.processUnaryRPC failed to write status")
	te.testServer = &funcServer{unaryCall: func(ctx context.Context, in *testpb.SimpleRequest) (*testpb.SimpleResponse, error) {
		errUnexpectedCall := errors.New("unexpected call func server method")
		t.Error(errUnexpectedCall)
		return nil, errUnexpectedCall
	}}
	te.startServer()
	defer te.tearDown()
	te.withServerTester(func(st *serverTester) {
		st.writeHeadersGRPC(1, "/grpc.testing.TestService/UnaryCall")
		// say we're sending 5 bytes, but then close the connection instead.
		st.writeData(1, false, []byte{0, 0, 0, 0, 5})
		st.cc.Close()
	})
}

func TestClientRequestBodyError_Cancel(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testClientRequestBodyError_Cancel(t, e)
	}
}

func testClientRequestBodyError_Cancel(t *testing.T, e env) {
	te := newTest(t, e)
	gotCall := make(chan bool, 1)
	te.testServer = &funcServer{unaryCall: func(ctx context.Context, in *testpb.SimpleRequest) (*testpb.SimpleResponse, error) {
		gotCall <- true
		return new(testpb.SimpleResponse), nil
	}}
	te.startServer()
	defer te.tearDown()
	te.withServerTester(func(st *serverTester) {
		st.writeHeadersGRPC(1, "/grpc.testing.TestService/UnaryCall")
		// Say we have 5 bytes coming, but cancel it instead.
		st.writeData(1, false, []byte{0, 0, 0, 0, 5})
		st.writeRSTStream(1, http2.ErrCodeCancel)

		// Verify we didn't a call yet.
		select {
		case <-gotCall:
			t.Fatal("unexpected call")
		default:
		}

		// And now send an uncanceled (but still invalid), just to get a response.
		st.writeHeadersGRPC(3, "/grpc.testing.TestService/UnaryCall")
		st.writeData(3, true, []byte{0, 0, 0, 0, 0})
		<-gotCall
		st.wantAnyFrame()
	})
}

func TestClientRequestBodyError_Cancel_StreamingInput(t *testing.T) {
	defer leakCheck(t)()
	for _, e := range listTestEnv() {
		testClientRequestBodyError_Cancel_StreamingInput(t, e)
	}
}

func testClientRequestBodyError_Cancel_StreamingInput(t *testing.T, e env) {
	te := newTest(t, e)
	recvErr := make(chan error, 1)
	te.testServer = &funcServer{streamingInputCall: func(stream testpb.TestService_StreamingInputCallServer) error {
		_, err := stream.Recv()
		recvErr <- err
		return nil
	}}
	te.startServer()
	defer te.tearDown()
	te.withServerTester(func(st *serverTester) {
		st.writeHeadersGRPC(1, "/grpc.testing.TestService/StreamingInputCall")
		// Say we have 5 bytes coming, but cancel it instead.
		st.writeData(1, false, []byte{0, 0, 0, 0, 5})
		st.writeRSTStream(1, http2.ErrCodeCancel)

		var got error
		select {
		case got = <-recvErr:
		case <-time.After(3 * time.Second):
			t.Fatal("timeout waiting for error")
		}
		if se, ok := got.(transport.StreamError); !ok || se.Code != codes.Canceled {
			t.Errorf("error = %#v; want transport.StreamError with code Canceled")
		}
	})
}

// interestingGoroutines returns all goroutines we care about for the purpose
// of leak checking. It excludes testing or runtime ones.
func interestingGoroutines() (gs []string) {
	buf := make([]byte, 2<<20)
	buf = buf[:runtime.Stack(buf, true)]
	for _, g := range strings.Split(string(buf), "\n\n") {
		sl := strings.SplitN(g, "\n", 2)
		if len(sl) != 2 {
			continue
		}
		stack := strings.TrimSpace(sl[1])
		if strings.HasPrefix(stack, "testing.RunTests") {
			continue
		}

		if stack == "" ||
			strings.Contains(stack, "testing.Main(") ||
			strings.Contains(stack, "runtime.goexit") ||
			strings.Contains(stack, "created by runtime.gc") ||
			strings.Contains(stack, "interestingGoroutines") ||
			strings.Contains(stack, "runtime.MHeap_Scavenger") ||
			strings.Contains(stack, "signal.signal_recv") ||
			strings.Contains(stack, "sigterm.handler") ||
			strings.Contains(stack, "runtime_mcall") ||
			strings.Contains(stack, "goroutine in C code") {
			continue
		}
		gs = append(gs, g)
	}
	sort.Strings(gs)
	return
}

// leakCheck snapshots the currently-running goroutines and returns a
// function to be run at the end of tests to see whether any
// goroutines leaked.
func leakCheck(t testing.TB) func() {
	orig := map[string]bool{}
	for _, g := range interestingGoroutines() {
		orig[g] = true
	}
	return func() {
		// Loop, waiting for goroutines to shut down.
		// Wait up to 5 seconds, but finish as quickly as possible.
		deadline := time.Now().Add(5 * time.Second)
		for {
			var leaked []string
			for _, g := range interestingGoroutines() {
				if !orig[g] {
					leaked = append(leaked, g)
				}
			}
			if len(leaked) == 0 {
				return
			}
			if time.Now().Before(deadline) {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			for _, g := range leaked {
				t.Errorf("Leaked goroutine: %v", g)
			}
			return
		}
	}
}

type lockingWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (lw *lockingWriter) Write(p []byte) (n int, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.w.Write(p)
}

func (lw *lockingWriter) setWriter(w io.Writer) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	lw.w = w
}

var testLogOutput = &lockingWriter{w: os.Stderr}

// awaitNewConnLogOutput waits for any of grpc.NewConn's goroutines to
// terminate, if they're still running. It spams logs with this
// message.  We wait for it so our log filter is still
// active. Otherwise the "defer restore()" at the top of various test
// functions restores our log filter and then the goroutine spams.
func awaitNewConnLogOutput() {
	awaitLogOutput(50*time.Millisecond, "grpc: the client connection is closing; please retry")
}

func awaitLogOutput(maxWait time.Duration, phrase string) {
	pb := []byte(phrase)

	timer := time.NewTimer(maxWait)
	defer timer.Stop()
	wakeup := make(chan bool, 1)
	for {
		if logOutputHasContents(pb, wakeup) {
			return
		}
		select {
		case <-timer.C:
			// Too slow. Oh well.
			return
		case <-wakeup:
		}
	}
}

func logOutputHasContents(v []byte, wakeup chan<- bool) bool {
	testLogOutput.mu.Lock()
	defer testLogOutput.mu.Unlock()
	fw, ok := testLogOutput.w.(*filterWriter)
	if !ok {
		return false
	}
	fw.mu.Lock()
	defer fw.mu.Unlock()
	if bytes.Contains(fw.buf.Bytes(), v) {
		return true
	}
	fw.wakeup = wakeup
	return false
}

func init() {
	grpclog.SetLogger(log.New(testLogOutput, "", log.LstdFlags))
}

var verboseLogs = flag.Bool("verbose_logs", false, "show all grpclog output, without filtering")

func noop() {}

// declareLogNoise declares that t is expected to emit the following noisy phrases,
// even on success. Those phrases will be filtered from grpclog output
// and only be shown if *verbose_logs or t ends up failing.
// The returned restore function should be called with defer to be run
// before the test ends.
func declareLogNoise(t *testing.T, phrases ...string) (restore func()) {
	if *verboseLogs {
		return noop
	}
	fw := &filterWriter{dst: os.Stderr, filter: phrases}
	testLogOutput.setWriter(fw)
	return func() {
		if t.Failed() {
			fw.mu.Lock()
			defer fw.mu.Unlock()
			if fw.buf.Len() > 0 {
				t.Logf("Complete log output:\n%s", fw.buf.Bytes())
			}
		}
		testLogOutput.setWriter(os.Stderr)
	}
}

type filterWriter struct {
	dst    io.Writer
	filter []string

	mu     sync.Mutex
	buf    bytes.Buffer
	wakeup chan<- bool // if non-nil, gets true on write
}

func (fw *filterWriter) Write(p []byte) (n int, err error) {
	fw.mu.Lock()
	fw.buf.Write(p)
	if fw.wakeup != nil {
		select {
		case fw.wakeup <- true:
		default:
		}
	}
	fw.mu.Unlock()

	ps := string(p)
	for _, f := range fw.filter {
		if strings.Contains(ps, f) {
			return len(p), nil
		}
	}
	return fw.dst.Write(p)
}
