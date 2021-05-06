package metrics

import (
	"bytes"
	"github.com/cilium/ebpf"
	"github.com/est357/owlk8s/helpers"
	// "github.com/vishvananda/netlink"
	"k8s.io/klog"
	"os"
	"strings"
	"syscall"
	"time"
)

// BpfLoader will load the BPF program and receive metrics
type BpfLoader struct {
	eBPFProgramName     string
	eBPFMapName         map[string]string
	eBPFMapKeys         map[string]uint32
	netIfIndex          int
	iP                  string
	eBPFMaps            map[string]*ebpf.Map
	sock                int
	detachedEbpfProgram *ebpf.Program
	cleanTimer          int64
}

// NewBPFLoader creates a eBPF loader
func NewBPFLoader(IP string, ifIndex int) *BpfLoader {
	eBPFMaps := make(map[string]*ebpf.Map)
	mapNames := map[string]string{
		"met": "metrics_map",
		"dur": "duration_start",
	}
	// eBPFMapKeys keeps key to metric counter correspondence.
	// 1 key is for the pod IP adress sent to the eBPF filter program.
	eBPFMapKeys := map[string]uint32{
		"duration": 2,
		"requests": 3,
		"err4":     4,
		"err5":     5,
	}

	return &BpfLoader{
		eBPFProgramName: "http_filter",
		eBPFMapName:     mapNames,
		eBPFMapKeys:     eBPFMapKeys,
		eBPFMaps:        eBPFMaps,
		netIfIndex:      ifIndex,
		iP:              IP,
	}
}

// Load method loads the BPF program into the kernel
func (bl *BpfLoader) Load() *BpfLoader {

	const SO_ATTACH_BPF = 50
	eBPFprogram := getEBPFProg()
	index := bl.netIfIndex

	spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(eBPFprogram))
	if err != nil {
		klog.Errorln("Error loading eBPF collectionSpec: ", err)
	}

	coll, err := ebpf.NewCollectionWithOptions(spec, ebpf.CollectionOptions{})
	if err != nil {
		klog.Errorln("Error getting the eBPF program collection: ", err)
	}
	defer coll.Close()

	sock, err := helpers.OpenRawSock(index)
	if err != nil {
		klog.Errorln("Error opening raw socket for eBPF program attachment: ", err)
	}
	bl.sock = sock

	bl.detachedEbpfProgram = coll.DetachProgram(bl.eBPFProgramName)
	if bl.detachedEbpfProgram == nil {
		klog.Errorf("Error: no program named %s found !", bl.eBPFProgramName)
	}

	if err := syscall.SetsockoptInt(bl.sock, syscall.SOL_SOCKET, SO_ATTACH_BPF, bl.detachedEbpfProgram.FD()); err != nil {
		klog.Errorln("Error ataching BPF program to socket: ", err)
	}

	helpers.Debug("Filtering on net if index: %d", index)

	bl.eBPFMaps[bl.eBPFMapName["dur"]] = coll.DetachMap(bl.eBPFMapName["dur"])
	if bl.eBPFMaps[bl.eBPFMapName["dur"]] == nil {
		klog.Errorf("No map named %s found", bl.eBPFMapName)
	}

	bl.eBPFMaps[bl.eBPFMapName["met"]] = coll.DetachMap(bl.eBPFMapName["met"])
	if bl.eBPFMaps[bl.eBPFMapName["met"]] == nil {
		klog.Errorf("No map named %s found", bl.eBPFMapName)
	}

	// Populate IP address to the map so that the C program
	// can read it and filter only that traffic
	const keyIPAddr uint32 = 1
	if err := bl.eBPFMaps[bl.eBPFMapName["met"]].Put(keyIPAddr, uint64(helpers.Htonl(helpers.IP4toDec(bl.iP)))); err != nil {
		klog.Errorln("Error from map PUT: ", err.Error())
	}

	return bl
}

// GetMetrics retrieves the metrics from eBPF the maps.
func (bl *BpfLoader) GetMetrics(values map[string]*uint64) {

	for k, v := range bl.eBPFMapKeys {
		if values[k] == nil {
			values[k] = new(uint64)
		}

		if err := bl.eBPFMaps[bl.eBPFMapName["met"]].Lookup(&v, values[k]); err != nil {
			if strings.Contains(err.Error(), "key does not exist") {
				helpers.Debug("Pod_IP: %s. Key does not exist yet for map key %s, returning 0 instead. Error: %s.",
					bl.iP, k, err.Error())
				*values[k] = 0
			} else {
				klog.Errorln("Map or key invalid: ", err.Error())
			}

		}
	}

	if v := os.Getenv("cleanBPFMap"); v == "true" {
		go bl.cleanDurMap()
	}

}

// cleanDurMap removes all entries from duration_start map so that it does
// not fill up. Normally this map should only have one entry into it at the
// time between the http request and the http response. The eBPF program tries
// to clean up but in case of calls which time out from the caller side and
// in weird circumstances this map might still fill up resulting in metrics
// not being updated anymore. Runs every 5 minutes in its own routine and
// only if the "cleanBPFMap" env varible is set  because it has !!HUGE!!
// performance impact at the syscall level.
func (bl *BpfLoader) cleanDurMap() {
	if bl.cleanTimer == 0 {
		bl.cleanTimer = time.Now().Unix()
		return
	}
	// if (time.Now().Unix() - bl.cleanTimer) < 300 {
	// 	return
	// }
	if v, ok := bl.eBPFMaps[bl.eBPFMapName["dur"]]; ok {
		helpers.Debug("%s", "MapCleaner started.")
		var key struct {
			SrcIP   uint32
			DstIP   uint32
			SrcPort uint16
			DstPort uint16
		}
		var val uint64

		count := 0
		for v.Iterate().Next(&key, &val) {
			if count > 1 {
				// Check if timestamp from map is older than 120s. In edge cases where
				// response is slow maybe because a lot of data is being sent in the
				// request we may end up cleaning usefull entries. These entries would
				// pile up because they are slow.
				// In the map val is in ns.
				if tnow, _ := helpers.GetMtime(); val <= (tnow - 120000000000) {
					helpers.Debug("MapCleaner detected more elements in map %s. Running cleanup for key with IPs: src:%d dst:%d; ports: src:%d dst:%d, with val: %d, tnow: %d",
						bl.eBPFMapName["dur"], helpers.Ntohl(key.SrcIP), helpers.Ntohl(key.DstIP), helpers.Ntohs(key.SrcPort), helpers.Ntohs(key.DstPort), val, tnow)
					if err := v.Delete(&key); err != nil {
						klog.Errorln("MapCleaner could not delete entry in map: ",
							bl.eBPFMapName["dur"])
					}
				}
			}
			count++
		}

	} else {
		helpers.Debug("MapCleaner not running because map %s is not loaded", bl.eBPFMapName["dur"])
	}
	bl.cleanTimer = time.Now().Unix()
}

// CleanEBPF closes all eBPF objects
func (bl *BpfLoader) CleanEBPF() {
	helpers.Debug("%s", "From CleanEBPF")
	if err := bl.detachedEbpfProgram.Close(); err != nil {
		klog.Errorf("Error closing eBPF program for Endpoint  IP: %s. Error: %s",
			bl.iP, err.Error())
	}

	for i := range bl.eBPFMapName {
		if err := bl.eBPFMaps[i].Close(); err != nil {
			klog.Errorf("Error closing map for Endpoint  IP: %s. Error: %s",
				bl.iP, err.Error())
		}
	}
	if err := syscall.Close(bl.sock); err != nil {
		klog.Errorf("Error closing socket for Endpoint  IP: %s. Error: %s",
			bl.iP, err.Error())
	}

}
