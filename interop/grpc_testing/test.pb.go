// Code generated by protoc-gen-go.
// source: test.proto
// DO NOT EDIT!

/*
Package grpc_testing is a generated protocol buffer package.

It is generated from these files:
	test.proto

It has these top-level messages:
	Empty
	Payload
	SimpleRequest
	SimpleResponse
	StreamingInputCallRequest
	StreamingInputCallResponse
	ResponseParameters
	StreamingOutputCallRequest
	StreamingOutputCallResponse
*/
package grpc_testing

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "github.com/VerveWireless/grpc-go"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
const _ = proto.ProtoPackageIsVersion1

// The type of payload that should be returned.
type PayloadType int32

const (
	// Compressable text format.
	PayloadType_COMPRESSABLE PayloadType = 0
	// Uncompressable binary format.
	PayloadType_UNCOMPRESSABLE PayloadType = 1
	// Randomly chosen from all other formats defined in this enum.
	PayloadType_RANDOM PayloadType = 2
)

var PayloadType_name = map[int32]string{
	0: "COMPRESSABLE",
	1: "UNCOMPRESSABLE",
	2: "RANDOM",
}
var PayloadType_value = map[string]int32{
	"COMPRESSABLE":   0,
	"UNCOMPRESSABLE": 1,
	"RANDOM":         2,
}

func (x PayloadType) Enum() *PayloadType {
	p := new(PayloadType)
	*p = x
	return p
}
func (x PayloadType) String() string {
	return proto.EnumName(PayloadType_name, int32(x))
}
func (x *PayloadType) UnmarshalJSON(data []byte) error {
	value, err := proto.UnmarshalJSONEnum(PayloadType_value, data, "PayloadType")
	if err != nil {
		return err
	}
	*x = PayloadType(value)
	return nil
}
func (PayloadType) EnumDescriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type Empty struct {
	XXX_unrecognized []byte `json:"-"`
}

func (m *Empty) Reset()                    { *m = Empty{} }
func (m *Empty) String() string            { return proto.CompactTextString(m) }
func (*Empty) ProtoMessage()               {}
func (*Empty) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

// A block of data, to simply increase gRPC message size.
type Payload struct {
	// The type of data in body.
	Type *PayloadType `protobuf:"varint,1,opt,name=type,enum=grpc.testing.PayloadType" json:"type,omitempty"`
	// Primary contents of payload.
	Body             []byte `protobuf:"bytes,2,opt,name=body" json:"body,omitempty"`
	XXX_unrecognized []byte `json:"-"`
}

func (m *Payload) Reset()                    { *m = Payload{} }
func (m *Payload) String() string            { return proto.CompactTextString(m) }
func (*Payload) ProtoMessage()               {}
func (*Payload) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *Payload) GetType() PayloadType {
	if m != nil && m.Type != nil {
		return *m.Type
	}
	return PayloadType_COMPRESSABLE
}

func (m *Payload) GetBody() []byte {
	if m != nil {
		return m.Body
	}
	return nil
}

// Unary request.
type SimpleRequest struct {
	// Desired payload type in the response from the server.
	// If response_type is RANDOM, server randomly chooses one from other formats.
	ResponseType *PayloadType `protobuf:"varint,1,opt,name=response_type,enum=grpc.testing.PayloadType" json:"response_type,omitempty"`
	// Desired payload size in the response from the server.
	// If response_type is COMPRESSABLE, this denotes the size before compression.
	ResponseSize *int32 `protobuf:"varint,2,opt,name=response_size" json:"response_size,omitempty"`
	// Optional input payload sent along with the request.
	Payload *Payload `protobuf:"bytes,3,opt,name=payload" json:"payload,omitempty"`
	// Whether SimpleResponse should include username.
	FillUsername *bool `protobuf:"varint,4,opt,name=fill_username" json:"fill_username,omitempty"`
	// Whether SimpleResponse should include OAuth scope.
	FillOauthScope   *bool  `protobuf:"varint,5,opt,name=fill_oauth_scope" json:"fill_oauth_scope,omitempty"`
	XXX_unrecognized []byte `json:"-"`
}

func (m *SimpleRequest) Reset()                    { *m = SimpleRequest{} }
func (m *SimpleRequest) String() string            { return proto.CompactTextString(m) }
func (*SimpleRequest) ProtoMessage()               {}
func (*SimpleRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *SimpleRequest) GetResponseType() PayloadType {
	if m != nil && m.ResponseType != nil {
		return *m.ResponseType
	}
	return PayloadType_COMPRESSABLE
}

func (m *SimpleRequest) GetResponseSize() int32 {
	if m != nil && m.ResponseSize != nil {
		return *m.ResponseSize
	}
	return 0
}

func (m *SimpleRequest) GetPayload() *Payload {
	if m != nil {
		return m.Payload
	}
	return nil
}

func (m *SimpleRequest) GetFillUsername() bool {
	if m != nil && m.FillUsername != nil {
		return *m.FillUsername
	}
	return false
}

func (m *SimpleRequest) GetFillOauthScope() bool {
	if m != nil && m.FillOauthScope != nil {
		return *m.FillOauthScope
	}
	return false
}

// Unary response, as configured by the request.
type SimpleResponse struct {
	// Payload to increase message size.
	Payload *Payload `protobuf:"bytes,1,opt,name=payload" json:"payload,omitempty"`
	// The user the request came from, for verifying authentication was
	// successful when the client expected it.
	Username *string `protobuf:"bytes,2,opt,name=username" json:"username,omitempty"`
	// OAuth scope.
	OauthScope       *string `protobuf:"bytes,3,opt,name=oauth_scope" json:"oauth_scope,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *SimpleResponse) Reset()                    { *m = SimpleResponse{} }
func (m *SimpleResponse) String() string            { return proto.CompactTextString(m) }
func (*SimpleResponse) ProtoMessage()               {}
func (*SimpleResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func (m *SimpleResponse) GetPayload() *Payload {
	if m != nil {
		return m.Payload
	}
	return nil
}

func (m *SimpleResponse) GetUsername() string {
	if m != nil && m.Username != nil {
		return *m.Username
	}
	return ""
}

func (m *SimpleResponse) GetOauthScope() string {
	if m != nil && m.OauthScope != nil {
		return *m.OauthScope
	}
	return ""
}

// Client-streaming request.
type StreamingInputCallRequest struct {
	// Optional input payload sent along with the request.
	Payload          *Payload `protobuf:"bytes,1,opt,name=payload" json:"payload,omitempty"`
	XXX_unrecognized []byte   `json:"-"`
}

func (m *StreamingInputCallRequest) Reset()                    { *m = StreamingInputCallRequest{} }
func (m *StreamingInputCallRequest) String() string            { return proto.CompactTextString(m) }
func (*StreamingInputCallRequest) ProtoMessage()               {}
func (*StreamingInputCallRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func (m *StreamingInputCallRequest) GetPayload() *Payload {
	if m != nil {
		return m.Payload
	}
	return nil
}

// Client-streaming response.
type StreamingInputCallResponse struct {
	// Aggregated size of payloads received from the client.
	AggregatedPayloadSize *int32 `protobuf:"varint,1,opt,name=aggregated_payload_size" json:"aggregated_payload_size,omitempty"`
	XXX_unrecognized      []byte `json:"-"`
}

func (m *StreamingInputCallResponse) Reset()                    { *m = StreamingInputCallResponse{} }
func (m *StreamingInputCallResponse) String() string            { return proto.CompactTextString(m) }
func (*StreamingInputCallResponse) ProtoMessage()               {}
func (*StreamingInputCallResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{5} }

func (m *StreamingInputCallResponse) GetAggregatedPayloadSize() int32 {
	if m != nil && m.AggregatedPayloadSize != nil {
		return *m.AggregatedPayloadSize
	}
	return 0
}

// Configuration for a particular response.
type ResponseParameters struct {
	// Desired payload sizes in responses from the server.
	// If response_type is COMPRESSABLE, this denotes the size before compression.
	Size *int32 `protobuf:"varint,1,opt,name=size" json:"size,omitempty"`
	// Desired interval between consecutive responses in the response stream in
	// microseconds.
	IntervalUs       *int32 `protobuf:"varint,2,opt,name=interval_us" json:"interval_us,omitempty"`
	XXX_unrecognized []byte `json:"-"`
}

func (m *ResponseParameters) Reset()                    { *m = ResponseParameters{} }
func (m *ResponseParameters) String() string            { return proto.CompactTextString(m) }
func (*ResponseParameters) ProtoMessage()               {}
func (*ResponseParameters) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{6} }

func (m *ResponseParameters) GetSize() int32 {
	if m != nil && m.Size != nil {
		return *m.Size
	}
	return 0
}

func (m *ResponseParameters) GetIntervalUs() int32 {
	if m != nil && m.IntervalUs != nil {
		return *m.IntervalUs
	}
	return 0
}

// Server-streaming request.
type StreamingOutputCallRequest struct {
	// Desired payload type in the response from the server.
	// If response_type is RANDOM, the payload from each response in the stream
	// might be of different types. This is to simulate a mixed type of payload
	// stream.
	ResponseType *PayloadType `protobuf:"varint,1,opt,name=response_type,enum=grpc.testing.PayloadType" json:"response_type,omitempty"`
	// Configuration for each expected response message.
	ResponseParameters []*ResponseParameters `protobuf:"bytes,2,rep,name=response_parameters" json:"response_parameters,omitempty"`
	// Optional input payload sent along with the request.
	Payload          *Payload `protobuf:"bytes,3,opt,name=payload" json:"payload,omitempty"`
	XXX_unrecognized []byte   `json:"-"`
}

func (m *StreamingOutputCallRequest) Reset()                    { *m = StreamingOutputCallRequest{} }
func (m *StreamingOutputCallRequest) String() string            { return proto.CompactTextString(m) }
func (*StreamingOutputCallRequest) ProtoMessage()               {}
func (*StreamingOutputCallRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{7} }

func (m *StreamingOutputCallRequest) GetResponseType() PayloadType {
	if m != nil && m.ResponseType != nil {
		return *m.ResponseType
	}
	return PayloadType_COMPRESSABLE
}

func (m *StreamingOutputCallRequest) GetResponseParameters() []*ResponseParameters {
	if m != nil {
		return m.ResponseParameters
	}
	return nil
}

func (m *StreamingOutputCallRequest) GetPayload() *Payload {
	if m != nil {
		return m.Payload
	}
	return nil
}

// Server-streaming response, as configured by the request and parameters.
type StreamingOutputCallResponse struct {
	// Payload to increase response size.
	Payload          *Payload `protobuf:"bytes,1,opt,name=payload" json:"payload,omitempty"`
	XXX_unrecognized []byte   `json:"-"`
}

func (m *StreamingOutputCallResponse) Reset()                    { *m = StreamingOutputCallResponse{} }
func (m *StreamingOutputCallResponse) String() string            { return proto.CompactTextString(m) }
func (*StreamingOutputCallResponse) ProtoMessage()               {}
func (*StreamingOutputCallResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{8} }

func (m *StreamingOutputCallResponse) GetPayload() *Payload {
	if m != nil {
		return m.Payload
	}
	return nil
}

func init() {
	proto.RegisterType((*Empty)(nil), "grpc.testing.Empty")
	proto.RegisterType((*Payload)(nil), "grpc.testing.Payload")
	proto.RegisterType((*SimpleRequest)(nil), "grpc.testing.SimpleRequest")
	proto.RegisterType((*SimpleResponse)(nil), "grpc.testing.SimpleResponse")
	proto.RegisterType((*StreamingInputCallRequest)(nil), "grpc.testing.StreamingInputCallRequest")
	proto.RegisterType((*StreamingInputCallResponse)(nil), "grpc.testing.StreamingInputCallResponse")
	proto.RegisterType((*ResponseParameters)(nil), "grpc.testing.ResponseParameters")
	proto.RegisterType((*StreamingOutputCallRequest)(nil), "grpc.testing.StreamingOutputCallRequest")
	proto.RegisterType((*StreamingOutputCallResponse)(nil), "grpc.testing.StreamingOutputCallResponse")
	proto.RegisterEnum("grpc.testing.PayloadType", PayloadType_name, PayloadType_value)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// Client API for TestService service

type TestServiceClient interface {
	// One empty request followed by one empty response.
	EmptyCall(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Empty, error)
	// One request followed by one response.
	// The server returns the client payload as-is.
	UnaryCall(ctx context.Context, in *SimpleRequest, opts ...grpc.CallOption) (*SimpleResponse, error)
	// One request followed by a sequence of responses (streamed download).
	// The server returns the payload with client desired type and sizes.
	StreamingOutputCall(ctx context.Context, in *StreamingOutputCallRequest, opts ...grpc.CallOption) (TestService_StreamingOutputCallClient, error)
	// A sequence of requests followed by one response (streamed upload).
	// The server returns the aggregated size of client payload as the result.
	StreamingInputCall(ctx context.Context, opts ...grpc.CallOption) (TestService_StreamingInputCallClient, error)
	// A sequence of requests with each request served by the server immediately.
	// As one request could lead to multiple responses, this interface
	// demonstrates the idea of full duplexing.
	FullDuplexCall(ctx context.Context, opts ...grpc.CallOption) (TestService_FullDuplexCallClient, error)
	// A sequence of requests followed by a sequence of responses.
	// The server buffers all the client requests and then serves them in order. A
	// stream of responses are returned to the client when the server starts with
	// first request.
	HalfDuplexCall(ctx context.Context, opts ...grpc.CallOption) (TestService_HalfDuplexCallClient, error)
}

type testServiceClient struct {
	cc *grpc.ClientConn
}

func NewTestServiceClient(cc *grpc.ClientConn) TestServiceClient {
	return &testServiceClient{cc}
}

func (c *testServiceClient) EmptyCall(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := grpc.Invoke(ctx, "/grpc.testing.TestService/EmptyCall", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *testServiceClient) UnaryCall(ctx context.Context, in *SimpleRequest, opts ...grpc.CallOption) (*SimpleResponse, error) {
	out := new(SimpleResponse)
	err := grpc.Invoke(ctx, "/grpc.testing.TestService/UnaryCall", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *testServiceClient) StreamingOutputCall(ctx context.Context, in *StreamingOutputCallRequest, opts ...grpc.CallOption) (TestService_StreamingOutputCallClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_TestService_serviceDesc.Streams[0], c.cc, "/grpc.testing.TestService/StreamingOutputCall", opts...)
	if err != nil {
		return nil, err
	}
	x := &testServiceStreamingOutputCallClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type TestService_StreamingOutputCallClient interface {
	Recv() (*StreamingOutputCallResponse, error)
	grpc.ClientStream
}

type testServiceStreamingOutputCallClient struct {
	grpc.ClientStream
}

func (x *testServiceStreamingOutputCallClient) Recv() (*StreamingOutputCallResponse, error) {
	m := new(StreamingOutputCallResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *testServiceClient) StreamingInputCall(ctx context.Context, opts ...grpc.CallOption) (TestService_StreamingInputCallClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_TestService_serviceDesc.Streams[1], c.cc, "/grpc.testing.TestService/StreamingInputCall", opts...)
	if err != nil {
		return nil, err
	}
	x := &testServiceStreamingInputCallClient{stream}
	return x, nil
}

type TestService_StreamingInputCallClient interface {
	Send(*StreamingInputCallRequest) error
	CloseAndRecv() (*StreamingInputCallResponse, error)
	grpc.ClientStream
}

type testServiceStreamingInputCallClient struct {
	grpc.ClientStream
}

func (x *testServiceStreamingInputCallClient) Send(m *StreamingInputCallRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *testServiceStreamingInputCallClient) CloseAndRecv() (*StreamingInputCallResponse, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(StreamingInputCallResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *testServiceClient) FullDuplexCall(ctx context.Context, opts ...grpc.CallOption) (TestService_FullDuplexCallClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_TestService_serviceDesc.Streams[2], c.cc, "/grpc.testing.TestService/FullDuplexCall", opts...)
	if err != nil {
		return nil, err
	}
	x := &testServiceFullDuplexCallClient{stream}
	return x, nil
}

type TestService_FullDuplexCallClient interface {
	Send(*StreamingOutputCallRequest) error
	Recv() (*StreamingOutputCallResponse, error)
	grpc.ClientStream
}

type testServiceFullDuplexCallClient struct {
	grpc.ClientStream
}

func (x *testServiceFullDuplexCallClient) Send(m *StreamingOutputCallRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *testServiceFullDuplexCallClient) Recv() (*StreamingOutputCallResponse, error) {
	m := new(StreamingOutputCallResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *testServiceClient) HalfDuplexCall(ctx context.Context, opts ...grpc.CallOption) (TestService_HalfDuplexCallClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_TestService_serviceDesc.Streams[3], c.cc, "/grpc.testing.TestService/HalfDuplexCall", opts...)
	if err != nil {
		return nil, err
	}
	x := &testServiceHalfDuplexCallClient{stream}
	return x, nil
}

type TestService_HalfDuplexCallClient interface {
	Send(*StreamingOutputCallRequest) error
	Recv() (*StreamingOutputCallResponse, error)
	grpc.ClientStream
}

type testServiceHalfDuplexCallClient struct {
	grpc.ClientStream
}

func (x *testServiceHalfDuplexCallClient) Send(m *StreamingOutputCallRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *testServiceHalfDuplexCallClient) Recv() (*StreamingOutputCallResponse, error) {
	m := new(StreamingOutputCallResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Server API for TestService service

type TestServiceServer interface {
	// One empty request followed by one empty response.
	EmptyCall(context.Context, *Empty) (*Empty, error)
	// One request followed by one response.
	// The server returns the client payload as-is.
	UnaryCall(context.Context, *SimpleRequest) (*SimpleResponse, error)
	// One request followed by a sequence of responses (streamed download).
	// The server returns the payload with client desired type and sizes.
	StreamingOutputCall(*StreamingOutputCallRequest, TestService_StreamingOutputCallServer) error
	// A sequence of requests followed by one response (streamed upload).
	// The server returns the aggregated size of client payload as the result.
	StreamingInputCall(TestService_StreamingInputCallServer) error
	// A sequence of requests with each request served by the server immediately.
	// As one request could lead to multiple responses, this interface
	// demonstrates the idea of full duplexing.
	FullDuplexCall(TestService_FullDuplexCallServer) error
	// A sequence of requests followed by a sequence of responses.
	// The server buffers all the client requests and then serves them in order. A
	// stream of responses are returned to the client when the server starts with
	// first request.
	HalfDuplexCall(TestService_HalfDuplexCallServer) error
}

func RegisterTestServiceServer(s *grpc.Server, srv TestServiceServer) {
	s.RegisterService(&_TestService_serviceDesc, srv)
}

func _TestService_EmptyCall_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	out, err := srv.(TestServiceServer).EmptyCall(ctx, in)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func _TestService_UnaryCall_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error) (interface{}, error) {
	in := new(SimpleRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	out, err := srv.(TestServiceServer).UnaryCall(ctx, in)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func _TestService_StreamingOutputCall_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(StreamingOutputCallRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(TestServiceServer).StreamingOutputCall(m, &testServiceStreamingOutputCallServer{stream})
}

type TestService_StreamingOutputCallServer interface {
	Send(*StreamingOutputCallResponse) error
	grpc.ServerStream
}

type testServiceStreamingOutputCallServer struct {
	grpc.ServerStream
}

func (x *testServiceStreamingOutputCallServer) Send(m *StreamingOutputCallResponse) error {
	return x.ServerStream.SendMsg(m)
}

func _TestService_StreamingInputCall_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(TestServiceServer).StreamingInputCall(&testServiceStreamingInputCallServer{stream})
}

type TestService_StreamingInputCallServer interface {
	SendAndClose(*StreamingInputCallResponse) error
	Recv() (*StreamingInputCallRequest, error)
	grpc.ServerStream
}

type testServiceStreamingInputCallServer struct {
	grpc.ServerStream
}

func (x *testServiceStreamingInputCallServer) SendAndClose(m *StreamingInputCallResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *testServiceStreamingInputCallServer) Recv() (*StreamingInputCallRequest, error) {
	m := new(StreamingInputCallRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _TestService_FullDuplexCall_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(TestServiceServer).FullDuplexCall(&testServiceFullDuplexCallServer{stream})
}

type TestService_FullDuplexCallServer interface {
	Send(*StreamingOutputCallResponse) error
	Recv() (*StreamingOutputCallRequest, error)
	grpc.ServerStream
}

type testServiceFullDuplexCallServer struct {
	grpc.ServerStream
}

func (x *testServiceFullDuplexCallServer) Send(m *StreamingOutputCallResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *testServiceFullDuplexCallServer) Recv() (*StreamingOutputCallRequest, error) {
	m := new(StreamingOutputCallRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _TestService_HalfDuplexCall_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(TestServiceServer).HalfDuplexCall(&testServiceHalfDuplexCallServer{stream})
}

type TestService_HalfDuplexCallServer interface {
	Send(*StreamingOutputCallResponse) error
	Recv() (*StreamingOutputCallRequest, error)
	grpc.ServerStream
}

type testServiceHalfDuplexCallServer struct {
	grpc.ServerStream
}

func (x *testServiceHalfDuplexCallServer) Send(m *StreamingOutputCallResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *testServiceHalfDuplexCallServer) Recv() (*StreamingOutputCallRequest, error) {
	m := new(StreamingOutputCallRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _TestService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "grpc.testing.TestService",
	HandlerType: (*TestServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "EmptyCall",
			Handler:    _TestService_EmptyCall_Handler,
		},
		{
			MethodName: "UnaryCall",
			Handler:    _TestService_UnaryCall_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamingOutputCall",
			Handler:       _TestService_StreamingOutputCall_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "StreamingInputCall",
			Handler:       _TestService_StreamingInputCall_Handler,
			ClientStreams: true,
		},
		{
			StreamName:    "FullDuplexCall",
			Handler:       _TestService_FullDuplexCall_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "HalfDuplexCall",
			Handler:       _TestService_HalfDuplexCall_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
}

var fileDescriptor0 = []byte{
	// 567 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0xbc, 0x54, 0x51, 0x6f, 0xd2, 0x50,
	0x14, 0xb6, 0x03, 0x64, 0x1c, 0x58, 0x43, 0x0e, 0x59, 0x64, 0x9d, 0x89, 0x4b, 0x7d, 0xb0, 0x9a,
	0x88, 0x86, 0x44, 0x1f, 0x35, 0x73, 0x63, 0x71, 0x09, 0x03, 0x6c, 0xe1, 0x99, 0x5c, 0xe1, 0x0e,
	0x9b, 0x94, 0xb6, 0xb6, 0xb7, 0x46, 0x7c, 0xf0, 0x8f, 0xf9, 0x67, 0xfc, 0x11, 0xfe, 0x00, 0xef,
	0xbd, 0x6d, 0xa1, 0x40, 0x17, 0x99, 0xc6, 0xbd, 0xb5, 0xdf, 0xf9, 0xce, 0x77, 0xbe, 0xef, 0x9e,
	0xdb, 0x02, 0x30, 0x1a, 0xb2, 0x96, 0x1f, 0x78, 0xcc, 0xc3, 0xda, 0x2c, 0xf0, 0x27, 0x2d, 0x01,
	0xd8, 0xee, 0x4c, 0x2f, 0x43, 0xa9, 0x33, 0xf7, 0xd9, 0x42, 0xef, 0x42, 0x79, 0x40, 0x16, 0x8e,
	0x47, 0xa6, 0xf8, 0x1c, 0x8a, 0x6c, 0xe1, 0xd3, 0xa6, 0x72, 0xa2, 0x18, 0x6a, 0xfb, 0xa8, 0x95,
	0x6d, 0x68, 0x25, 0xa4, 0x21, 0x27, 0x98, 0x92, 0x86, 0x08, 0xc5, 0x8f, 0xde, 0x74, 0xd1, 0xdc,
	0xe3, 0xf4, 0x9a, 0x29, 0x9f, 0xf5, 0x5f, 0x0a, 0x1c, 0x58, 0xf6, 0xdc, 0x77, 0xa8, 0x49, 0x3f,
	0x47, 0xbc, 0x15, 0xdf, 0xc0, 0x41, 0x40, 0x43, 0xdf, 0x73, 0x43, 0x3a, 0xde, 0x4d, 0xbd, 0x96,
	0xf2, 0xc5, 0x1b, 0x3e, 0xce, 0xf4, 0x87, 0xf6, 0x37, 0x2a, 0xc7, 0x95, 0x56, 0x24, 0x8b, 0x63,
	0xf8, 0x02, 0xca, 0x7e, 0xac, 0xd0, 0x2c, 0xf0, 0x72, 0xb5, 0x7d, 0x98, 0x2b, 0x6f, 0xa6, 0x2c,
	0xa1, 0x7a, 0x6d, 0x3b, 0xce, 0x38, 0x0a, 0x69, 0xe0, 0x92, 0x39, 0x6d, 0x16, 0x79, 0xdb, 0xbe,
	0x59, 0x13, 0xe0, 0x28, 0xc1, 0xd0, 0x80, 0xba, 0x24, 0x79, 0x24, 0x62, 0x9f, 0xc6, 0xe1, 0xc4,
	0xe3, 0xee, 0x4b, 0x92, 0xa7, 0x0a, 0xbc, 0x2f, 0x60, 0x4b, 0xa0, 0xfa, 0x77, 0x50, 0xd3, 0xd4,
	0xb1, 0xab, 0xac, 0x23, 0x65, 0x27, 0x47, 0x1a, 0xec, 0x2f, 0xcd, 0x88, 0x88, 0x15, 0x73, 0xf9,
	0x8e, 0x8f, 0xa0, 0x9a, 0xf5, 0x50, 0x90, 0x65, 0xf0, 0x56, 0xf3, 0xbb, 0x70, 0x64, 0xb1, 0x80,
	0x92, 0x39, 0x97, 0xbe, 0x74, 0xfd, 0x88, 0x9d, 0x11, 0xc7, 0x49, 0x37, 0x70, 0x5b, 0x2b, 0xfa,
	0x10, 0xb4, 0x3c, 0xb5, 0x24, 0xd9, 0x6b, 0x78, 0x40, 0x66, 0xb3, 0x80, 0xce, 0x08, 0xa3, 0xd3,
	0x71, 0xd2, 0x13, 0xaf, 0x46, 0x91, 0xab, 0x39, 0x5c, 0x95, 0x13, 0x69, 0xb1, 0x23, 0xfd, 0x12,
	0x30, 0xd5, 0x18, 0x90, 0x80, 0xc7, 0x62, 0x34, 0x08, 0xc5, 0x25, 0xca, 0xb4, 0xca, 0x67, 0x11,
	0xd7, 0x76, 0x79, 0xf5, 0x0b, 0x11, 0x0b, 0x4a, 0x16, 0x0e, 0x29, 0x34, 0x0a, 0xf5, 0x9f, 0x4a,
	0xc6, 0x61, 0x3f, 0x62, 0x1b, 0x81, 0xff, 0xf5, 0xca, 0x7d, 0x80, 0xc6, 0xb2, 0xdf, 0x5f, 0x5a,
	0xe5, 0x3e, 0x0a, 0xfc, 0xf0, 0x4e, 0xd6, 0x55, 0xb6, 0x23, 0x99, 0x18, 0x6c, 0xc7, 0xbc, 0xed,
	0x05, 0xd5, 0x7b, 0x70, 0x9c, 0x9b, 0xf0, 0x2f, 0xaf, 0xd7, 0xb3, 0xb7, 0x50, 0xcd, 0x04, 0xc6,
	0x3a, 0xd4, 0xce, 0xfa, 0x57, 0x03, 0xb3, 0x63, 0x59, 0xa7, 0xef, 0xba, 0x9d, 0xfa, 0x3d, 0xbe,
	0x08, 0x75, 0xd4, 0x5b, 0xc3, 0x14, 0x04, 0xb8, 0x6f, 0x9e, 0xf6, 0xce, 0xfb, 0x57, 0xf5, 0xbd,
	0xf6, 0x8f, 0x22, 0x54, 0x87, 0x5c, 0xdd, 0xe2, 0x4b, 0xb0, 0x27, 0x14, 0x5f, 0x41, 0x45, 0xfe,
	0x40, 0x84, 0x2d, 0x6c, 0xac, 0x4f, 0x97, 0x05, 0x2d, 0x0f, 0xc4, 0x0b, 0xa8, 0x8c, 0x5c, 0x12,
	0xc4, 0x6d, 0xc7, 0xeb, 0x8c, 0xb5, 0x1f, 0x87, 0xf6, 0x30, 0xbf, 0x98, 0x1c, 0x80, 0x03, 0x8d,
	0x9c, 0xf3, 0x41, 0x63, 0xa3, 0xe9, 0xc6, 0x4b, 0xa2, 0x3d, 0xdd, 0x81, 0x19, 0xcf, 0x7a, 0xa9,
	0xa0, 0x0d, 0xb8, 0xfd, 0x45, 0xe0, 0x93, 0x1b, 0x24, 0x36, 0xbf, 0x40, 0xcd, 0xf8, 0x33, 0x31,
	0x1e, 0x65, 0x88, 0x51, 0xea, 0x45, 0xe4, 0x38, 0xe7, 0x11, 0x4f, 0xfb, 0xf5, 0xbf, 0x65, 0x32,
	0x14, 0x99, 0x4a, 0x7d, 0x4f, 0x9c, 0xeb, 0x3b, 0x18, 0xf5, 0x3b, 0x00, 0x00, 0xff, 0xff, 0x4c,
	0x41, 0xfe, 0xb6, 0x89, 0x06, 0x00, 0x00,
}
