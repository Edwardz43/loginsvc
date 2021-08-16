package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"google.golang.org/grpc"

	lightstep "github.com/lightstep/lightstep-tracer-go"
	stdopentracing "github.com/opentracing/opentracing-go"
	zipkinot "github.com/openzipkin-contrib/zipkin-go-opentracing"
	zipkin "github.com/openzipkin/zipkin-go"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
	"sourcegraph.com/sourcegraph/appdash"
	appdashot "sourcegraph.com/sourcegraph/appdash/opentracing"

	"loginsvc/pkg/loginservice"
	"loginsvc/pkg/logintransport"

	"github.com/go-kit/kit/log"
	// addthrift "github.com/go-kit/examples/addsvc/thrift/gen-go/addsvc"
)

func main() {
	// The addcli presumes no service discovery system, and expects users to
	// provide the direct address of an addsvc. This presumption is reflected in
	// the addcli binary and the client packages: the -transport.addr flags
	// and various client constructors both expect host:port strings. For an
	// example service with a client built on top of a service discovery system,
	// see profilesvc.
	fs := flag.NewFlagSet("addcli", flag.ExitOnError)
	var (
		httpAddr = fs.String("http-addr", "", "HTTP address of addsvc")
		grpcAddr = fs.String("grpc-addr", "", "gRPC address of addsvc")
		// thriftAddr     = fs.String("thrift-addr", "", "Thrift address of addsvc")
		// jsonRPCAddr = fs.String("jsonrpc-addr", "", "JSON RPC address of addsvc")
		// thriftProtocol = fs.String("thrift-protocol", "binary", "binary, compact, json, simplejson")
		// thriftBuffer   = fs.Int("thrift-buffer", 0, "0 for unbuffered")
		// thriftFramed   = fs.Bool("thrift-framed", false, "true to enable framing")
		zipkinURL      = fs.String("zipkin-url", "", "Enable Zipkin tracing via HTTP reporter URL e.g. http://localhost:9411/api/v2/spans")
		zipkinBridge   = fs.Bool("zipkin-ot-bridge", false, "Use Zipkin OpenTracing bridge instead of native implementation")
		lightstepToken = fs.String("lightstep-token", "", "Enable LightStep tracing via a LightStep access token")
		appdashAddr    = fs.String("appdash-addr", "", "Enable Appdash tracing via an Appdash server host:port")
		method         = fs.String("method", "name", "name")
	)
	fs.Usage = usageFor(fs, os.Args[0]+" [flags] <n>")
	fs.Parse(os.Args[1:])
	if len(fs.Args()) == 0 {
		fs.Usage()
		os.Exit(1)
	}

	// This is a demonstration of the native Zipkin tracing client. If using
	// Zipkin this is the more idiomatic client over OpenTracing.
	var zipkinTracer *zipkin.Tracer
	{
		if *zipkinURL != "" {
			var (
				err         error
				hostPort    = "" // if host:port is unknown we can keep this empty
				serviceName = "addsvc-cli"
				reporter    = zipkinhttp.NewReporter(*zipkinURL)
			)
			defer reporter.Close()
			zEP, _ := zipkin.NewEndpoint(serviceName, hostPort)
			zipkinTracer, err = zipkin.NewTracer(reporter, zipkin.WithLocalEndpoint(zEP))
			if err != nil {
				fmt.Fprintf(os.Stderr, "unable to create zipkin tracer: %s\n", err.Error())
				os.Exit(1)
			}
		}
	}

	// This is a demonstration client, which supports multiple tracers.
	// Your clients will probably just use one tracer.
	var otTracer stdopentracing.Tracer
	{
		if *zipkinBridge && zipkinTracer != nil {
			otTracer = zipkinot.Wrap(zipkinTracer)
			zipkinTracer = nil // do not instrument with both native and ot bridge
		} else if *lightstepToken != "" {
			otTracer = lightstep.NewTracer(lightstep.Options{
				AccessToken: *lightstepToken,
			})
			defer lightstep.FlushLightStepTracer(otTracer)
		} else if *appdashAddr != "" {
			otTracer = appdashot.NewTracer(appdash.NewRemoteCollector(*appdashAddr))
		} else {
			otTracer = stdopentracing.GlobalTracer() // no-op
		}
	}

	// This is a demonstration client, which supports multiple transports.
	// Your clients will probably just define and stick with 1 transport.
	var (
		svc loginservice.Service
		err error
	)
	if *httpAddr != "" {
		svc, err = logintransport.NewHTTPClient(*httpAddr, otTracer, zipkinTracer, log.NewNopLogger())
	} else if *grpcAddr != "" {
		// conn, err := grpc.Dial(*grpcAddr, grpc.WithInsecure(), grpc.WithTimeout(time.Second))
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		conn, err := grpc.DialContext(ctx, *grpcAddr, grpc.WithInsecure())
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v", err)
			cancel()
			os.Exit(1)
		}
		defer conn.Close()
		svc = logintransport.NewGRPCClient(conn, otTracer, zipkinTracer, log.NewNopLogger())
		// } else if *jsonRPCAddr != "" {
		// svc, err = logintransport.NewJSONRPCClient(*jsonRPCAddr, otTracer, log.NewNopLogger())
		// } else if *thriftAddr != "" {
		// 	// It's necessary to do all of this construction in the func main,
		// 	// because (among other reasons) we need to control the lifecycle of the
		// 	// Thrift transport, i.e. close it eventually.
		// 	var protocolFactory thrift.TProtocolFactory
		// 	switch *thriftProtocol {
		// 	case "compact":
		// 		protocolFactory = thrift.NewTCompactProtocolFactory()
		// 	case "simplejson":
		// 		protocolFactory = thrift.NewTSimpleJSONProtocolFactory()
		// 	case "json":
		// 		protocolFactory = thrift.NewTJSONProtocolFactory()
		// 	case "binary", "":
		// 		protocolFactory = thrift.NewTBinaryProtocolFactoryDefault()
		// 	default:
		// 		fmt.Fprintf(os.Stderr, "error: invalid protocol %q\n", *thriftProtocol)
		// 		os.Exit(1)
		// 	}
		// 	var transportFactory thrift.TTransportFactory
		// 	if *thriftBuffer > 0 {
		// 		transportFactory = thrift.NewTBufferedTransportFactory(*thriftBuffer)
		// 	} else {
		// 		transportFactory = thrift.NewTTransportFactory()
		// 	}
		// 	if *thriftFramed {
		// 		transportFactory = thrift.NewTFramedTransportFactory(transportFactory)
		// 	}
		// 	transportSocket, err := thrift.NewTSocket(*thriftAddr)
		// 	if err != nil {
		// 		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		// 		os.Exit(1)
		// 	}
		// 	transport, err := transportFactory.GetTransport(transportSocket)
		// 	if err != nil {
		// 		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		// 		os.Exit(1)
		// 	}
		// 	if err := transport.Open(); err != nil {
		// 		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		// 		os.Exit(1)
		// 	}
		// 	defer transport.Close()
		// 	// client := addthrift.NewAddServiceClientFactory(transport, protocolFactory)
		// 	svc = addtransport.NewThriftClient(client)
	} else {
		fmt.Fprintf(os.Stderr, "error: no remote address specified\n")
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	switch *method {
	case "name":
		n := fs.Args()[0]
		v, err := svc.Name(context.Background(), string(n))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "name: %s, sid: %s\n", n, v)

	default:
		fmt.Fprintf(os.Stderr, "error: invalid method %q\n", *method)
		os.Exit(1)
	}
}

func usageFor(fs *flag.FlagSet, short string) func() {
	return func() {
		fmt.Fprintf(os.Stderr, "USAGE\n")
		fmt.Fprintf(os.Stderr, "  %s\n", short)
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		w := tabwriter.NewWriter(os.Stderr, 0, 2, 2, ' ', 0)
		fs.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(w, "\t-%s %s\t%s\n", f.Name, f.DefValue, f.Usage)
		})
		w.Flush()
		fmt.Fprintf(os.Stderr, "\n")
	}
}
