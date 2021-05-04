# Prometheus example

This is by far the most complicated example. If you just want to see how it works please check the simpleLog example.

This example uses a concept called prometheus Collectors to get the metrics. This is because we cannot simply increment a classic prometheus counter because depending on the rate the scraper reads the counter it may have incremented several times in the eBPF program as it increments with live traffic for the given Endpoint.

It also uses a hostPort meaning that the DaemonSet will open an actual port on your node external net interface. It has to do this because it runs in hostNetwork.
One should be aware of this and filter this port from their firewall for external access. That's why in order to deploy you have to do it with the `deploy-owlk8s-ds.sh` script which uses port 65534 and tries to see if it is allocated in your cluster.

Tested with latest minikube.

## Deploy Prometheus

Deploy ConfigMap
```
kubectl create ns prometheus
kubectl -n prometheus create configmap prometheus-config --from-file=prometheus.yml=deployPrometheus/prometheus-config.yaml
```

Deploy Prometheus
```
kubectl -n prometheus apply -f deployPrometheus/prometheus-deploy.yaml
```

## Deploy Owlk8s

```
./build.sh
kubectl apply -f clusterrolebinding.yaml
deploy-owlk8s-ds.sh
```

All this is assuming minikube is used. If a real cluster is used images have to be pushed to container registry and DaemonSet manifest should be updated to use that, build.sh script needs changing...
