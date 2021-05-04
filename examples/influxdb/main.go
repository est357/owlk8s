package main

import (
	"fmt"
	"github.com/est357/owlk8s"
	iflxClient "github.com/influxdata/influxdb1-client/v2"
	"k8s.io/klog"
	"os"
	"strings"
	"time"
)

func newInflxQueryObj(host, port, user, pass string) iflxClient.Client {

	c, err := iflxClient.NewHTTPClient(iflxClient.HTTPConfig{
		Addr:     fmt.Sprintf("http://%s:%s", host, port),
		Username: user,
		Password: pass})

	if err != nil {
		klog.Errorln("Error creating InfluxDB Client: ", err.Error())

	}

	return c

}

func main() {
	// timeInterval is what gives the resolution of the metrics.
	// A lower value yields higher resolution. It also depends on how much
	// traffic you are getting meaning that with higher traffic you may want
	// higher resolution. You could set this as an env variable.
	var timeInterval time.Duration = 1000
	var envVars = map[string]string{
		"IFLXHOST":  "",
		"IFLXPORT":  "",
		"IFLXUSER":  "",
		"IFLXPASS":  "",
		"IFLXDB":    "",
		"IFLXTABLE": "",
	}
	// Get Influx env variables needed
	for k := range envVars {
		if val := os.Getenv(k); val != "" {
			envVars[k] = val
		} else {
			klog.Fatalln("Missing required env var: ", k)
		}
	}

	results := make(owlk8s.ResMap)
	owlk8s.NewController(results).Run()

	for {

		bp, err := iflxClient.NewBatchPoints(iflxClient.BatchPointsConfig{
			Database:  envVars["IFLXDB"],
			Precision: "ms",
		})
		if err != nil {
			klog.Errorln("Could not create influx BatchPoints: ", err.Error())
		}

		for _, v := range results {
			// Always check the Results function value is not nil
			// because the controller might have NOT found any
			// svc endpoints to monitor yet.
			if v.Results != nil {
				// v.Results() writes to the user's map the following
				// keys: "duration", "requests", "err4", "err5"
				res := make(map[string]*uint64)
				v.Results(res)

				tags := map[string]string{
					"namespace": v.NS,
					"podname":   v.PodName,
					"owner":     v.OwnerName,
					"service":   v.SvcName,
				}
				// Conversion needed to uint because of issue in influxdb library.
				// Still 64bit though.
				fields := map[string]interface{}{
					"duration": uint(*res["duration"]),
					"requests": uint(*res["requests"]),
					"err4":     uint(*res["err4"]),
					"err5":     uint(*res["err5"]),
				}
				pt, err := iflxClient.NewPoint(envVars["IFLXTABLE"], tags, fields, time.Now())
				if err != nil {
					klog.Errorln("Error creating influx point", err)
				}
				bp.AddPoint(pt)
				fmt.Printf("Results: Owner obj %s, pod %s, in NS %s, with svc object %s: Duration: %d, Requests %d, Err4: %d Err5 %d\n",
					v.OwnerName, v.PodName, v.NS, v.SvcName, *res["duration"]/1000, *res["requests"], *res["err4"], *res["err5"])

			}
		}
		c := newInflxQueryObj(envVars["IFLXHOST"], envVars["IFLXPORT"], envVars["IFLXUSER"], envVars["IFLXPASS"])
		if err := c.Write(bp); err == nil {
			klog.Infoln("Influx query successfull !")
			c.Close()
		} else {
			if strings.Contains(err.Error(), "database not found") {
				q := iflxClient.NewQuery("create database "+envVars["IFLXDB"], "", "")
				if r, err := c.Query(q); err != nil || r.Error() != nil {
					klog.Errorln("Could not create database: ", err.Error(), r.Error())
					c.Close()
				}
			}
			klog.Errorln("Influx query failed: ", err.Error())
			c.Close()
		}
		time.Sleep(time.Millisecond * timeInterval)

	}
}
