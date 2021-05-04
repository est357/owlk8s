#!/bin/bash

if [ "$#" -eq "0" ]
then

	echo "Help: ` basename $0` <namespace>"
	exit
fi


kubectl -n $1 create configmap grafana-config \
  --from-file=influxdb-datasource.yml=grafana-influxdb-datasource.yml \
  --from-file=grafana-dashboard-provider.yml=grafana-dashboard-provider.yml \
	--from-file=owlk8s-dash-example.json
