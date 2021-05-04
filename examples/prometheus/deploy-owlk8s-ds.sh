#!/bin/bash

port=65534
is_takenN=$(kubectl get svc -A -o jsonpath='{range .items[*]}{range .spec.ports[*]}{.nodePort}{"\n"}{end}{end}' | grep $port | wc -l)
is_takenH=$(kubectl get pods -A -o jsonpath='{range .items[*]}{range .spec.containers[*]}{range .ports[*]}{.hostPort}{end}{end}{end}' | grep $port | wc -l )
if [ "$is_takenN" -gt "0" ] || [ "$is_takenH" -gt "0" ]
then
   echo -n "Port 65534 is taken in your cluster. Please chose another port: "
   while true
   do
     read port
     if ! [[ $port =~ [0-9]{1,5} ]]
     then
      echo "Input is not numeric or larger than allowed port max: 65535 !"
     fi
     break
   done
fi
sed -re "s/##PORT##/$port/" owlk8s-ds.yaml | kubectl apply -f -
