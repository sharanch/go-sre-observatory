# Runbook: HighErrorRate

## Alert
HTTP error rate exceeded 5% over a 5 minute window.

## Impact
Users are receiving 5xx errors. Orders, payments, or other endpoints may be failing.

## Investigation

### 1. Check which endpoint is failing
```
sum(rate(http_requests_total{status=~"5.."}[5m])) by (path)
```

### 2. Check recent logs
```
{app="observatory-app", level="error"}
```

### 3. Check if the pod is healthy
```bash
kubectl get pods -n observatory
kubectl logs deployment/observatory-app -n observatory --tail=50
```

## Common causes
- Upstream dependency timeout — check `/orders` upstream connections
- Pod OOMKilled — check memory limits
- Bad deployment — check recent image changes

## Resolution
- If bad deployment: `kubectl rollout undo deployment/observatory-app -n observatory`
- If OOM: increase memory limits in `k8s/app/deployment.yaml`
- If upstream: check dependent services

## Escalation
If unresolved after 15 minutes escalate to on-call lead.