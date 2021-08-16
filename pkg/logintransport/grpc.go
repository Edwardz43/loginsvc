package logintransport

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"

	stdopentracing "github.com/opentracing/opentracing-go"
	stdzipkin "github.com/openzipkin/zipkin-go"
	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"

	"loginsvc/pkg/loginendpoint"

	"github.com/go-kit/kit/circuitbreaker"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/ratelimit"
	"github.com/go-kit/kit/tracing/opentracing"
	"github.com/go-kit/kit/tracing/zipkin"
	"github.com/go-kit/kit/transport"
	grpctransport "github.com/go-kit/kit/transport/grpc"

	pb "loginsvc/pb"
	"loginsvc/pkg/loginservice"
)

type grpcServer struct {
	name grpctransport.Handler
	pb.UnimplementedLoginServer
}

func (s *grpcServer) Name(ctx context.Context, req *pb.NameRequest) (*pb.NameReply, error) {
	_, rep, err := s.name.ServeGRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return rep.(*pb.NameReply), nil
}

func (s *grpcServer) mustEmbedUnimplementedLoginServer() {}

// NewGRPCServer makes a set of endpoints available as a gRPC LoginServer.
func NewGRPCServer(endpoints loginendpoint.Set, otTracer stdopentracing.Tracer, zipkinTracer *stdzipkin.Tracer, logger log.Logger) pb.LoginServer {
	options := []grpctransport.ServerOption{
		grpctransport.ServerErrorHandler(transport.NewLogErrorHandler(logger)),
	}

	if zipkinTracer != nil {
		// Zipkin GRPC Server Trace can either be instantiated per gRPC method with a
		// provided operation name or a global tracing service can be instantiated
		// without an operation name and fed to each Go kit gRPC server as a
		// ServerOption.
		// In the latter case, the operation name will be the endpoint's grpc method
		// path if used in combination with the Go kit gRPC Interceptor.
		//
		// In this example, we demonstrate a global Zipkin tracing service with
		// Go kit gRPC Interceptor.
		options = append(options, zipkin.GRPCServerTrace(zipkinTracer))
	}

	g := &grpcServer{
		name: grpctransport.NewServer(
			endpoints.LoginEndpoint,
			decodeGRPCNameRequest,
			encodeGRPCNameResponse,
			append(options, grpctransport.ServerBefore(opentracing.GRPCToContext(otTracer, "Name", logger)))...,
		),
	}
	return g
}

// NewGRPCClient returns an LoginService backed by a gRPC server at the other end
// of the conn. The caller is responsible for constructing the conn, and
// eventually closing the underlying transport. We bake-in certain middlewares,
// implementing the client library pattern.
func NewGRPCClient(conn *grpc.ClientConn, otTracer stdopentracing.Tracer, zipkinTracer *stdzipkin.Tracer, logger log.Logger) loginservice.Service {
	// We construct a single ratelimiter middleware, to limit the total outgoing
	// QPS from this client to all methods on the remote instance. We also
	// construct per-endpoint circuitbreaker middlewares to demonstrate how
	// that's done, although they could easily be combined into a single breaker
	// for the entire remote instance, too.
	limiter := ratelimit.NewErroringLimiter(rate.NewLimiter(rate.Every(time.Second), 100))

	// global client middlewares
	var options []grpctransport.ClientOption

	if zipkinTracer != nil {
		// Zipkin GRPC Client Trace can either be instantiated per gRPC method with a
		// provided operation name or a global tracing client can be instantiated
		// without an operation name and fed to each Go kit client as ClientOption.
		// In the latter case, the operation name will be the endpoint's grpc method
		// path.
		//
		// In this example, we demonstrace a global tracing client.
		options = append(options, zipkin.GRPCClientTrace(zipkinTracer))

	}
	// Each individual endpoint is an grpc/transport.Client (which implements
	// endpoint.Endpoint) that gets wrapped with various middlewares. If you
	// made your own client library, you'd do this work there, so your server
	// could rely on a consistent set of client behavior.
	var nameEndpoint endpoint.Endpoint
	{
		nameEndpoint = grpctransport.NewClient(
			conn,
			"pb.Login",
			"Name",
			encodeGRPCNameRequest,
			decodeGRPCNameResponse,
			pb.NameReply{},
			append(options, grpctransport.ClientBefore(opentracing.ContextToGRPC(otTracer, logger)))...,
		).Endpoint()
		nameEndpoint = opentracing.TraceClient(otTracer, "Name")(nameEndpoint)
		nameEndpoint = limiter(nameEndpoint)
		nameEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "Name",
			Timeout: 30 * time.Second,
		}))(nameEndpoint)
	}

	// Returning the endpoint.Set as a service.Service relies on the
	// endpoint.Set implementing the Service methods. That's just a simple bit
	// of glue code.
	return loginendpoint.Set{
		LoginEndpoint: nameEndpoint,
	}
}

// decodeGRPCNameRequest is a transport/grpc.DecodeRequestFunc that converts a
// gRPC name request to a user-domain name request. Primarily useful in a server.
func decodeGRPCNameRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*pb.NameRequest)
	return loginendpoint.LoginRequest{N: string(req.N)}, nil
}

// decodeGRPCNameResponse is a transport/grpc.DecodeResponseFunc that converts a
// gRPC name reply to a user-domain name response. Primarily useful in a client.
func decodeGRPCNameResponse(_ context.Context, grpcReply interface{}) (interface{}, error) {
	reply := grpcReply.(*pb.NameReply)
	return loginendpoint.LoginResponse{V: string(reply.V), Err: str2err(reply.Err)}, nil
}

// encodeGRPCConcatResponse is a transport/grpc.EncodeResponseFunc that converts
// a user-domain concat response to a gRPC concat reply. Primarily useful in a
// server.
func encodeGRPCNameResponse(_ context.Context, response interface{}) (interface{}, error) {
	resp := response.(loginendpoint.LoginResponse)
	return &pb.NameReply{V: resp.V, Err: err2str(resp.Err)}, nil
}

// encodeGRPCNameRequest is a transport/grpc.EncodeRequestFunc that converts a
// user-domain name request to a gRPC name request. Primarily useful in a client.
func encodeGRPCNameRequest(_ context.Context, request interface{}) (interface{}, error) {
	req := request.(loginendpoint.LoginRequest)
	return &pb.NameRequest{N: string(req.N)}, nil
}

// encodeGRPCConcatRequest is a transport/grpc.EncodeRequestFunc that converts a
// user-domain concat request to a gRPC concat request. Primarily useful in a
// client.
func encodeGRPCConcatRequest(_ context.Context, request interface{}) (interface{}, error) {
	req := request.(loginendpoint.LoginRequest)
	return &pb.NameRequest{N: req.N}, nil
}

// These annoying helper functions are required to translate Go error types to
// and from strings, which is the type we use in our IDLs to represent errors.
// There is special casing to treat empty strings as nil errors.

func str2err(s string) error {
	if s == "" {
		return nil
	}
	return errors.New(s)
}

func err2str(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
