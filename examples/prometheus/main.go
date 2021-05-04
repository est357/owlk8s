package main

import (
	"fmt"
	"github.com/est357/owlk8s"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strings"
)

func init() {

	runtime.SetBlockProfileRate(1)
}

// MyCollector implements the prometheus Collector.
type MyCollector struct {
	counterDesc    map[string]*prometheus.Desc
	results        owlk8s.ResStruct
	metricNameDesc map[string]string
	dummy          uint
	resMap         owlk8s.ResMap
	oldResMap      owlk8s.ResMap
	collectors     map[string]*MyCollector
}

// newRegisterCollector registers new metrics when pods change or added.
// It's implemented as an prometheus "unchecked collector" meaning Collect
// and Describe  methods don't return anything but run the
// registerNewMetrics method.
func newRegisterCollector(results owlk8s.ResMap) *MyCollector {
	return &MyCollector{
		dummy: 1,
		// counterDesc: map[string]*prometheus.Desc{"dummy": &prometheus.Desc{}},
		resMap:     results,
		oldResMap:  make(owlk8s.ResMap),
		collectors: make(map[string]*MyCollector),
	}
}

// newMyCollector implements the standard prometheus Collector.
func newMyCollector(results owlk8s.ResStruct, metricNameDesc map[string]string) *MyCollector {
	// Prometheus const labels not dynamic!
	l := prometheus.Labels{
		"ownerObj": strings.ReplaceAll(results.OwnerName, "-", "_"),
		"podName":  strings.ReplaceAll(results.PodName, "-", "_"),
		"ns":       strings.ReplaceAll(results.NS, "-", "_"),
		"svc":      strings.ReplaceAll(results.SvcName, "-", "_"),
	}
	var counterDesc = make(map[string]*prometheus.Desc)
	for k, v := range metricNameDesc {
		counterDesc[k] = prometheus.NewDesc(k, v,
			nil, l)
	}

	return &MyCollector{
		results:        results,
		metricNameDesc: metricNameDesc,
		counterDesc:    counterDesc,
	}
}

// Describe is called by the Gatherer to get the hane and help.
func (c *MyCollector) Describe(ch chan<- *prometheus.Desc) {
	if c.dummy == 1 {
		return
	}
	for _, v := range c.counterDesc {
		ch <- v
	}
}

// Collect is called by the Gatherer - a prometheus concept - and does the
// actual collection of eBPF metrics as well as handles initialization
// of metrics for new Endpoints/Pods k8s objects.
func (c *MyCollector) Collect(ch chan<- prometheus.Metric) {

	// Here we register new metrics.
	if c.dummy == 1 {
		c.registerNewMetrics()
		return
	}

	// We get the actual metrics from the eBPF program.
	// Always check the Results function value is not nil
	// because the controller might have NOT found any
	// svc endpoints to monitor yet.
	if c.results.Results == nil {
		return
	}

	res := make(map[string]*uint64)
	c.results.Results(res)
	for k, v := range res {

		// Only duration is gauge
		if k == "duration" {

			ch <- prometheus.MustNewConstMetric(
				c.counterDesc[k],
				prometheus.GaugeValue,
				// Value from kernel is in us. Here we want ms.
				float64(*v)/1000,
			)
		} else {

			ch <- prometheus.MustNewConstMetric(
				c.counterDesc[k],
				prometheus.CounterValue,
				float64(*v),
			)
		}
	}

}

// registerNewMetrics keeps track of already seen pods and
// registers/unregisters metrics when needed.
func (c *MyCollector) registerNewMetrics() {
	// Create map that has the same keys as the BPF metrics results map.
	// keys: "requests", "err4", "err5", "duration" RED
	metricNameDesc := map[string]string{
		"requests": "Total number of requests",
		"err4":     "Number of 4xx requests",
		"err5":     "Number of 5xx requests",
		"duration": "Time last request took in ms",
	}
	for k, v := range c.oldResMap {
		if _, ok := c.resMap[k]; !ok || v.PodName != c.resMap[k].PodName {

			prometheus.Unregister(c.collectors[k])

			delete(c.oldResMap, k)
		}
	}
	for k, v := range c.resMap {
		// Only add results that are not already added
		// thus present in the oldResMap.
		if _, ok := c.oldResMap[k]; !ok {

			col := newMyCollector(*v, metricNameDesc)
			prometheus.MustRegister(col)
			c.collectors[k] = col
			c.oldResMap[k] = v
		}
		// This is just for logging purposes..
		if v.Results != nil {
			// v.Results is the user's Map map[string]*uint64 with following
			// keys: "requests", "err4", "err5", "duration" RED
			res := make(map[string]*uint64)
			v.Results(res)
			fmt.Printf("IP: %s, OwnerObj: %s, Pod: %s, NS: %s, SVC: %s, Duration: %d, Requests: %d, Err4: %d Err5: %d\n", k,
				v.OwnerName, v.PodName, v.NS, v.SvcName, *res["duration"]/1000, *res["requests"], *res["err4"], *res["err5"])
		}
	}

}

func main() {

	port := os.Getenv("listenPort")
	if port == "" {
		klog.Exitln("listenPort env variable must exist!")
	}

	results := make(owlk8s.ResMap)
	owlk8s.NewController(results).Run()
	prometheus.MustRegister(newRegisterCollector(results))

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":"+port, nil)

}
