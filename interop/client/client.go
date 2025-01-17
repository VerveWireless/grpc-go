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

package main

import (
	"flag"
	"net"
	"strconv"

	"github.com/VerveWireless/grpc-go"
	"github.com/VerveWireless/grpc-go/credentials"
	"github.com/VerveWireless/grpc-go/credentials/oauth"
	"github.com/VerveWireless/grpc-go/grpclog"
	"github.com/VerveWireless/grpc-go/interop"
	testpb "github.com/VerveWireless/grpc-go/interop/grpc_testing"
)

var (
	useTLS                = flag.Bool("use_tls", false, "Connection uses TLS if true, else plain TCP")
	testCA                = flag.Bool("use_test_ca", false, "Whether to replace platform root CAs with test CA as the CA root")
	serviceAccountKeyFile = flag.String("service_account_key_file", "", "Path to service account json key file")
	oauthScope            = flag.String("oauth_scope", "", "The scope for OAuth2 tokens")
	defaultServiceAccount = flag.String("default_service_account", "", "Email of GCE default service account")
	serverHost            = flag.String("server_host", "127.0.0.1", "The server host name")
	serverPort            = flag.Int("server_port", 10000, "The server port number")
	tlsServerName         = flag.String("server_host_override", "x.test.youtube.com", "The server name use to verify the hostname returned by TLS handshake if it is not empty. Otherwise, --server_host is used.")
	testCase              = flag.String("test_case", "large_unary",
		`Configure different test cases. Valid options are:
        empty_unary : empty (zero bytes) request and response;
        large_unary : single request and (large) response;
        client_streaming : request streaming with single response;
        server_streaming : single request with response streaming;
        ping_pong : full-duplex streaming;
        empty_stream : full-duplex streaming with zero message;
        timeout_on_sleeping_server: fullduplex streaming on a sleeping server;
        compute_engine_creds: large_unary with compute engine auth;
        service_account_creds: large_unary with service account auth;
        jwt_token_creds: large_unary with jwt token auth;
        per_rpc_creds: large_unary with per rpc token;
        oauth2_auth_token: large_unary with oauth2 token auth;
        cancel_after_begin: cancellation after metadata has been sent but before payloads are sent;
        cancel_after_first_response: cancellation after receiving 1st message from the server.`)

	// The test CA root cert file
	testCAFile = "testdata/ca.pem"
)

func main() {
	flag.Parse()
	serverAddr := net.JoinHostPort(*serverHost, strconv.Itoa(*serverPort))
	var opts []grpc.DialOption
	if *useTLS {
		var sn string
		if *tlsServerName != "" {
			sn = *tlsServerName
		}
		var creds credentials.TransportAuthenticator
		if *testCA {
			var err error
			creds, err = credentials.NewClientTLSFromFile(testCAFile, sn)
			if err != nil {
				grpclog.Fatalf("Failed to create TLS credentials %v", err)
			}
		} else {
			creds = credentials.NewClientTLSFromCert(nil, sn)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
		if *testCase == "compute_engine_creds" {
			opts = append(opts, grpc.WithPerRPCCredentials(oauth.NewComputeEngine()))
		} else if *testCase == "service_account_creds" {
			jwtCreds, err := oauth.NewServiceAccountFromFile(*serviceAccountKeyFile, *oauthScope)
			if err != nil {
				grpclog.Fatalf("Failed to create JWT credentials: %v", err)
			}
			opts = append(opts, grpc.WithPerRPCCredentials(jwtCreds))
		} else if *testCase == "jwt_token_creds" {
			jwtCreds, err := oauth.NewJWTAccessFromFile(*serviceAccountKeyFile)
			if err != nil {
				grpclog.Fatalf("Failed to create JWT credentials: %v", err)
			}
			opts = append(opts, grpc.WithPerRPCCredentials(jwtCreds))
		} else if *testCase == "oauth2_auth_token" {
			opts = append(opts, grpc.WithPerRPCCredentials(oauth.NewOauthAccess(interop.GetToken(*serviceAccountKeyFile, *oauthScope))))
		}
	} else {
		opts = append(opts, grpc.WithInsecure())
	}
	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		grpclog.Fatalf("Fail to dial: %v", err)
	}
	defer conn.Close()
	tc := testpb.NewTestServiceClient(conn)
	switch *testCase {
	case "empty_unary":
		interop.DoEmptyUnaryCall(tc)
	case "large_unary":
		interop.DoLargeUnaryCall(tc)
	case "client_streaming":
		interop.DoClientStreaming(tc)
	case "server_streaming":
		interop.DoServerStreaming(tc)
	case "ping_pong":
		interop.DoPingPong(tc)
	case "empty_stream":
		interop.DoEmptyStream(tc)
	case "timeout_on_sleeping_server":
		interop.DoTimeoutOnSleepingServer(tc)
	case "compute_engine_creds":
		if !*useTLS {
			grpclog.Fatalf("TLS is not enabled. TLS is required to execute compute_engine_creds test case.")
		}
		interop.DoComputeEngineCreds(tc, *defaultServiceAccount, *oauthScope)
	case "service_account_creds":
		if !*useTLS {
			grpclog.Fatalf("TLS is not enabled. TLS is required to execute service_account_creds test case.")
		}
		interop.DoServiceAccountCreds(tc, *serviceAccountKeyFile, *oauthScope)
	case "jwt_token_creds":
		if !*useTLS {
			grpclog.Fatalf("TLS is not enabled. TLS is required to execute jwt_token_creds test case.")
		}
		interop.DoJWTTokenCreds(tc, *serviceAccountKeyFile)
	case "per_rpc_creds":
		if !*useTLS {
			grpclog.Fatalf("TLS is not enabled. TLS is required to execute per_rpc_creds test case.")
		}
		interop.DoPerRPCCreds(tc, *serviceAccountKeyFile, *oauthScope)
	case "oauth2_auth_token":
		if !*useTLS {
			grpclog.Fatalf("TLS is not enabled. TLS is required to execute oauth2_auth_token test case.")
		}
		interop.DoOauth2TokenCreds(tc, *serviceAccountKeyFile, *oauthScope)
	case "cancel_after_begin":
		interop.DoCancelAfterBegin(tc)
	case "cancel_after_first_response":
		interop.DoCancelAfterFirstResponse(tc)
	default:
		grpclog.Fatal("Unsupported test case: ", *testCase)
	}
}
