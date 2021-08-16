package loginendpoint

import (
	"context"
	"time"

	"loginsvc/pkg/loginservice"

	"github.com/go-kit/kit/circuitbreaker"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/ratelimit"
	"github.com/go-kit/kit/tracing/opentracing"
	"github.com/go-kit/kit/tracing/zipkin"
	stdopentracing "github.com/opentracing/opentracing-go"
	stdzipkin "github.com/openzipkin/zipkin-go"
	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"
)

type Set struct {
	LoginEndpoint endpoint.Endpoint
}

func New(svc loginservice.Service, logger log.Logger, duration metrics.Histogram, otTracer stdopentracing.Tracer, zipkinTracer *stdzipkin.Tracer) Set {
	var loginEndpoint endpoint.Endpoint
	{
		loginEndpoint = MakeLoginEndpoint(svc)
		// Sum is limited to 1 request per second with burst of 1 request.
		// Note, rate is defined as a time interval between requests.
		loginEndpoint = ratelimit.NewErroringLimiter(rate.NewLimiter(rate.Every(time.Second), 1))(loginEndpoint)
		loginEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{}))(loginEndpoint)
		loginEndpoint = opentracing.TraceServer(otTracer, "Name")(loginEndpoint)
		if zipkinTracer != nil {
			loginEndpoint = zipkin.TraceEndpoint(zipkinTracer, "Name")(loginEndpoint)
		}
		loginEndpoint = LoggingMiddleware(log.With(logger, "method", "Name"))(loginEndpoint)
		loginEndpoint = InstrumentingMiddleware(duration.With("method", "Name"))(loginEndpoint)
	}
	return Set{
		LoginEndpoint: loginEndpoint,
	}
}

func (s Set) Name(ctx context.Context, n string) (string, error) {
	resp, err := s.LoginEndpoint(ctx, LoginRequest{N: n})
	if err != nil {
		return "", err
	}
	response := resp.(LoginResponse)
	return response.V, response.Err
}

func MakeLoginEndpoint(s loginservice.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(LoginRequest)
		v, err := s.Name(ctx, req.N)
		return LoginResponse{V: v, Err: err}, nil
	}
}

var (
	_ endpoint.Failer = LoginResponse{}
)

type LoginRequest struct {
	N string
}

type LoginResponse struct {
	V   string `json:"v"`
	Err error  `json:"-"`
}

func (r LoginResponse) Failed() error { return r.Err }
