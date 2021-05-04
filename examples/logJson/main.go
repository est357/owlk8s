package main

import (
	"encoding/json"
	"fmt"
	"github.com/est357/owlk8s"
	"k8s.io/klog"
	"time"
)

type jsonResultsT struct {
	Timestamp int64              `json:"timestamp"`
	PodIP     string             `json:"podip"`
	Owner     string             `json:"owner"`
	PodName   string             `json:"podname"`
	Namesapce string             `json:"namespace"`
	Service   string             `json:"service"`
	Metrics   map[string]*uint64 `json:"metrics"`
}

func main() {
	// timeInterval is what gives the resolution of the metrics.
	// A lower value yields higher resolution. It also depends on how much
	// traffic you are getting meaning that with higher traffic you may want
	// higher resolution. You could set this as an env variable.
	var timeInterval time.Duration = 1000
	results := make(owlk8s.ResMap)
	owlk8s.NewController(results).Run()
	for {

		for k, v := range results {
			// Always check the Results function value is not nil
			// because the controller might have NOT found any
			// svc endpoints to monitor yet.
			if v.Results != nil {
				// v.Results() writes to the user's map the following
				// keys: "duration", "requests", "err4", "err5"
				// Duration comes in us, here we convert to ms.
				res := make(map[string]*uint64)
				v.Results(res)

				jsonResults := jsonResultsT{
					Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
					PodIP:     k,
					Owner:     v.OwnerName,
					PodName:   v.PodName,
					Namesapce: v.NS,
					Service:   v.SvcName,
					Metrics:   res,
				}

				if res, err := json.Marshal(jsonResults); err == nil {
					fmt.Println(string(res))
				} else {
					klog.Errorln("Error marshaling json:", err.Error())
				}
			}
		}
		time.Sleep(time.Millisecond * timeInterval)
	}
}
