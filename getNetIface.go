package owlk8s

import (
	"net"

	"github.com/est357/owlk8s/helpers"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/klog"
)

func getNetIface(ifNames map[string]int) {

	ifs, err := net.Interfaces()
	if err != nil {
		klog.Errorln("Unable to get network interfaces: ", err)
	}

	for ip := range ifNames {

		if route, err := netlink.RouteGet(net.ParseIP(ip)); err == nil && len(route) == 1 {
			if link, err := netlink.LinkByIndex(route[0].LinkIndex); err == nil && link.Type() == "bridge" {
				brMemberName := getBridgeInterface(link, ip, ifs)
				// If we can't get the veth interface we remove this IP from the map
				if brMemberName != nil {
					ifNames[ip] = *brMemberName
				} else {
					klog.Errorln("Could not get bridge member interface for IP:", ip)
					delete(ifNames, ip)
				}
			} else if err != nil {
				klog.Errorf("Netlink could not get interface by name %s", err.Error())
			} else {
				helpers.Debug("IP: %s will be filered on IFid: %d", ip, link.Attrs().Index)
				ifNames[ip] = link.Attrs().Index
			}
		} else {
			// We remove the entry from the map if we can't get it's interface.
			klog.Errorf("Could not get routes or more than 1 route was returned. Removed IP %s from list.", ip)
			delete(ifNames, ip)
		}
	}

}

func getBridgeInterface(link netlink.Link, ip string, ifs []net.Interface) *int {

	var mac string
	var retVal *int

	// Ping is needed so that arp and bridge tables get populated.
	helpers.Ping(ip)

	neigh, err := netlink.NeighList(link.Attrs().Index, int(unix.AF_INET))
	if err != nil {
		klog.Errorln("Error getting ARP table:", err.Error())
	}
	for _, n := range neigh {
		if net.ParseIP(ip).Equal(n.IP) {
			helpers.Debug("MAC from ARP for net if %s acquired: %s", link.Attrs().Name, n.HardwareAddr.String())
			mac = n.HardwareAddr.String()
		}
	}

	// Run through all interfaces again to check for veth bridge members
	for _, v := range ifs {
		link, err := netlink.LinkByName(v.Name)
		if err != nil {
			klog.Errorf("Netlink could not get interface by name %s", err.Error())
		}
		if link.Type() == "veth" {
			helpers.Debug("Bridge interface from if %s with type %s", v.Name, link.Type())
			// Get MACs in AF_BRIDGE for interfaces and match them to AF_INET ones to
			// get the IP -> veth interface relationship
			neighBr, errBr := netlink.NeighList(link.Attrs().Index, int(unix.AF_BRIDGE))
			if errBr != nil {
				klog.Errorln("Error getting macs for bridge interface:", errBr.Error())
			}

			for _, nBr := range neighBr {
				// We assume here that there are no duplicate MACs as they should be
				// unique at least at the host level. But standards violations may
				// happen for various reasons.
				if nBr.HardwareAddr.String() == mac {
					klog.Infof("Found veth interface name: %s for IP: %s and MAC: %s\n",
						v.Name, ip, nBr.HardwareAddr.String())
					return &v.Index
				}
			}
		}
	}
	return retVal
}
