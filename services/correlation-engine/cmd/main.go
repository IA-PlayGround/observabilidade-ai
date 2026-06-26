package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	config := loadConfig()

	tp := initTracer(ctx, config)
	defer func() { _ = tp.Shutdown(ctx) }()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	log.Println("connected to redis")

	correlator := NewCorrelator(redisClient, config)
	engine := NewEngine(correlator, config)

	go consumeTraces(ctx, engine, config)
	go consumeMetrics(ctx, engine, config)
	go consumeLogs(ctx, engine, config)

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(30 * time.Second))
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	router.Handle("/metrics", promhttp.Handler())
	router.Get("/health", healthHandler)
	router.Get("/api/v1/correlation/{traceId}", engine.handleGetCorrelation)
	router.Post("/api/v1/correlation/query", engine.handleQueryCorrelation)
	router.Get("/api/v1/correlation/service/{service}", engine.handleGetByService)

	addr := fmt.Sprintf(":%d", config.Port)
	log.Printf("correlation engine starting on %s", addr)

	server := &http.Server{Addr: addr, Handler: router}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx)
}

type Config struct {
	Port              int
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	KafkaBrokers      []string
	CorrelationWindow time.Duration
	PrometheusURL     string
	TempoURL          string
	LokiURL           string
}

func loadConfig() Config {
	port, _ := strconv.Atoi(getEnv("CORRELATION_PORT", "8081"))
	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	window, _ := strconv.Atoi(getEnv("CORRELATION_WINDOW_SECONDS", "120"))

	return Config{
		Port:              port,
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		RedisDB:           redisDB,
		KafkaBrokers:      []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
		CorrelationWindow: time.Duration(window) * time.Second,
		PrometheusURL:     getEnv("PROMETHEUS_URL", "http://localhost:9090"),
		TempoURL:          getEnv("TEMPO_URL", "http://localhost:3200"),
		LokiURL:           getEnv("LOKI_URL", "http://localhost:3100"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func initTracer(ctx context.Context, config Config) *sdktrace.TracerProvider {
	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		log.Printf("warning: failed to create OTLP exporter: %v", err)
		return sdktrace.NewTracerProvider()
	}

	res, _ := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("correlation-engine"),
			semconv.ServiceVersion("1.0.0"),
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

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// Telemetry data structures
type TelemetryTrace struct {
	TraceID      string            `json:"traceId"`
	SpanID       string            `json:"spanId"`
	ParentSpanID string            `json:"parentSpanId,omitempty"`
	Name         string            `json:"name"`
	ServiceName  string            `json:"serviceName"`
	Timestamp    time.Time         `json:"timestamp"`
	Duration     time.Duration     `json:"duration"`
	StatusCode   int               `json:"statusCode"`
	StatusMsg    string            `json:"statusMessage,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
	Events       []SpanEvent       `json:"events,omitempty"`
}

type SpanEvent struct {
	Name       string            `json:"name"`
	Timestamp  time.Time         `json:"timestamp"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type TelemetryMetric struct {
	Name       string            `json:"name"`
	Service    string            `json:"service"`
	Value      float64           `json:"value"`
	Timestamp  time.Time         `json:"timestamp"`
	Labels     map[string]string `json:"labels"`
	TraceID    string            `json:"traceId,omitempty"`
	SpanID     string            `json:"spanId,omitempty"`
}

type TelemetryLog struct {
	Timestamp   time.Time         `json:"timestamp"`
	ServiceName string            `json:"serviceName"`
	Severity    string            `json:"severity"`
	Body        string            `json:"body"`
	TraceID     string            `json:"traceId"`
	SpanID      string            `json:"spanId"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

// Correlation result
type CorrelationResult struct {
	TraceID     string            `json:"traceId"`
	RootSpanID  string            `json:"rootSpanId,omitempty"`
	ServiceName string            `json:"serviceName"`
	Spans       []TelemetryTrace  `json:"spans"`
	Metrics     []TelemetryMetric `json:"metrics"`
	Logs        []TelemetryLog    `json:"logs"`
	Correlated  time.Time         `json:"correlatedAt"`
}

// Correlator stores data in Redis
type Correlator struct {
	redis  *redis.Client
	config Config
}

func NewCorrelator(client *redis.Client, config Config) *Correlator {
	return &Correlator{redis: client, config: config}
}

func (c *Correlator) IndexTrace(ctx context.Context, trace TelemetryTrace) error {
	key := fmt.Sprintf("trace:%s", trace.TraceID)
	traceJSON, _ := json.Marshal(trace)

	pipe := c.redis.Pipeline()
	pipe.SAdd(ctx, key+":spans", string(traceJSON))
	pipe.SAdd(ctx, fmt.Sprintf("service:%s:traces", trace.ServiceName), trace.TraceID)
	pipe.Expire(ctx, key+":spans", 24*time.Hour)
	pipe.Expire(ctx, fmt.Sprintf("service:%s:traces", trace.ServiceName), 24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Correlator) IndexMetric(ctx context.Context, metric TelemetryMetric) error {
	if metric.TraceID == "" {
		return nil
	}
	key := fmt.Sprintf("trace:%s", metric.TraceID)
	metricJSON, _ := json.Marshal(metric)
	pipe := c.redis.Pipeline()
	pipe.SAdd(ctx, key+":metrics", string(metricJSON))
	pipe.Expire(ctx, key+":metrics", 24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Correlator) IndexLog(ctx context.Context, log TelemetryLog) error {
	if log.TraceID == "" {
		return nil
	}
	key := fmt.Sprintf("trace:%s", log.TraceID)
	logJSON, _ := json.Marshal(log)
	pipe := c.redis.Pipeline()
	pipe.SAdd(ctx, key+":logs", string(logJSON))
	pipe.Expire(ctx, key+":logs", 24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Correlator) GetCorrelation(ctx context.Context, traceID string) (*CorrelationResult, error) {
	key := fmt.Sprintf("trace:%s", traceID)

	spansData, err := c.redis.SMembers(ctx, key+":spans").Result()
	if err != nil {
		return nil, fmt.Errorf("no trace data for %s", traceID)
	}

	result := &CorrelationResult{
		TraceID:    traceID,
		Correlated: time.Now(),
	}

	for _, data := range spansData {
		var span TelemetryTrace
		if json.Unmarshal([]byte(data), &span) == nil {
			result.Spans = append(result.Spans, span)
			if span.ParentSpanID == "" {
				result.RootSpanID = span.SpanID
				result.ServiceName = span.ServiceName
			}
		}
	}

	metricsData, _ := c.redis.SMembers(ctx, key+":metrics").Result()
	for _, data := range metricsData {
		var metric TelemetryMetric
		if json.Unmarshal([]byte(data), &metric) == nil {
			result.Metrics = append(result.Metrics, metric)
		}
	}

	logsData, _ := c.redis.SMembers(ctx, key+":logs").Result()
	for _, data := range logsData {
		var log TelemetryLog
		if json.Unmarshal([]byte(data), &log) == nil {
			result.Logs = append(result.Logs, log)
		}
	}

	return result, nil
}

// Engine coordinates correlation and API
type Engine struct {
	correlator *Correlator
	config     Config
}

func NewEngine(correlator *Correlator, config Config) *Engine {
	return &Engine{correlator: correlator, config: config}
}

func (e *Engine) handleGetCorrelation(w http.ResponseWriter, r *http.Request) {
	traceID := chi.URLParam(r, "traceId")
	ctx := r.Context()

	result, err := e.correlator.GetCorrelation(ctx, traceID)
	if err != nil {
		http.Error(w, `{"error":"trace not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (e *Engine) handleQueryCorrelation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TraceID string `json:"traceId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	result, err := e.correlator.GetCorrelation(r.Context(), req.TraceID)
	if err != nil {
		http.Error(w, `{"error":"trace not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (e *Engine) handleGetByService(w http.ResponseWriter, r *http.Request) {
	service := chi.URLParam(r, "service")
	ctx := r.Context()

	traceIDs, err := e.correlator.redis.SMembers(ctx, fmt.Sprintf("service:%s:traces", service)).Result()
	if err != nil {
		http.Error(w, `{"error":"no traces found"}`, http.StatusNotFound)
		return
	}

	correlations := make([]CorrelationResult, 0, len(traceIDs))
	for _, traceID := range traceIDs {
		result, err := e.correlator.GetCorrelation(ctx, traceID)
		if err == nil {
			correlations = append(correlations, *result)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":       service,
		"correlations":  correlations,
		"traceCount":    len(traceIDs),
	})
}

// Kafka consumers
func consumeTraces(ctx context.Context, engine *Engine, config Config) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: config.KafkaBrokers,
		Topic:   "otlp.traces",
		GroupID: "correlation-engine-traces",
		MaxWait: time.Second,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	log.Println("kafka traces consumer started")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("error reading trace from kafka: %v", err)
				continue
			}

			var trace TelemetryTrace
			if err := json.Unmarshal(msg.Value, &trace); err != nil {
				log.Printf("error unmarshalling trace: %v", err)
				continue
			}

			if err := engine.correlator.IndexTrace(ctx, trace); err != nil {
				log.Printf("error indexing trace: %v", err)
			}
		}
	}
}

func consumeMetrics(ctx context.Context, engine *Engine, config Config) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: config.KafkaBrokers,
		Topic:   "otlp.metrics",
		GroupID: "correlation-engine-metrics",
		MaxWait: time.Second,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	log.Println("kafka metrics consumer started")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("error reading metric from kafka: %v", err)
				continue
			}

			var metric TelemetryMetric
			if err := json.Unmarshal(msg.Value, &metric); err != nil {
				continue
			}

			engine.correlator.IndexMetric(ctx, metric)
		}
	}
}

func consumeLogs(ctx context.Context, engine *Engine, config Config) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: config.KafkaBrokers,
		Topic:   "otlp.logs",
		GroupID: "correlation-engine-logs",
		MaxWait: time.Second,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	log.Println("kafka logs consumer started")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("error reading log from kafka: %v", err)
				continue
			}

			var logEntry TelemetryLog
			if err := json.Unmarshal(msg.Value, &logEntry); err != nil {
				continue
			}

			engine.correlator.IndexLog(ctx, logEntry)
		}
	}
}
