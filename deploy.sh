#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------
# go-sre-observatory — full deploy to minikube
# -------------------------------------------------------

NAMESPACE="observatory"
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

step() { echo -e "\n${CYAN}▶ $1${NC}"; }
ok()   { echo -e "${GREEN}✓ $1${NC}"; }
warn() { echo -e "${YELLOW}⚠ $1${NC}"; }

# --- Preflight ---
step "Checking prerequisites"
for cmd in minikube kubectl docker; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "Error: $cmd not found"; exit 1; }
done
ok "All tools present"

# --- Minikube ---
step "Starting minikube"
if ! minikube status | grep -q "Running"; then
  minikube start --cpus=4 --memory=4096 --driver=docker
fi
ok "minikube running"

eval "$(minikube docker-env --shell bash)"
ok "Docker env pointed at minikube"


# --- Namespace ---
step "Creating namespace"
kubectl apply -f k8s/monitoring/prometheus.yaml | grep -E "^(namespace|serviceaccount|clusterrole)" || true
ok "Namespace $NAMESPACE ready"

# --- Slack webhook ---
if [ -z "$SLACK_WEBHOOK_URL" ]; then
  warn "SLACK_WEBHOOK_URL not set — Slack alerts will not work"
  warn "Run: export SLACK_WEBHOOK_URL=https://hooks.slack.com/services/..."
  warn "Slack alerts: edit k8s/monitoring/alertmanager.yaml and set your webhook URL"
else
  step "Patching Slack webhook secret"
  kubectl create secret generic alertmanager-secret \
    --from-literal=SLACK_WEBHOOK_URL="$SLACK_WEBHOOK_URL" \
    --namespace "$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -
  ok "Slack secret updated"
  kubectl rollout restart deployment/alertmanager -n "$NAMESPACE" 2>/dev/null || true
fi

# --- Deploy in order ---
step "Deploying Prometheus + RBAC"
kubectl apply -f k8s/monitoring/prometheus.yaml

step "Deploying Alertmanager"
kubectl apply -f k8s/monitoring/alertmanager.yaml

step "Deploying Grafana"
kubectl apply -f k8s/monitoring/grafana.yaml

step "Deploying Node Exporter"
kubectl apply -f k8s/monitoring/node-exporter.yaml

step "Deploying Loki + Promtail"
kubectl apply -f k8s/logging/loki-promtail.yaml

step "Deploying application"
kubectl apply -f k8s/app/deployment.yaml

step "Deploying load generator"
kubectl apply -f k8s/loadgen/deployment.yaml

# --- Wait for rollouts ---
step "Waiting for deployments to be ready"
for deploy in prometheus grafana alertmanager loki observatory-app loadgen; do
  echo -n "  Waiting for $deploy ... "
  kubectl rollout status deployment/"$deploy" -n "$NAMESPACE" --timeout=120s
  ok "$deploy ready"
done

# --- Port-forwards ---
step "Setting up port-forwards (background)"

# Kill any existing port-forwards
pkill -f "kubectl port-forward.*$NAMESPACE" 2>/dev/null || true
sleep 1

kubectl port-forward svc/grafana      -n "$NAMESPACE" 3000:3000 &>/dev/null &
kubectl port-forward svc/prometheus   -n "$NAMESPACE" 9090:9090 &>/dev/null &
kubectl port-forward svc/alertmanager -n "$NAMESPACE" 9093:9093 &>/dev/null &
kubectl port-forward svc/observatory-app -n "$NAMESPACE" 8080:80 &>/dev/null &

sleep 2
ok "Port-forwards active"

echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}  go-sre-observatory deployed!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "  Grafana       →  http://localhost:3000  (admin / observatory)"
echo "  Prometheus    →  http://localhost:9090"
echo "  Alertmanager  →  http://localhost:9093"
echo "  App           →  http://localhost:8080"
echo ""
echo "  Dashboards auto-provisioned under: Observatory > App Overview"
echo ""
echo ""
