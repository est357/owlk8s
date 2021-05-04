package helpers

import (
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/go-ping/ping"
	"golang.org/x/sys/unix"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Debug prints debug messages to logs
func Debug(format string, message ...interface{}) {

	if debug := os.Getenv("debug"); debug == "true" {

		myFmt := "DEBUG: " + format
		klog.InfoDepth(1, fmt.Sprintf(myFmt, message...))

	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// OutOfClusterAuth creates config and creds for out of cluster auth
func OutOfClusterAuth() (config *rest.Config) {

	var err error
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		klog.Infoln(err.Error())
		os.Exit(3)
	}
	return
}

// InClusterAuth creates config and creds for out of cluster auth
func InClusterAuth() (config *rest.Config) {
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Infoln(err.Error())
		os.Exit(3)
	}
	return
}

// Ping checks ip thus populating ARP and bridge MAC table
func Ping(ip string) {
	pinger := ping.New(ip)

	pinger.SetPrivileged(true)
	pinger.Count = 1
	pinger.Timeout = time.Second
	if err := pinger.Run(); err != nil {
		klog.Errorln("Ping error:", err.Error())
	}
}

// Byteorder converter functions from the internet..

// Htons converts to network byte order short uint16.
func Htons(i uint16) uint16 {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, i)
	return *(*uint16)(unsafe.Pointer(&b[0]))
}

// Htonl converts to network byte order long uint32.
func Htonl(i uint32) uint32 {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, i)
	return *(*uint32)(unsafe.Pointer(&b[0]))
}

// Ntohs converts from network byte order to host uint16.
func Ntohs(i uint16) uint16 {
	return binary.BigEndian.Uint16((*(*[2]byte)(unsafe.Pointer(&i)))[:])
}

// Ntohl converts from network byte order to host uint32.
func Ntohl(i uint32) uint32 {
	return binary.BigEndian.Uint32((*(*[4]byte)(unsafe.Pointer(&i)))[:])
}

// GetMtime gets a timestamp which can be compared with the output
// of bpf_ktime_get_ns() helper. Taken from cilium...
func GetMtime() (uint64, error) {
	var ts unix.Timespec

	err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)
	if err != nil {
		return 0, fmt.Errorf("Unable get time: %s", err)
	}

	return uint64(unix.TimespecToNsec(ts)), nil
}

// IP4toDec transforms and IPv4 to decimal
func IP4toDec(IPv4Addr string) uint32 {
	bits := strings.Split(IPv4Addr, ".")

	b0, _ := strconv.Atoi(bits[0])
	b1, _ := strconv.Atoi(bits[1])
	b2, _ := strconv.Atoi(bits[2])
	b3, _ := strconv.Atoi(bits[3])

	var sum uint32

	// left shifting 24,16,8,0 and bitwise OR

	sum += uint32(b0) << 24
	sum += uint32(b1) << 16
	sum += uint32(b2) << 8
	sum += uint32(b3)

	return sum
}

// OpenRawSock opens a raw socket
func OpenRawSock(index int) (int, error) {
	// const ETH_P_ALL uint16 = 0x00<<8 | 0x03
	const ETH_P_ALL uint16 = 0x03

	sock, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC, int(Htons(ETH_P_ALL)))
	if err != nil {
		return 0, err
	}
	sll := syscall.SockaddrLinklayer{}
	sll.Protocol = Htons(ETH_P_ALL)
	sll.Ifindex = index
	if err := syscall.Bind(sock, &sll); err != nil {
		return 0, err
	}
	return sock, nil
}
