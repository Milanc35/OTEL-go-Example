package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/milan/OTEL-go-demo/logger"
	"github.com/segmentio/kafka-go"
	"go.elastic.co/apm/module/apmgorilla/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

var (
	appName = "ordersvcs"
	tracer  = otel.Tracer("ordersvcs")
	meter   = otel.Meter("debug")
	rollCnt metric.Int64Counter
	writerK = &kafka.Writer{
		Addr:  kafka.TCP(os.Getenv("KAFKA_BROKER_URL")),
		Topic: os.Getenv("KAFKA_TOPIC"),
	}
)

func handleErr(err error, message string) {
	if err != nil {
		panic(fmt.Sprintf("%s: %s", err, message))
	}
}

/***
Tracing
***/
// Initializes an OTLP exporter, and configures the trace provider
func initTracer() func() {
	ctx := context.Background()

	driver := otlpgrpc.NewDriver(
		otlpgrpc.WithInsecure(),
		otlpgrpc.WithEndpoint("tempo:55680"),
		otlpgrpc.WithDialOption(otlpgrpc.WithBlock()), // useful for testing
	)
	exp, err := otlp.NewExporter(ctx, driver)
	handleErr(err, "failed to create exporter")

	res, err := resource.New(ctx,
		resource.WithAttributes(
			// the service name used to display traces in backends
			semconv.ServiceNameKey.String("demo-service"),
		),
	)
	handleErr(err, "failed to create resource")

	bsp := sdktrace.NewBatchSpanProcessor(exp)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})
	otel.SetTracerProvider(tracerProvider)

	return func() {
		// Shutdown will flush any remaining spans.
		handleErr(tracerProvider.Shutdown(ctx), "failed to shutdown TracerProvider")
	}
}

func main() {
	err := logger.InitLogger("debug", appName, "dev")
	if err != nil {
		fmt.Println("Failed to init  log")
		return
	}
	logger.LogHandle.Info("Starting server...")
	apmUrl := os.Getenv("ELASTIC_APM_SERVER_URL")
	fmt.Println(apmUrl)
	r := mux.NewRouter()
	apmgorilla.Instrument(r)
	r.HandleFunc("/order", placeNewOrder)

	logger.LogHandle.Fatal(http.ListenAndServe(os.Getenv("PORT"), r))
}
