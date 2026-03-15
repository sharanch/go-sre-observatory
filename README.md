# go-sre-observatory

A production-grade observability platform built on Kubernetes, demonstrating SRE practices through a fully instrumented Go microservice. Covers the three pillars of observability — metrics, logs, and alerting — with automated traffic generation so dashboards are always live.

---

## What this demonstrates

| Practice | Implementation |
|---|---|
| Metrics instrumentation | Go app exposes RED metrics (Rate, Errors, Duration) via `prometheus/client_golang` |
| Log aggregation | Structured JSON logs shipped by Promtail → Loki, queryable in Grafana |
| Alerting | Prometheus alert rules → Alertmanager → Slack with severity routing |
| Kubernetes-native discovery | Prometheus auto-discovers pods via `prometheus.io/scrape` annotations |
| Realistic traffic | Load generator simulates 10 RPS baseline with 40 RPS spikes every 2 minutes |
| Runbook-driven alerts | Every alert links to a runbook explaining the response |

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  Kubernetes (minikube)                │
│  namespace: observatory                               │
│                                                       │
│  ┌─────────────┐   metrics    ┌──────────────────┐   │
│  │  Go App     │ ──────────── │   Prometheus     │   │
│  │  :8080      │              │   15s scrape     │   │
│  │  /orders    │   logs       └────────┬─────────┘   │
│  │  /payments  │ ──────────── Promtail │    │         │
│  │  /inventory │              → Loki  │    │ rules    │
│  │  /slow      │                       │    ▼         │
│  └─────────────┘              ┌────────┴──────────┐  │
│                                │   Alertmanager    │  │
│  ┌─────────────┐               │   Slack routing   │  │
│  │  Loadgen    │               └───────────────────┘  │
│  │  10 RPS +   │                         │            │
│  │  spikes     │              ┌───────────────────┐   │
│  └─────────────┘              │   Grafana :3000   │   │
│                                │   Dashboards      │   │
│  ┌─────────────┐               │   Log explorer    │   │
│  │Node Exporter│               └───────────────────┘  │
│  │ host metrics│                                       │
│  └─────────────┘                                       │
└─────────────────────────────────────────────────────┘
```

---

## Stack

| Component | Version | Role |
|---|---|---|
| Go | 1.22 | Application runtime |
| Prometheus | 2.51 | Metrics collection & alerting |
| Grafana | 10.4 | Visualization & dashboards |
| Alertmanager | 0.27 | Alert routing to Slack |
| Loki | 3.0 | Log aggregation |
| Promtail | 3.0 | Log shipping (DaemonSet) |
| Node Exporter | 1.8 | Host-level metrics |
| Kubernetes | 1.28+ | Orchestration (minikube) |

---

## Getting started

### Prerequisites

- [minikube](https://minikube.sigs.k8s.io/docs/start/) ≥ 1.32
- [kubectl](https://kubernetes.io/docs/tasks/tools/) ≥ 1.28
- [Docker](https://docs.docker.com/get-docker/)
- 4 CPU cores and 4 GB RAM available

### Deploy (one command)

```bash
git clone https://github.com/yourusername/go-sre-observatory
cd go-sre-observatory
./deploy.sh
```

The script will:
1. Start minikube (if not running)
2. Build both Docker images inside minikube's daemon
3. Apply all Kubernetes manifests in dependency order
4. Wait for every deployment to become ready
5. Open port-forwards for local access

### Access the UIs

| Service | URL | Credentials |
|---|---|---|
| Grafana | http://localhost:3000 | admin / observatory |
| Prometheus | http://localhost:9090 | — |
| Alertmanager | http://localhost:9093 | — |
| App | http://localhost:8080 | — |

Dashboard auto-provisions under **Observatory → App Overview** in Grafana.

### Configure Slack alerts

Edit `k8s/monitoring/alertmanager.yaml` and replace the placeholder webhook:

```yaml
stringData:
  SLACK_WEBHOOK_URL: "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK"
```

Then apply:
```bash
kubectl apply -f k8s/monitoring/alertmanager.yaml
```

---

## Application endpoints

| Endpoint | Behaviour | Simulated error rate |
|---|---|---|
| `GET /healthz` | Health check, no latency | 0% |
| `GET /orders` | 20–200ms latency | ~5% (503 upstream timeout) |
| `GET /payments` | 50–500ms latency | ~2% (502 gateway error) |
| `GET /inventory` | 5–40ms latency | 0% |
| `GET /slow` | 800–2000ms latency | 0% |
| `GET /metrics` | Prometheus exposition | — |

The `/slow` endpoint is intentional — it exists to demonstrate P99 latency alerts firing in Grafana.

---

## Metrics exposed

The Go app exposes the following custom metrics:

```
# Requests counter — method, path, status code
http_requests_total{method="GET", path="/orders", status="200"} 1423

# Latency histogram — method, path
http_request_duration_seconds_bucket{method="GET", path="/payments", le="0.25"} 891

# In-flight gauge
http_requests_in_flight 3

# Error counter — path, error type
app_errors_total{path="/orders", error_type="upstream_timeout"} 12

# Build info gauge
app_info{version="1.0.0", goversion="go1.22"} 1
```

---

## Alert runbooks

### `HighErrorRate`
**Fires when:** 5xx error rate exceeds 5% over a 5-minute window.

**Immediate response:**
```bash
# Check which endpoints are erroring
kubectl logs -l app=observatory-app -n observatory --tail=50 | grep '"level":"error"'

# Check Prometheus for per-path error breakdown
# Query: sum(rate(http_requests_total{status=~"5.."}[5m])) by (path)
```

**Likely causes:** Upstream dependency failure (`/orders` → upstream timeout), deployment rollout issue, pod OOMKill. Check pod events:
```bash
kubectl describe pods -l app=observatory-app -n observatory
```

---

### `HighP99Latency`
**Fires when:** P99 latency on any endpoint exceeds 1 second for 5 minutes.

**Immediate response:**
```bash
# Confirm which path is slow
# Prometheus query: histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, path))

# Check in-flight requests (saturation indicator)
# Query: http_requests_in_flight
```

**Note:** `/slow` is intentionally slow and will often contribute to this alert. Use the `path` label to filter it from SLO calculations in production.

---

### `AppDown`
**Fires when:** A pod has been unreachable for 1 minute.

**Immediate response:**
```bash
kubectl get pods -n observatory
kubectl describe pod <pod-name> -n observatory
kubectl logs <pod-name> -n observatory --previous
```

---

## Project structure

```
go-sre-observatory/
├── app/
│   ├── main.go          # Go HTTP server — instrumented with Prometheus
│   ├── go.mod
│   └── Dockerfile       # Multi-stage build → scratch image
├── k8s/
│   ├── app/
│   │   └── deployment.yaml
│   ├── monitoring/
│   │   ├── prometheus.yaml   # Deployment + RBAC + alert rules ConfigMap
│   │   ├── grafana.yaml      # Deployment + provisioned datasources + dashboard
│   │   ├── alertmanager.yaml # Deployment + Slack routing config
│   │   └── node-exporter.yaml
│   ├── logging/
│   │   └── loki-promtail.yaml
│   └── loadgen/
│       ├── main.go      # Traffic generator — 10 RPS + periodic spikes
│       ├── Dockerfile
│       └── deployment.yaml
├── deploy.sh            # One-command deploy
├── teardown.sh
└── README.md
```

---

## Design decisions

**Why Go for the app?** Go's standard library is lean, the Prometheus client is first-class, and binary size matters when using `scratch` base images. The app compiles to a ~7MB container.

**Why not use the Prometheus Operator?** The Operator is the right choice for production multi-team setups, but using raw manifests here makes the RBAC and scrape configuration explicit and visible — better for learning and showcasing understanding.

**Why Loki over ELK?** Loki follows the same label model as Prometheus, which means the same label set that identifies a pod in Prometheus also finds its logs in Loki. No separate index infrastructure, and the resource footprint is much lighter for a portfolio setup.

**Why a custom load generator over k6/locust?** A Go-native generator runs in the same cluster with minimal overhead and demonstrates Go proficiency beyond the main app. For production load testing, k6 would be the right tool.

---

## Tear down

```bash
./teardown.sh
```

---

## Author

Built as part of a DevOps/SRE portfolio transition from cloud operations (Oracle Cloud, Ctrls). This project bridges hands-on infrastructure experience with platform engineering practices.
