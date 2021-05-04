package main

import (
	"fmt"
	"github.com/est357/owlk8s"
	"time"
)

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
				// keys: "duration", "requests", "err4", "err5".
				res := make(map[string]*uint64)
				v.Results(res)
				fmt.Printf("IP: %s, OwnerObj: %s, Pod: %s, NS: %s, SVC: %s, Duration: %d, Requests: %d, Err4: %d, Err5: %d\n", k,
					v.OwnerName, v.PodName, v.NS, v.SvcName, *res["duration"], *res["requests"], *res["err4"], *res["err5"])
			}
		}
		time.Sleep(time.Millisecond * timeInterval)
	}
}
