package owlk8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/klog"
)

func getOwnerAnno(ns, podName, nodename string, clientset *kubernetes.Clientset) (string, bool) {

	var annoPrefix = "owlk8s/"
	var ignoreAnno = annoPrefix + "ignore"
	var anno bool
	podVal, err := clientset.CoreV1().Pods(ns).
		Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		klog.Exitln("Error getting pods")
	}
	if val, ok := podVal.GetAnnotations()[ignoreAnno]; ok && val == "true" {
		anno = true
	} else if getNSAnno(ns, clientset, ignoreAnno) {
		anno = true
	} else if getNodeAnno(clientset, nodename, ignoreAnno) {
		anno = true
	} else {
		anno = false
	}

	var name string
	for _, or := range podVal.OwnerReferences {
		if or.Controller != nil && *or.Controller {
			// klog.Infoln("KIND", or.Kind)
			// Aici am ramas. De implementat astea. Pt fiecare caz
			switch or.Kind {
			case "ReplicaSet":
				name = getOwnerRS(ns, or.Name, clientset)
			case "StatefulSet":
				name = or.Name
			case "DaemonSet":
				name = or.Name
			}
		}

	}

	return name, anno
}

func getOwnerRS(ns, rsName string, clientset *kubernetes.Clientset) string {

	var name string
	rsVal, err := clientset.AppsV1().ReplicaSets(ns).
		Get(context.TODO(), rsName, metav1.GetOptions{})
	if err != nil {
		klog.Exitln("Error getting ReplicaSets.")
	}
	for _, or := range rsVal.OwnerReferences {
		if or.Controller != nil && *or.Controller {
			if or.Kind == "Deployment" {
				name = or.Name
			} else {
				name = rsName
			}
		}
	}
	return name
}

func getNSAnno(ns string, clientset *kubernetes.Clientset, ignoreAnno string) bool {
	nsVal, err := clientset.CoreV1().Namespaces().
		Get(context.TODO(), ns, metav1.GetOptions{})
	if err != nil {
		klog.Exitln("Error getting namespaces.")
	}
	if val, ok := nsVal.GetAnnotations()[ignoreAnno]; ok && val == "true" {
		return true
	}
	return false
}

func getNodeAnno(clientset *kubernetes.Clientset, nodename, ignoreAnno string) bool {
	nodeVal, err := clientset.CoreV1().Nodes().
		Get(context.TODO(), nodename, metav1.GetOptions{})
	if err != nil {
		klog.Exitln("Error getting nodes.")
	}
	if val, ok := nodeVal.GetAnnotations()[ignoreAnno]; ok && val == "true" {
		return true
	}
	return false
}
