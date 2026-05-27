## Build locally first

```bash
docker build -t go-sre-observatory:latest app/.
docker build -t loadgen:latest k8s/loadgen/.
```

## Load into Minikube

```bash
minikube image load go-sre-observatory:latest
minikube image load loadgen:latest
```

## Point deployments to local images

```bash
kubectl set image deployment/observatory-app app=go-sre-observatory:latest -n observatory
kubectl patch deployment observatory-app -n observatory \
  --type=json -p='[{"op":"replace","path":"/spec/template/spec/containers/0/imagePullPolicy","value":"Never"}]'

kubectl set image deployment/loadgen loadgen=loadgen:latest -n observatory
kubectl patch deployment loadgen -n observatory \
  --type=json -p='[{"op":"replace","path":"/spec/template/spec/containers/0/imagePullPolicy","value":"Never"}]'
```