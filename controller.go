// Package owlk8s is a library for monitoring ClusterIP HTTP services with eBPF.
package owlk8s

import (
	"fmt"
	"os"

	"github.com/est357/owlk8s/helpers"
	"github.com/est357/owlk8s/metrics"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

// ResStruct is the struct part of ResMap the user gets
type ResStruct struct {
	cleanEBPF                       func()
	OwnerName, PodName, NS, SvcName string
	Results                         func(map[string]*uint64)
}

// ResMap is the results map that the user gets.
type ResMap map[string]*ResStruct

// Controller defines a k8s controller for Endpoints
type Controller struct {
	stopper  chan struct{}
	informer cache.SharedIndexInformer
}

// NewController creates a new controller
func NewController(results ResMap) Controller {

	config := helpers.InClusterAuth()

	nodename := os.Getenv("nodename")
	if nodename == "" {
		klog.Exitln("No nodename env varible !")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Exitln(err.Error())
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)

	informer := factory.Core().V1().Endpoints().Informer()

	stopper := make(chan struct{})

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			newEndpoint := obj.(*corev1.Endpoints)
			var oldEndpoint *corev1.Endpoints
			helpers.Debug("%+v %s", newEndpoint, "Added")
			addAndUpdateEndpoint(oldEndpoint, newEndpoint, nodename, results, clientset)
		},
		DeleteFunc: func(obj interface{}) {
			endpoint := obj.(*corev1.Endpoints)
			helpers.Debug("%+v %s", endpoint, "Deleted")
			deleteEndpoint(endpoint, results)

		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			oldEndpoint := oldObj.(*corev1.Endpoints)
			newEndpoint := newObj.(*corev1.Endpoints)
			helpers.Debug("%+v %s", newEndpoint, "Updated")
			addAndUpdateEndpoint(oldEndpoint, newEndpoint, nodename, results, clientset)
		},
	})

	return Controller{stopper: stopper, informer: informer}
}

// Run runs the k8s controller
func (c Controller) Run() {
	go func() {
		defer runtime.HandleCrash()
		c.informer.Run(c.stopper)
		if !cache.WaitForCacheSync(c.stopper, c.informer.HasSynced) {
			runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
			return
		}
		<-c.stopper
	}()
}

func deleteEndpoint(endpoint *corev1.Endpoints, results ResMap) {
	for _, subsets := range endpoint.Subsets {
		for _, address := range subsets.Addresses {
			if _, ok := results[address.IP]; ok {
				results[address.IP].cleanEBPF()
				delete(results, address.IP)
			}
		}
	}
}

func addAndUpdateEndpoint(oldEndpoint, endpoint *corev1.Endpoints,
	nodename string, results ResMap, clientset *kubernetes.Clientset) {

	// This newIP map exists just to store all IPs from the new Endpoint state so
	// that we can check old vs new states.
	var newIP = make(map[string]struct{})
	// This ifNames map contains the IPs for which net interfaces must be found so
	// that we can load the eBPF filter program on them.
	var ifNames = make(map[string]int)
	for _, subsets := range endpoint.Subsets {
		for _, address := range subsets.Addresses {
			if address.NodeName != nil && *address.NodeName == nodename {
				owner, ignoreAnno := getOwnerAnno(address.TargetRef.Namespace,
					address.TargetRef.Name, nodename, clientset)
				if ignoreAnno {
					continue
				}
				if _, ok := results[address.IP]; ok {
					newIP[address.IP] = struct{}{}
					continue
				}
				results[address.IP] = &ResStruct{
					OwnerName: owner,
					NS:        address.TargetRef.Namespace,
					PodName:   address.TargetRef.Name,
					SvcName:   endpoint.GetName(),
				}
				newIP[address.IP] = struct{}{}
				ifNames[address.IP] = -1
			}

		}
	}

	// Remove old IP on enpoint update. If the IP in the old Endpoint object does
	// not exist in the new Enpoint object it means that the pod with that IP or
	// the service object itself was deleted so we remove it from the results map.
	if oldEndpoint != nil {
		for _, subsets := range oldEndpoint.Subsets {
			for _, address := range subsets.Addresses {
				if _, ok := newIP[address.IP]; !ok {
					if _, ok := results[address.IP]; ok {
						// results[address.IP].threadClose <- 1
						results[address.IP].cleanEBPF()
						delete(results, address.IP)
					}
				}
			}
		}
	}
	helpers.Debug("Results Map: %+v", results)

	getNetIface(ifNames)
	var res *metrics.BpfLoader
	for k, v := range ifNames {
		res = metrics.NewBPFLoader(k, v).Load()
		// go res.GetMetrics(results[k].threadClose)
		results[k].Results = res.GetMetrics
		results[k].cleanEBPF = res.CleanEBPF
	}

}
