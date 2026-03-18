package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// --- Metrics ---

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests partitioned by method, path, and status code.",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5},
	}, []string{"method", "path"})

	httpRequestsInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Current number of requests being served.",
	})

	appErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "app_errors_total",
		Help: "Total application errors partitioned by path and error type.",
	}, []string{"path", "error_type"})

	appVersion = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "app_info",
		Help: "Application build info.",
	}, []string{"version", "goversion"})
)

// --- Instrumented ResponseWriter ---

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newStatusResponseWriter(w http.ResponseWriter) *statusResponseWriter {
	return &statusResponseWriter{w, http.StatusOK}
}

func (sw *statusResponseWriter) WriteHeader(code int) {
	sw.statusCode = code
	sw.ResponseWriter.WriteHeader(code)
}

// --- Middleware ---

func instrument(path string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sw := newStatusResponseWriter(w)
		start := time.Now()
		httpRequestsInFlight.Inc()

		defer func() {
			duration := time.Since(start).Seconds()
			status := strconv.Itoa(sw.statusCode)
			httpRequestsInFlight.Dec()
			httpRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
			httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
		}()

		next(sw, r)
	}
}

func jsonResponse(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func logJSON(level, path, msg string, fields map[string]any) {
	entry := map[string]any{
		"time":  time.Now().Format(time.RFC3339),
		"level": level,
		"path":  path,
		"msg":   msg,
	}
	for k, v := range fields {
		entry[k] = v
	}
	b, _ := json.Marshal(entry)
	log.Println(string(b))
}

// --- Handlers ---

func sharanHandler(w http.ResponseWriter, r *http.Request){
	logJSON("info", "/sharan", "request served", map[string]any{})
    jsonResponse(w, http.StatusOK, map[string]any{
        "name": "sharan",
        "role": "SRE",
    })
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func ordersHandler(w http.ResponseWriter, r *http.Request) {
	latency := time.Duration(20+rand.Intn(180)) * time.Millisecond
	time.Sleep(latency)

	// ~5% upstream timeout errors
	if rand.Float64() < 0.05 {
		appErrorsTotal.WithLabelValues("/orders", "upstream_timeout").Inc()
		logJSON("error", "/orders", "upstream timeout", map[string]any{"latency_ms": latency.Milliseconds()})
		jsonResponse(w, http.StatusServiceUnavailable, map[string]string{"error": "upstream timeout"})
		return
	}

	logJSON("info", "/orders", "request served", map[string]any{"latency_ms": latency.Milliseconds()})
	jsonResponse(w, http.StatusOK, map[string]any{
		"orders": []map[string]string{
			{"id": "ord-001", "status": "shipped"},
			{"id": "ord-002", "status": "pending"},
			{"id": "ord-003", "status": "processing"},
		},
	})
}

func paymentsHandler(w http.ResponseWriter, r *http.Request) {
	latency := time.Duration(50+rand.Intn(450)) * time.Millisecond
	time.Sleep(latency)

	// ~2% payment gateway errors
	if rand.Float64() < 0.02 {
		appErrorsTotal.WithLabelValues("/payments", "gateway_error").Inc()
		logJSON("error", "/payments", "payment gateway error", map[string]any{"latency_ms": latency.Milliseconds()})
		jsonResponse(w, http.StatusBadGateway, map[string]string{"error": "payment gateway error"})
		return
	}

	logJSON("info", "/payments", "payment processed", map[string]any{"latency_ms": latency.Milliseconds()})
	jsonResponse(w, http.StatusOK, map[string]any{
		"status":     "processed",
		"latency_ms": latency.Milliseconds(),
	})
}

func inventoryHandler(w http.ResponseWriter, r *http.Request) {
	latency := time.Duration(5+rand.Intn(40)) * time.Millisecond
	time.Sleep(latency)

	logJSON("info", "/inventory", "request served", map[string]any{"latency_ms": latency.Milliseconds()})
	jsonResponse(w, http.StatusOK, map[string]any{
		"items": []map[string]any{
			{"sku": "SKU-A", "stock": 142},
			{"sku": "SKU-B", "stock": 0},
			{"sku": "SKU-C", "stock": 37},
		},
	})
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
    latency := time.Duration(10+rand.Intn(60)) * time.Millisecond
    time.Sleep(latency)

    if rand.Float64() < 0.01 {
        appErrorsTotal.WithLabelValues("/users", "user_not_found").Inc()
        logJSON("error", "/users", "user not found", map[string]any{"latency_ms": latency.Milliseconds()})
        jsonResponse(w, http.StatusNotFound, map[string]string{"error": "user not found"})
        return
    }

    logJSON("info", "/users", "request served", map[string]any{"latency_ms": latency.Milliseconds()})
    jsonResponse(w, http.StatusOK, map[string]any{
        "users": []map[string]string{
            {"id": "usr-001", "name": "sharan", "role": "admin"},
            {"id": "usr-002", "name": "alice",  "role": "viewer"},
        },
    })
}

func slowHandler(w http.ResponseWriter, r *http.Request) {
	// Intentionally slow endpoint to demonstrate latency SLO breaches
	latency := time.Duration(800+rand.Intn(1200)) * time.Millisecond
	time.Sleep(latency)

	logJSON("warn", "/slow", "slow response", map[string]any{"latency_ms": latency.Milliseconds()})
	jsonResponse(w, http.StatusOK, map[string]any{
		"message":    "this endpoint is intentionally slow — watch p99 latency in Grafana",
		"latency_ms": latency.Milliseconds(),
	})
}

func main() {
	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = "1.0.0"
	}
	appVersion.WithLabelValues(version, "go1.22").Set(1)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz",    instrument("/healthz",    healthHandler))
	mux.HandleFunc("/orders",     instrument("/orders",     ordersHandler))
	mux.HandleFunc("/payments",   instrument("/payments",   paymentsHandler))
	mux.HandleFunc("/inventory",  instrument("/inventory",  inventoryHandler))
	mux.HandleFunc("/slow",       instrument("/slow",       slowHandler))
	mux.Handle("/metrics",        promhttp.Handler())
	mux.HandleFunc("/users", instrument("/users", usersHandler))
	mux.HandleFunc("/sharan", instrument("sharan", sharanHandler))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logJSON("info", "/", "server starting", map[string]any{"port": port, "version": version})
	fmt.Printf("Listening on :%s\n", port)

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
