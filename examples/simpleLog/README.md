# Simple log example

This shows how to use owlk8s with just a simple log csv. This had been tested with latest minikube.

## Build and deploy example powered by owlk8s
**Make sure your kubectl config context points to the right cluster**
```
./build.sh
kubectl apply -f clusterrolebinding.yaml
kubectl apply -f owlk8s-ds.yaml
```

All this is assuming minikube is used. If a real cluster is used images have to be pushed to container registry and DaemonSet manifest should be updated to use that, build.sh script needs changing...
