package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	tp := initTracer()
	defer func() { _ = tp.Shutdown(ctx) }()

	tracer = otel.Tracer("checkout-service")

	mux := http.NewServeMux()
	mux.HandleFunc("/checkout", handleCheckout)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	wrapped := otelhttp.NewHandler(mux, "checkout")
	server := &http.Server{Addr: ":8083", Handler: wrapped}

	go func() {
		log.Println("checkout service listening on :8083")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	server.Shutdown(context.Background())
}

func initTracer() *sdktrace.TracerProvider {
	ctx := context.Background()

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint("otel-collector:4317"),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatalf("failed to create exporter: %v", err)
	}

	res, _ := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("checkout"),
			semconv.ServiceVersion("1.0.0"),
			semconv.DeploymentEnvironment("development"),
			attribute.String("k8s.namespace", "demo"),
		),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return tp
}

func handleCheckout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := tracer.Start(ctx, "process_checkout",
		trace.WithAttributes(
			attribute.String("user.id", fmt.Sprintf("user_%d", rand.Intn(1000))),
		),
	)
	defer span.End()

	// Simulate sub-operations
	time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)
	validateCart(ctx)
	processPayment(ctx)
	createOrder(ctx)

	// 10% chance of error to generate interesting traces
	if rand.Float32() < 0.10 {
		span.SetAttributes(attribute.String("error.type", "payment_declined"))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"payment declined"}`))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success","order_id":"ord_` + fmt.Sprint(rand.Intn(100000)) + `"}`))
}

func validateCart(ctx context.Context) {
	_, span := tracer.Start(ctx, "validate_cart")
	defer span.End()
	time.Sleep(time.Duration(20+rand.Intn(40)) * time.Millisecond)
}

func processPayment(ctx context.Context) {
	_, span := tracer.Start(ctx, "process_payment")
	defer span.End()
	time.Sleep(time.Duration(80+rand.Intn(120)) * time.Millisecond)
	span.SetAttributes(attribute.String("payment.provider", "stripe"))
}

func createOrder(ctx context.Context) {
	_, span := tracer.Start(ctx, "create_order")
	defer span.End()
	time.Sleep(time.Duration(30+rand.Intn(50)) * time.Millisecond)
}
