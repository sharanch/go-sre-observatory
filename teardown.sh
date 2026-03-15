#!/usr/bin/env bash
set -euo pipefail

echo "Tearing down go-sre-observatory..."

pkill -f "kubectl port-forward.*observatory" 2>/dev/null || true
kubectl delete namespace observatory --ignore-not-found
echo "Done. Run ./deploy.sh to redeploy."
