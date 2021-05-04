#!/bin/bash

kubectl -n influxdb create secret generic influxdb-creds \
  --from-literal=INFLUXDB_DATABASE=influx \
  --from-literal=INFLUXDB_USERNAME=influx \
  --from-literal=INFLUXDB_PASSWORD=influx \
  --from-literal=INFLUXDB_HOST=influxdb
