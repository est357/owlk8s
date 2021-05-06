# Owlk8s

[![Go Reference](https://pkg.go.dev/badge/github.com/est357/owlk8s.svg)](https://pkg.go.dev/github.com/est357/owlk8s)

Seamless RED monitoring of k8s ClusterIP HTTP services.


This library provides RED (rate,error,duration) monitoring for all(by default but exclusions can be done with annotations) ClusterIp services in your cluster with pod level granularity. It is intended for those who don't want to (maybe to avoid complexity) or can't (maybe for security constraints) install a service mesh or cilium CNI. **It does not insert any sidecars** into k8s objects to accomplish this. It uses eBPF. It was tested and works with basic docker bridge (as in minikube), calico and flannel. Maybe with others that use the same CNI concepts.
Tested with latest minikube and the following cloud providers: AWS,Gcloud(only Ubuntu images work see Caveats section for more details),IBMCloud (it should work with Azure too but not tested). Tested with kernels 4.14, 4.19, 5.4 so it should work with recent kernels.

## How it works  

It is run as a DaemonSet with hostNetwork:true. **No securityContext capabilities are needed** as the eBPF program type is SOCKET_FILTER which doesn't require elevated privileges. It works by creating a k8s controller which watches Endpoints and loads a eBPF filter program on every network interface corresponding to an Endpoint(so for every pod's net interface). It collects metrics in real time. The user program can read the metrics at a desired time interval. The interval at which the user reads will decide the resolution of the metrics.
The users receive just basic data and can format and use it in any way they see fit.

The data model the user receives is a map with the key being "PodIPAddress" and value a pointer to a struct:
```
type ResMap map[string]*ResStruct
```
The struct provides metadata from the cluster and a function returning the actual metrics:
```
type ResStruct struct {
	OwnerName, PodName, NS, SvcName string
	Results                         func(map[string]*uint64)
}

```
From the Results() function you receive the actual metrics from the eBPF program loaded in the kernel in the form of a Go map with following self explanatory keys: "requests","err4","err5","duration" - RED.

**The simplest way to use** the library is like this (as shown in the [simpleLog example](../examples/simpleLog/main.go)):
```
package main
import (
	"fmt"
	"github.com/est357/owlk8s"
	"time"
)
func main() {
	var timeInterval time.Duration = 1000
	results := make(owlk8s.ResMap)
	owlk8s.NewController(results).Run()
	for {
		for k, v := range results {
			if v.Results != nil {
				res := make(map[string]*uint64)
				v.Results(res)
				fmt.Printf("IP: %s, OwnerObj: %s, Pod: %s, NS: %s, SVC: %s, Duration: %d, Requests: %d, Err4: %d, Err5: %d\n", k,
					v.OwnerName, v.PodName, v.NS, v.SvcName, *res["duration"]/1000, *res["requests"], *res["err4"], *res["err5"])
			}
		}
		time.Sleep(time.Millisecond * timeInterval)
	}
}
```
About the metrics:
* requests - HTTP calls with all response codes always increasing value (counter)
* err4 - HTTP calls with response 4xx always increasing value (counter)
* err5 - HTTP calls with response 5xx always increasing value (counter)
* duration - time passed between a HTTP call and a HTTP response in microseconds. You get the value for the latest HTTP call. Can go up and down (gauge).

## Examples
The examples folder contains implementations like:
* simpleLog - the simplest implementation which can be used to understand how it works
* logJson - can be used with fluentd to send logs further to ELK for example
* influxdb - sends to an Influxdb to be graphed with Grafana
* prometheus - implements a /metrics endpoint to be scraped by a prometheus server

All examples have the necessary k8s manifests and build.sh scripts to deploy and test. They are set up for minikube. In order to be used on a real cluster they will need adjustment(image,restartPolicy, build.sh). All examples have their own README files which further describe how to use them.

## Configuration
**Annotations**

`owlk8s/ignore: "true"` - to exclude pods from being monitored one can set this annotation at pod, namespace or node scope.

**ENV Variables**

debug - env variable which if set to true will make the implementation output debug information to stdout.

cleanBPFMap - Cleans the duration_start eBPF map which holds the HTTP request details until the response comes. It should only be needed in very odd and special cases (flood of requests with no reply at the TCP level - maybe because of some filtering or misbehaving program which should be very unlikely since all action is occurring inside a node) in which this duration_start eBPF map might fill up. Incurs a big performance penalty due to some Go runtime.syscall which I have not investigated.

nodename - set like below in the DaemonSet manifest; examples already contain this
```
env:
- name: nodename
	valueFrom:
	 fieldRef:
		 fieldPath: spec.nodeName
```

## Packages
**owlk8s**

Used for monitoring k8s clusters via k8s Informers.

**metrics**

Can be used standalone as an abstraction to all eBPF monitoring logic to create monitoring tools for other types of workload platforms (maybe docker on VM,BM). Constructor takes the interface IP and interface index of the monitored interface.
```
func NewBPFLoader(IP string, ifIndex int) *BpfLoader
```

**helpers**

Just some helpers functions used in this module.

## Caveats
At the time of this writing minikube has the default limit for MEMLOCK set and thus BPF maps cannot be loaded as they use locked memory. To circumvent this I used this to start minikube:
```
#!/bin/bash

minikube start && minikube ssh -- bash -c 'echo && sudo sed -re  "s/LimitCORE=infinity/LimitCORE=infinity\\nLimitMEMLOCK=infinity/" -i /usr/lib/systemd/system/docker.service && sudo systemctl daemon-reload && sudo systemctl restart docker'
```
Unfortunately the Container-Optimized OS on Gcloud also suffers from the problem mentioned above. Use the Ubuntu images on GKE if possible. Those work.

Duration is the last value seen by the eBPF program at the moment in time that the user space program is reading it. Hence if you want to get more resolution for duration you should read more often by adjusting the `timeInterval` variable.
