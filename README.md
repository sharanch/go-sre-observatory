# go-sre-observatory

A production-grade observability platform running on Kubernetes - built to demonstrate SRE engineering practices through a fully instrumented Go microservice. Covers the three pillars of observability: metrics, logs, and alerting. A built-in load generator keeps dashboards live at all times.


---

```
Code change → git push
    ↓
GitHub Actions: test → build → push to GHCR
    ↓
./deploy.sh locally:
    - injects SLACK_WEBHOOK_URL from env
    - applies all manifests
    - pulls latest image (Always policy)
    - sets up ingress for all services
    ↓
Full stack running:
    - Go app serving traffic
    - Prometheus scraping metrics
    - Grafana showing dashboard + logs
    - Loki receiving logs via Promtail
    - Alertmanager routing to Slack
```

## Screenshots

### Grafana — live dashboard with traffic spikes
![Grafana dashboard showing request rate, error rate, p99 latency and traffic spikes](docs/screenshots/grafana-dashboard.png)
16 req/s baseline with periodic 40 RPS spikes every 2 minutes. P99 latency shown in red — driven by the intentionally slow `/slow` endpoint breaching the 1s SLO threshold.

### Grafana — application logs via Loki
![Grafana dashboard showing application logs panel](docs/screenshots/grafana-logs.png)
Structured JSON logs shipped via Promtail → Loki, queryable alongside metrics. App label extracted from pod filesystem path via regex pipeline stage.

### Prometheus — auto-discovered scrape targets
![Prometheus targets page showing all pods UP](docs/screenshots/prometheus-targets.png)
Prometheus auto-discovers pods via `prometheus.io/scrape` annotations — no static config needed. Both app replicas, node exporter, and Prometheus itself all healthy.

### Alertmanager — alerts firing and routed to Slack
![Alertmanager showing HighP50Latency, HighP99Latency and TestAlert firing](docs/screenshots/alertmanager-firing.png)
Three active alert groups: `HighP50Latency` on `/payments` and `/slow`, `HighP99Latency` on `/slow`, all routing via `slack-warning` receiver.

### Slack — real alert messages in #all-alerts
![Slack channel showing firing and resolved alerts with descriptions](docs/screenshots/slack-alerts.png)
End-to-end alert delivery: Prometheus evaluates rules → Alertmanager routes → Slack receives structured messages with summary, description, and timestamp. `[RESOLVED]` messages fire automatically when conditions clear.

---

## What this demonstrates

| Practice | Implementation |
|---|---|
| Metrics instrumentation | Go app exposes RED metrics (Rate, Errors, Duration) via `prometheus/client_golang` |
| Kubernetes-native discovery | Prometheus scrapes pods via `prometheus.io/scrape` annotations — zero static config |
| Log aggregation | Structured JSON logs shipped via Promtail → Loki, queryable alongside metrics in Grafana |
| Alerting pipeline | Prometheus rules → Alertmanager → Slack with severity routing and runbook links |
| Realistic traffic simulation | Load generator produces 10 RPS baseline + 40 RPS spikes every 2 minutes |
| Runbook-driven operations | Every alert rule includes a runbook with diagnosis steps |
| Infrastructure as code | Entire stack deployable with a single command, torn down with another |
| CI pipeline | GitHub Actions builds and pushes images to GHCR on every push |
| Ingress routing | All services accessible via a single host via nginx ingress |

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                   Kubernetes — namespace: observatory      │
│                                                            │
│  ┌──────────────┐  GET /metrics   ┌───────────────────┐  │
│  │  Go app      │ ──────────────► │   Prometheus      │  │
│  │  :8080       │   every 15s     │   TSDB storage    │  │
│  │              │                 └─────────┬─────────┘  │
│  │  /orders     │  stdout logs              │             │
│  │  /payments   │ ──────────────► Promtail  │ alert rules │
│  │  /inventory  │                 → Loki    │             │
│  │  /users      │                           ▼             │
│  │  /slow       │                 ┌───────────────────┐  │
│  └──────────────┘                 │   Alertmanager    │  │
│                                   │   Slack routing   │  │
│  ┌──────────────┐                 └─────────┬─────────┘  │
│  │  Load gen    │                           │             │
│  │  10 RPS      │                           ▼             │
│  │  + spikes    │                 ┌───────────────────┐  │
│  └──────────────┘                 │   Grafana :3000   │  │
│                                   │   + Loki datasource│  │
│  ┌──────────────┐                 └───────────────────┘  │
│  │ Node exporter│                                         │
│  │ host metrics │           Slack #all-alerts             │
│  └──────────────┘                ▲                        │
│                                  │ webhook                │
│  ┌──────────────┐                │                        │
│  │ nginx ingress│ ───────────────┘                        │
│  │ observatory  │                                         │
│  │ .local       │                                         │
│  └──────────────┘                                         │
└──────────────────────────────────────────────────────────┘
```

---

## Stack

| Component | Version | Role |
|---|---|---|
| Go | 1.22 | Instrumented HTTP server |
| Prometheus | 2.51 | Metrics scraping, storage, alert evaluation |
| Grafana | 10.4 | Dashboards — auto-provisioned on deploy |
| Alertmanager | 0.27 | Alert deduplication, grouping, Slack routing |
| Loki | 3.0 | Log aggregation and storage |
| Promtail | 3.0 | Log shipping DaemonSet |
| Node Exporter | 1.8 | Host CPU, memory, disk metrics |
| nginx ingress | latest | Single host routing for all services |
| Kubernetes | 1.28+ | Orchestration via minikube |

---

## Getting started

### Prerequisites

- [minikube](https://minikube.sigs.k8s.io/docs/start/) ≥ 1.32
- [kubectl](https://kubernetes.io/docs/tasks/tools/) ≥ 1.28
- [Docker](https://docs.docker.com/get-docker/)
- 4 CPU cores and 4 GB RAM

### Deploy

```bash
git clone https://github.com/sharanch/go-sre-observatory
cd go-sre-observatory

# set your Slack webhook so alerts route automatically
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK"

./deploy.sh
```

The script handles everything: starts minikube, enables ingress addon, applies all manifests in dependency order, injects the Slack webhook secret, waits for rollouts, and sets up port-forwards for Prometheus and Alertmanager.

Images are built and pushed to GHCR automatically via GitHub Actions on every push — `deploy.sh` pulls them directly, no local build needed.

### Add the host entry

```bash
echo "$(minikube ip) observatory.local" | sudo tee -a /etc/hosts
```

### Access the UIs

| Service | URL | Credentials |
|---|---|---|
| Grafana | http://observatory.local/grafana | admin / observatory |
| App | http://observatory.local | — |
| Prometheus | http://localhost:9090 | — |
| Alertmanager | http://localhost:9093 | — |

The Grafana dashboard auto-provisions under **Dashboards → Observatory → App Overview**.

### Tear down

```bash
./teardown.sh
```

---

## Application endpoints

| Endpoint | Latency | Error rate | Purpose |
|---|---|---|---|
| `GET /healthz` | ~0ms | 0% | Health check and liveness probe |
| `GET /orders` | 20–200ms | ~5% (503) | Simulates upstream timeout failures |
| `GET /payments` | 50–500ms | ~2% (502) | Simulates payment gateway errors |
| `GET /inventory` | 5–40ms | 0% | Fast read endpoint |
| `GET /users` | 10–70ms | ~1% (404) | User lookup with rare failures |
| `GET /slow` | 800–2000ms | 0% | Intentional SLO breach for latency alerts |
| `GET /metrics` | — | — | Prometheus metrics exposition |

The `/slow` endpoint exists specifically to trigger `HighP99Latency` and `HighP50Latency` alerts — making the alerting pipeline demonstrable without having to inject real failures.

---

## Metrics exposed

```
# Total requests — labelled by method, path, status code
http_requests_total{method="GET", path="/orders", status="200"} 1423

# Latency histogram — used to compute p50/p95/p99 in Grafana
http_request_duration_seconds_bucket{method="GET", path="/payments", le="0.25"} 891

# Current in-flight requests (gauge — goes up and down)
http_requests_in_flight 3

# Application errors — labelled by path and error type
app_errors_total{path="/orders", error_type="upstream_timeout"} 12

# Build info — lets you correlate incidents with deployments
app_info{version="1.0.0", goversion="go1.22"} 1
```

Every metric is recorded automatically by the `instrument()` middleware — handlers contain only business logic.

---

## Alert runbooks

### `HighErrorRate`
**Fires when:** 5xx error rate exceeds 5% over a 5-minute window.

```bash
# Which endpoints are throwing errors?
kubectl logs -l app=observatory-app -n observatory --tail=50 \
  | grep '"level":"error"'

# Per-path breakdown in Prometheus:
# sum(rate(http_requests_total{status=~"5.."}[5m])) by (path)
```

Likely causes: upstream dependency down, bad deployment rollout, pod OOMKill.
```bash
kubectl describe pods -l app=observatory-app -n observatory
```

Full runbook: [docs/runbooks/high-error-rate.md](docs/runbooks/high-error-rate.md)

---

### `HighP99Latency`
**Fires when:** P99 latency on any endpoint exceeds 1s for 5 minutes.

```bash
# Which path is slow?
# histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, path))

# Is the system saturated?
# http_requests_in_flight
```

Note: `/slow` will always breach this threshold by design. In production, exclude it from SLO calculations using a `path!="/slow"` filter.

---

### `HighP50Latency`
**Fires when:** Median latency on any endpoint exceeds 200ms for 5 minutes.

```bash
# histogram_quantile(0.50, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, path))
```

A rising p50 often indicates a systemic issue rather than tail latency — check for resource contention on the node.

---

### `AppDown`
**Fires when:** A pod has been unreachable for 1 minute.

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
│   ├── main.go            # Go HTTP server with Prometheus instrumentation
│   ├── go.mod
│   └── Dockerfile         # Multi-stage build — golang:alpine → alpine
├── k8s/
│   ├── app/
│   │   └── deployment.yaml
│   ├── monitoring/
│   │   ├── prometheus.yaml    # Deployment + RBAC + alert rules
│   │   ├── grafana.yaml       # Deployment + auto-provisioned dashboard
│   │   ├── alertmanager.yaml  # Deployment + Slack routing config
│   │   └── node-exporter.yaml
│   ├── logging/
│   │   └── loki-promtail.yaml
│   ├── loadgen/
│   │   ├── main.go        # Weighted traffic generator with spike logic
│   │   ├── Dockerfile
│   │   └── deployment.yaml
│   └── ingress.yaml       # nginx ingress routing all services via observatory.local
├── docs/
│   ├── runbooks/
│   │   └── high-error-rate.md
│   └── screenshots/
├── .github/workflows/
│   ├── build-app.yaml     # test → build → push app image to GHCR
│   └── build-loadgen.yaml # build → push loadgen image to GHCR
├── deploy.sh              # One-command deploy to minikube
├── teardown.sh
└── README.md
```

---

## Design decisions

**Why Go for the app?** Go's Prometheus client is first-class, compile times are fast, and the resulting binary is small. The app compiles to under 10MB in the final image.

**Why raw manifests instead of the Prometheus Operator?** The Operator is the right tool for multi-team production setups. Using raw manifests here makes the RBAC, scrape configuration, and alert rules fully explicit — every line is visible and explainable.

**Why Loki over ELK?** Loki uses the same label model as Prometheus. The same labels that identify a pod in a Prometheus query (`namespace`, `pod`, `app`) find its logs in Loki — no separate index infrastructure or schema to maintain. Resource footprint is also significantly lighter.

**Why static file discovery for Promtail instead of Kubernetes SD?** Promtail 3.0 has a known issue on minikube where Kubernetes SD auto-injects a stale node name filter, silently discovering zero pods. Static file discovery via `/var/log/pods/observatory_*/*/*.log` is more reliable for single-node local setups. The `app` label is extracted from the file path using a regex pipeline stage.

**Why a custom load generator over k6 or Locust?** Running a Go generator inside the cluster avoids external network overhead and keeps the entire demo self-contained. It also adds a second Go service to the repo, demonstrating the language beyond a single file. For production load testing, k6 would be the right tool.

**Why two app replicas?** The Prometheus targets screenshot shows both replicas scraping independently — demonstrating Kubernetes-native service discovery working as designed, not just a single-pod proof of concept.

**Why ingress over port-forwards?** Port-forwards are fragile — they die when the terminal closes or the connection drops. nginx ingress runs inside the cluster as a stable reverse proxy, routing all services via a single host (`observatory.local`) without any local process to keep alive.
