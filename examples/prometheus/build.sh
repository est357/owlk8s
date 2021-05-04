#!/bin/bash
[ "$(env | grep minikube | wc -l)" -eq "0" ] && eval $(minikube -p `kubectl config current-context` docker-env)
go get github.com/est357/owlk8s
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o owlk8s
docker build -t owlk8s:0.0.1 .
pod=$(kubectl get pods | awk '{print $1}' | grep owlk8s)
if [[ "$pod" =~ owlk8s.+ ]]
then
  kubectl delete pods $pod
fi
# kubectl create configmap prometheus-config --from-file=prometheus.yml=prometheus-config.yaml
