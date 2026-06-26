package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	config := loadGatewayConfig()
	gateway := NewGateway(config)

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(60 * time.Second))
	router.Use(middleware.RequestID)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   strings.Split(config.CORSAllowedOrigins, ","),
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"Link", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           600,
	}))

	router.Handle("/metrics", promhttp.Handler())
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy", "version": "1.0.0"})
	})

	// Public endpoints
	router.Post("/api/v1/auth/login", gateway.handleLogin)
	router.Post("/api/v1/webhooks/deploy", gateway.handleDeployWebhook)
	router.Post("/api/v1/webhooks/alert", gateway.handleAlertWebhook)

	// Protected endpoints
	router.Group(func(r chi.Router) {
		r.Use(gateway.authMiddleware)

		// Correlation
		r.Get("/api/v1/correlation/{traceId}", gateway.proxyCorrelation("GET"))
		r.Post("/api/v1/correlation/query", gateway.proxyCorrelation("POST"))
		r.Get("/api/v1/correlation/service/{service}", gateway.proxyCorrelation("GET"))

		// AI / MCP
		r.Post("/api/v1/ai/diagnose", gateway.proxyAI("POST", "/mcp/diagnose"))
		r.Post("/api/v1/ai/report", gateway.proxyAI("POST", "/mcp/report"))
		r.Post("/api/v1/ai/release-analysis", gateway.proxyAI("POST", "/mcp/release-analysis"))
		r.Post("/api/v1/ai/query", gateway.proxyAI("POST", "/mcp/query"))

		// Admin
		r.Get("/api/v1/admin/sources", gateway.handleListSources)
		r.Post("/api/v1/admin/sources", gateway.handleCreateSource)
		r.Put("/api/v1/admin/sources/{id}", gateway.handleUpdateSource)
		r.Delete("/api/v1/admin/sources/{id}", gateway.handleDeleteSource)
		r.Get("/api/v1/admin/health", gateway.handleSystemHealth)

		// Grafana proxy
		r.Get("/api/v1/grafana/*", gateway.proxyGrafana("GET"))
	})

	log.Printf("API Gateway starting on :%s", config.Port)

	server := &http.Server{Addr: ":" + config.Port, Handler: router}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()
	<-ctx.Done()
	log.Println("shutting down API gateway...")
	server.Shutdown(context.Background())
}

// ── Config ─────────────────────────────────────────────────────────

type GatewayConfig struct {
	Port               string
	CorrelationURL     string
	AIEngineURL        string
	GrafanaURL         string
	JWTSecret          string
	CORSAllowedOrigins string
}

func loadGatewayConfig() GatewayConfig {
	return GatewayConfig{
		Port:               getEnv("API_GATEWAY_PORT", "8080"),
		CorrelationURL:     getEnv("CORRELATION_URL", "http://localhost:8081"),
		AIEngineURL:        getEnv("AI_ENGINE_URL", "http://localhost:8082"),
		GrafanaURL:         getEnv("GRAFANA_URL", "http://localhost:3000"),
		JWTSecret:          getEnv("JWT_SECRET", "dev-secret-change-me"),
		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "*"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ── Gateway ────────────────────────────────────────────────────────

type Gateway struct {
	config     GatewayConfig
	correlationProxy *httputil.ReverseProxy
	aiProxy          *httputil.ReverseProxy
	grafanaProxy     *httputil.ReverseProxy
	sources          []DataSource
}

type DataSource struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Endpoint string `json:"endpoint"`
	Enabled  bool   `json:"enabled"`
}

func NewGateway(config GatewayConfig) *Gateway {
	correlationURL, _ := url.Parse(config.CorrelationURL)
	aiURL, _ := url.Parse(config.AIEngineURL)
	grafanaURL, _ := url.Parse(config.GrafanaURL)

	return &Gateway{
		config:           config,
		correlationProxy: httputil.NewSingleHostReverseProxy(correlationURL),
		aiProxy:          httputil.NewSingleHostReverseProxy(aiURL),
		grafanaProxy:     httputil.NewSingleHostReverseProxy(grafanaURL),
		sources: []DataSource{
			{ID: "1", Name: "Prometheus", Type: "prometheus", Endpoint: "http://prometheus:9090", Enabled: true},
			{ID: "2", Name: "Tempo", Type: "tempo", Endpoint: "http://tempo:3200", Enabled: true},
			{ID: "3", Name: "Loki", Type: "loki", Endpoint: "http://loki:3100", Enabled: true},
		},
	}
}

// ── Auth ───────────────────────────────────────────────────────────

func (g *Gateway) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
			return
		}

		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(g.config.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) handleLogin(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	// Simplified auth — in production, integrate with OIDC/LDAP
	if creds.Username != "admin" || creds.Password != "spectrum" {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  creds.Username,
		"role": "admin",
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(g.config.JWTSecret))
	if err != nil {
		http.Error(w, `{"error":"failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"token":     tokenString,
		"expiresIn": "86400",
	})
}

// ── Proxy Helpers ──────────────────────────────────────────────────

func (g *Gateway) proxyCorrelation(_ string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g.correlationProxy.ServeHTTP(w, r)
	}
}

func (g *Gateway) proxyAI(method string, path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = path

		// Enrich request with context
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))

		g.aiProxy.ServeHTTP(w, r)
	}
}

func (g *Gateway) proxyGrafana(method string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/api/v1/grafana")
		g.grafanaProxy.ServeHTTP(w, r)
	}
}

// ── Webhooks ───────────────────────────────────────────────────────

func (g *Gateway) handleDeployWebhook(w http.ResponseWriter, r *http.Request) {
	var deploy struct {
		Service     string `json:"service"`
		Version     string `json:"version"`
		CommitSHA   string `json:"commitSha"`
		Environment string `json:"environment"`
		Timestamp   string `json:"timestamp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&deploy); err != nil {
		http.Error(w, `{"error":"invalid deploy payload"}`, http.StatusBadRequest)
		return
	}

	log.Printf("deploy detected: service=%s version=%s commit=%s env=%s",
		deploy.Service, deploy.Version, deploy.CommitSHA, deploy.Environment)

	// Trigger release analysis asynchronously via AI engine
	go func() {
		analysisReq := map[string]interface{}{
			"service":   deploy.Service,
			"version":   deploy.Version,
			"commitSha": deploy.CommitSHA,
		}
		body, _ := json.Marshal(analysisReq)
		http.Post(g.config.AIEngineURL+"/mcp/release-analysis",
			"application/json", bytes.NewReader(body))
	}()

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "accepted",
		"message": "release analysis triggered",
	})
}

func (g *Gateway) handleAlertWebhook(w http.ResponseWriter, r *http.Request) {
	var alert struct {
		Name        string `json:"name"`
		Service     string `json:"service"`
		Severity    string `json:"severity"`
		Description string `json:"description"`
		TraceID     string `json:"traceId"`
		Timestamp   string `json:"timestamp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
		http.Error(w, `{"error":"invalid alert payload"}`, http.StatusBadRequest)
		return
	}

	log.Printf("alert received: name=%s service=%s severity=%s", alert.Name, alert.Service, alert.Severity)

	// Trigger diagnosis via AI engine asynchronously
	go func() {
		mcpCtx := map[string]interface{}{
			"incident": map[string]interface{}{
				"alertName":   alert.Name,
				"serviceName": alert.Service,
				"severity":    alert.Severity,
				"traceId":     alert.TraceID,
				"description": alert.Description,
				"startTime":   alert.Timestamp,
			},
			"action": "diagnose",
		}
		body, _ := json.Marshal(mcpCtx)
		http.Post(g.config.AIEngineURL+"/mcp/diagnose",
			"application/json", bytes.NewReader(body))
	}()

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "accepted",
	})
}

// ── Admin Handlers ─────────────────────────────────────────────────

func (g *Gateway) handleListSources(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(g.sources)
}

func (g *Gateway) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	var source DataSource
	if err := json.NewDecoder(r.Body).Decode(&source); err != nil {
		http.Error(w, `{"error":"invalid source"}`, http.StatusBadRequest)
		return
	}
	source.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	g.sources = append(g.sources, source)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(source)
}

func (g *Gateway) handleUpdateSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var source DataSource
	if err := json.NewDecoder(r.Body).Decode(&source); err != nil {
		http.Error(w, `{"error":"invalid source"}`, http.StatusBadRequest)
		return
	}
	for i, s := range g.sources {
		if s.ID == id {
			source.ID = id
			g.sources[i] = source
			json.NewEncoder(w).Encode(source)
			return
		}
	}
	http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
}

func (g *Gateway) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	for i, s := range g.sources {
		if s.ID == id {
			g.sources = append(g.sources[:i], g.sources[i+1:]...)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
}

func (g *Gateway) handleSystemHealth(w http.ResponseWriter, r *http.Request) {
	services := map[string]string{}

	checkService := func(name, url string) string {
		resp, err := http.Get(url)
		if err != nil {
			return "unreachable"
		}
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			return "healthy"
		}
		return "degraded"
	}

	services["correlation-engine"] = checkService("correlation", g.config.CorrelationURL+"/health")
	services["ai-engine"] = checkService("ai", g.config.AIEngineURL+"/health")

	allHealthy := true
	for _, status := range services {
		if status != "healthy" {
			allHealthy = false
			break
		}
	}

	status := "healthy"
	if !allHealthy {
		status = "degraded"
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   status,
		"services": services,
	})
}

