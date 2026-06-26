package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	config := loadAIConfig()
	engine := NewAIEngine(config)

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(120 * time.Second))
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
		MaxAge:         300,
	}))

	router.Handle("/metrics", promhttp.Handler())
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	// MCP endpoints — Model Context Protocol
	router.Post("/mcp/diagnose", engine.handleDiagnose)
	router.Post("/mcp/report", engine.handleGenerateReport)
	router.Post("/mcp/release-analysis", engine.handleReleaseAnalysis)
	router.Post("/mcp/query", engine.handleAdHocQuery)

	addr := fmt.Sprintf(":%s", config.Port)
	log.Printf("AI engine starting on %s (backend: %s)", addr, config.LLMBackend)

	server := &http.Server{Addr: addr, Handler: router}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down AI engine...")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	server.Shutdown(shutdownCtx)
}

// ── MCP Types ──────────────────────────────────────────────────────

type MCPContext struct {
	Incident  IncidentContext  `json:"incident"`
	Telemetry TelemetryContext `json:"telemetry"`
	Action    string           `json:"action"`
}

type IncidentContext struct {
	AlertName    string    `json:"alertName"`
	ServiceName  string    `json:"serviceName"`
	Severity     string    `json:"severity"`
	TraceID      string    `json:"traceId"`
	StartTime    time.Time `json:"startTime"`
	Description  string    `json:"description"`
}

type TelemetryContext struct {
	Metrics    []MetricSnapshot `json:"metrics"`
	Spans      []SpanSnapshot   `json:"spans"`
	Logs       []LogSnapshot    `json:"logs"`
	GrafanaURL string           `json:"grafanaUrl,omitempty"`
}

type MetricSnapshot struct {
	Name      string            `json:"name"`
	Service   string            `json:"service"`
	Value     float64           `json:"value"`
	Unit      string            `json:"unit"`
	Timestamp time.Time         `json:"timestamp"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type SpanSnapshot struct {
	Name        string            `json:"name"`
	ServiceName string            `json:"serviceName"`
	Duration    string            `json:"duration"`
	StatusCode  int               `json:"statusCode"`
	StatusMsg   string            `json:"statusMessage,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

type LogSnapshot struct {
	Timestamp time.Time `json:"timestamp"`
	Severity  string    `json:"severity"`
	Body      string    `json:"body"`
	TraceID   string    `json:"traceId"`
}

// ── MCP Response Types ─────────────────────────────────────────────

type MCPDiagnosis struct {
	RootCause      string            `json:"rootCause"`
	Confidence     float64           `json:"confidence"`
	AffectedServices []string        `json:"affectedServices"`
	Evidence       []string          `json:"evidence"`
	Recommendations []string         `json:"recommendations"`
	Severity       string            `json:"severity"`
	GeneratedAt    time.Time         `json:"generatedAt"`
	Model          string            `json:"model"`
}

type MCPIncidentReport struct {
	Title          string            `json:"title"`
	Summary        string            `json:"summary"`
	Timeline       []ReportTimelineEvent `json:"timeline"`
	RootCause      string            `json:"rootCause"`
	Diagnosis      MCPDiagnosis      `json:"diagnosis"`
	Evidence       []string          `json:"evidence"`
	ActionItems    []string          `json:"actionItems"`
	Prevention     []string          `json:"prevention"`
	GeneratedAt    time.Time         `json:"generatedAt"`
}

type ReportTimelineEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Event       string    `json:"event"`
	Description string    `json:"description"`
}

type MCPReleaseAnalysis struct {
	Service        string            `json:"service"`
	Version        string            `json:"version"`
	CommitSHA      string            `json:"commitSha"`
	HasRegression  bool              `json:"hasRegression"`
	DegradationPct float64           `json:"degradationPct"`
	Metrics        []ReleaseMetric   `json:"metrics"`
	Risk           string            `json:"risk"`
	Recommendation string           `json:"recommendation"`
	GeneratedAt    time.Time         `json:"generatedAt"`
}

type ReleaseMetric struct {
	Name    string  `json:"name"`
	Before  float64 `json:"before"`
	After   float64 `json:"after"`
	ChangePct float64 `json:"changePct"`
	Unit    string  `json:"unit"`
}

// ── LLM Backend Interface ──────────────────────────────────────────

type LLMBackend interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	ModelName() string
}

// ── AI Engine ──────────────────────────────────────────────────────

type AIConfig struct {
	Port           string
	LLMBackend     string
	OpenAIKey      string
	AnthropicKey   string
	OpenAIModel    string
	AnthropicModel string
	CorrelationURL string
	PrometheusURL  string
	TempoURL       string
	LokiURL        string
	Timeout        time.Duration
}

type AIEngine struct {
	config  AIConfig
	backend LLMBackend
	client  *http.Client
}

func NewAIEngine(config AIConfig) *AIEngine {
	client := &http.Client{Timeout: config.Timeout}

	var backend LLMBackend
	switch config.LLMBackend {
	case "anthropic":
		backend = NewAnthropicBackend(config.AnthropicKey, config.AnthropicModel, client)
	case "openai":
		fallthrough
	default:
		backend = NewOpenAIBackend(config.OpenAIKey, config.OpenAIModel, client)
	}

	log.Printf("initialized LLM backend: %s (model: %s)", config.LLMBackend, backend.ModelName())

	return &AIEngine{
		config:  config,
		backend: backend,
		client:  client,
	}
}

func (e *AIEngine) buildPrompt(ctx MCPContext) (string, string) {
	systemPrompt := `You are an expert Site Reliability Engineer specialized in distributed systems observability.
Your role is to analyze telemetry data (metrics, traces, logs) from OpenTelemetry-instrumented services.

Rules:
1. Be precise and data-driven. Only state facts supported by the telemetry data provided.
2. When diagnosing, identify the most likely root cause based on error patterns, latency spikes, and log evidence.
3. For incident reports, provide actionable recommendations and preventive measures.
4. For release analysis, compare pre/post deploy metrics and flag regressions.
5. ALWAYS respond in valid JSON format matching the requested schema.
6. If confidence is below 0.5, clearly state that more data is needed.
7. Sanitize any sensitive data in your analysis.`

	userPrompt := fmt.Sprintf(`Analyze the following telemetry context.

Action: %s
Service: %s
TraceID: %s

Metrics:
%s

Spans (trace tree):
%s

Logs:
%s

Provide your analysis in JSON format.`, ctx.Action, ctx.Incident.ServiceName, ctx.Incident.TraceID,
		toJSON(ctx.Telemetry.Metrics), toJSON(ctx.Telemetry.Spans), toJSON(ctx.Telemetry.Logs))

	return systemPrompt, userPrompt
}

func (e *AIEngine) invokeLLM(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	result, err := e.backend.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("llm invocation failed: %w", err)
	}
	return result, nil
}

// ── MCP Handlers ───────────────────────────────────────────────────

func (e *AIEngine) handleDiagnose(w http.ResponseWriter, r *http.Request) {
	var mcpCtx MCPContext
	if err := json.NewDecoder(r.Body).Decode(&mcpCtx); err != nil {
		http.Error(w, `{"error":"invalid MCP context"}`, http.StatusBadRequest)
		return
	}
	mcpCtx.Action = "diagnose"

	sysPrompt, userPrompt := e.buildPrompt(mcpCtx)
	userPrompt += "\n\nExpected output schema: {\"rootCause\": string, \"confidence\": float (0-1), \"affectedServices\": [string], \"evidence\": [string], \"recommendations\": [string], \"severity\": string}"

	response, err := e.invokeLLM(r.Context(), sysPrompt, userPrompt)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	var diagnosis MCPDiagnosis
	if err := json.Unmarshal([]byte(response), &diagnosis); err != nil {
		// Try to extract JSON from response
		diagnosis = extractDiagnosisFromText(response)
	}
	diagnosis.GeneratedAt = time.Now()
	diagnosis.Model = e.backend.ModelName()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(diagnosis)
}

func (e *AIEngine) handleGenerateReport(w http.ResponseWriter, r *http.Request) {
	var mcpCtx MCPContext
	if err := json.NewDecoder(r.Body).Decode(&mcpCtx); err != nil {
		http.Error(w, `{"error":"invalid MCP context"}`, http.StatusBadRequest)
		return
	}
	mcpCtx.Action = "generate_incident_report"

	sysPrompt, userPrompt := e.buildPrompt(mcpCtx)
	userPrompt += "\n\nExpected output: full incident report in JSON with fields: title, summary (2-3 sentences), timeline (array of {timestamp, event, description}), rootCause, evidence (array), actionItems (array), prevention (array)"

	response, err := e.invokeLLM(r.Context(), sysPrompt, userPrompt)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	var report MCPIncidentReport
	if err := json.Unmarshal([]byte(response), &report); err != nil {
		report = MCPIncidentReport{
			Title:     fmt.Sprintf("Incident Report — %s", mcpCtx.Incident.ServiceName),
			Summary:   response,
			RootCause: "See LLM response for details",
		}
	}
	report.GeneratedAt = time.Now()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

func (e *AIEngine) handleReleaseAnalysis(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Service    string           `json:"service"`
		Version    string           `json:"version"`
		CommitSHA  string           `json:"commitSha"`
		PreDeploy  []MetricSnapshot `json:"preDeployMetrics"`
		PostDeploy []MetricSnapshot `json:"postDeployMetrics"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	// Statistical comparison
	analysis := compareMetrics(req.PreDeploy, req.PostDeploy)

	mcpCtx := MCPContext{
		Action: "release_analysis",
		Incident: IncidentContext{
			ServiceName: req.Service,
		},
		Telemetry: TelemetryContext{
			Metrics: req.PostDeploy,
		},
	}

	sysPrompt, userPrompt := e.buildPrompt(mcpCtx)
	userPrompt += fmt.Sprintf("\n\nAdditional context:\nVersion: %s\nCommit: %s\nPre/post metric comparison:\n%s\n\nProvide release analysis in JSON with fields: hasRegression (bool), degradationPct (float), risk (low|medium|high), recommendation (string)", req.Version, req.CommitSHA, toJSON(analysis))

	response, err := e.invokeLLM(r.Context(), sysPrompt, userPrompt)
	if err != nil {
		// Fallback to basic analysis if LLM fails
		analysisJSON, _ := json.Marshal(analysis)
		w.Header().Set("Content-Type", "application/json")
		w.Write(analysisJSON)
		return
	}

	var aiAnalysis MCPReleaseAnalysis
	if err := json.Unmarshal([]byte(response), &aiAnalysis); err != nil {
		aiAnalysis = extractReleaseAnalysisFromText(response)
	}
	aiAnalysis.Service = req.Service
	aiAnalysis.Version = req.Version
	aiAnalysis.CommitSHA = req.CommitSHA
	aiAnalysis.Metrics = analysis
	aiAnalysis.GeneratedAt = time.Now()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(aiAnalysis)
}

func (e *AIEngine) handleAdHocQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query   string `json:"query"`
		Context string `json:"context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	systemPrompt := "You are a helpful observability assistant. Answer questions about distributed systems based on the provided context."
	userPrompt := fmt.Sprintf("Context: %s\n\nQuestion: %s\n\nProvide a concise, accurate answer.", req.Context, req.Query)

	response, err := e.invokeLLM(r.Context(), systemPrompt, userPrompt)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"answer": response,
		"model":  e.backend.ModelName(),
	})
}

// ── OpenAI Backend ─────────────────────────────────────────────────

type OpenAIBackend struct {
	apiKey string
	model  string
	client *http.Client
}

func NewOpenAIBackend(apiKey, model string, client *http.Client) *OpenAIBackend {
	return &OpenAIBackend{apiKey: apiKey, model: model, client: client}
}

func (b *OpenAIBackend) ModelName() string { return b.model }

func (b *OpenAIBackend) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	payload := map[string]interface{}{
		"model": b.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.3,
		"max_tokens":  2000,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.apiKey)

	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("openai API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("openai returned no choices")
	}

	return result.Choices[0].Message.Content, nil
}

// ── Anthropic Backend ──────────────────────────────────────────────

type AnthropicBackend struct {
	apiKey string
	model  string
	client *http.Client
}

func NewAnthropicBackend(apiKey, model string, client *http.Client) *AnthropicBackend {
	return &AnthropicBackend{apiKey: apiKey, model: model, client: client}
}

func (b *AnthropicBackend) ModelName() string { return b.model }

func (b *AnthropicBackend) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	payload := map[string]interface{}{
		"model":      b.model,
		"max_tokens": 2000,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", b.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("anthropic API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("anthropic returned no content")
	}

	return result.Content[0].Text, nil
}

// ── Helpers ────────────────────────────────────────────────────────

func toJSON(v interface{}) string {
	data, _ := json.MarshalIndent(v, "", "  ")
	return string(data)
}

func loadAIConfig() AIConfig {
	timeout, _ := time.ParseDuration(getEnv("LLM_TIMEOUT", "30s"))
	return AIConfig{
		Port:           getEnv("AI_ENGINE_PORT", "8082"),
		LLMBackend:     getEnv("LLM_BACKEND", "openai"),
		OpenAIKey:      getEnv("OPENAI_API_KEY", ""),
		AnthropicKey:   getEnv("ANTHROPIC_API_KEY", ""),
		OpenAIModel:    getEnv("OPENAI_MODEL", "gpt-4o"),
		AnthropicModel: getEnv("ANTHROPIC_MODEL", "claude-sonnet-4-6"),
		CorrelationURL: getEnv("CORRELATION_URL", "http://localhost:8081"),
		PrometheusURL:  getEnv("PROMETHEUS_URL", "http://localhost:9090"),
		TempoURL:       getEnv("TEMPO_URL", "http://localhost:3200"),
		LokiURL:        getEnv("LOKI_URL", "http://localhost:3100"),
		Timeout:        timeout,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func compareMetrics(pre, post []MetricSnapshot) []ReleaseMetric {
	preMap := make(map[string]float64)
	for _, m := range pre {
		preMap[m.Name+m.Service] = m.Value
	}

	var results []ReleaseMetric
	for _, m := range post {
		before, exists := preMap[m.Name+m.Service]
		if !exists {
			continue
		}
		changePct := 0.0
		if before > 0 {
			changePct = ((m.Value - before) / before) * 100
		}
		results = append(results, ReleaseMetric{
			Name:      m.Name,
			Before:    before,
			After:     m.Value,
			ChangePct: changePct,
			Unit:      m.Unit,
		})
	}
	return results
}

func extractDiagnosisFromText(text string) MCPDiagnosis {
	// Try to find JSON in the response text
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		jsonStr := text[start : end+1]
		var diagnosis MCPDiagnosis
		if json.Unmarshal([]byte(jsonStr), &diagnosis) == nil {
			return diagnosis
		}
	}
	return MCPDiagnosis{
		RootCause:  text,
		Confidence: 0.5,
		Severity:   "unknown",
	}
}

func extractReleaseAnalysisFromText(text string) MCPReleaseAnalysis {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		jsonStr := text[start : end+1]
		var analysis MCPReleaseAnalysis
		if json.Unmarshal([]byte(jsonStr), &analysis) == nil {
			return analysis
		}
	}
	return MCPReleaseAnalysis{}
}
