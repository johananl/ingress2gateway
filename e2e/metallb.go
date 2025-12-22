package e2e

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v4/pkg/cli"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// We use MetalLB to expose LoadBalancer services in e2e tests on kind clusters or k8s clusters on
// other providers with no load balancer implementation.

const (
	metalLBChartVersion = "0.13.12"
	metalLBChartRepo    = "https://metallb.github.io/metallb"
	metalLBNamespace    = "metallb-system"
)

func deployMetalLB(ctx context.Context, t TestingT, client *kubernetes.Clientset, dynamicClient dynamic.Interface, kubeconfigPath string, skipCleanup bool) func() {
	t.Logf("Deploying metallb chart %s", metalLBChartVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	installChart(t, settings, metalLBChartRepo, "metallb", "metallb", metalLBChartVersion, metalLBNamespace, true, nil)

	cidr := detectNodeCIDR(ctx, t, client)
	start, end := getIPRange(t, cidr)
	t.Logf("Configuring MetalLB with IP range: %s-%s", start, end)

	applyMetalLBConfig(ctx, t, dynamicClient, start, end)

	return func() {
		if skipCleanup {
			t.Logf("Skipping cleanup of metallb")
			return
		}
		t.Logf("Cleaning up metallb")
		uninstallChart(t, settings, "metallb", metalLBNamespace)

		deleteNamespaceAndWait(ctx, t, client, metalLBNamespace)
	}
}

func detectNodeCIDR(ctx context.Context, t TestingT, client *kubernetes.Clientset) string {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, nodes.Items)

	var internalIP string
	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				if net.ParseIP(addr.Address).To4() != nil {
					internalIP = addr.Address
					break
				}
			}
		}
		if internalIP != "" {
			break
		}
	}

	if internalIP == "" {
		t.Fatalf("Could not find node with IPv4 InternalIP")
	}

	localAddr, err := findEgressNIC(internalIP)
	require.NoError(t, err)
	require.NotNil(t, localAddr)

	ipNet, err := ipNetworkForIP(localAddr.IP)
	require.NoError(t, err)
	require.NotNil(t, ipNet)

	ones, _ := ipNet.Mask.Size()
	return fmt.Sprintf("%s/%d", internalIP, ones)
}

// Searches the local network interfaces for an interface with the specified IP and returns the
// matching address as a pointer to net.IPNet.
func ipNetworkForIP(ip net.IP) (*net.IPNet, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("listing interfaces: %w", err)
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				if ipNet.IP.Equal(ip) {
					return ipNet, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("could not find local interface for IP %s", ip.String())
}

// Figures out which local NIC is used to get to the specified IP.
func findEgressNIC(ip string) (*net.UDPAddr, error) {
	// We're using a UDP socket without writing so we aren't sending any packets.
	conn, err := net.Dial("udp", fmt.Sprintf("%s:80", ip))
	if err != nil {
		return nil, fmt.Errorf("dialing node IP %s: %w", ip, err)
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr), nil
}

// Accepts a CIDR, carves out a range to be used by MetalLB and returns that range as a start IP
// and end IP.
func getIPRange(t TestingT, cidr string) (string, string) {
	// Compute the network address (first address in CIDR).
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("Invalid CIDR: %s", cidr)
	}

	// Convert network address to an IP we can do bitwise operations on.
	ip4 := ipnet.IP.To4()
	if ip4 == nil {
		t.Fatalf("IPv6 support not implemented")
	}

	// Compute the subnet mask.
	ones, bits := ipnet.Mask.Size()
	if bits != 32 {
		t.Fatalf("IPv6 support not implemented")
	}

	// Ensure the CIDR is big enough.
	if ones > 26 {
		t.Fatalf("Subnet too small for MetalLB range")
	}

	// Compute the broadcast address (last address in CIDR).
	broadcast := net.IP(make([]byte, 4))
	for i := range ip4 {
		broadcast[i] = ip4[i] | ^ipnet.Mask[i]
	}

	// Convert broadcast address to an integer.
	broadcastVal := binary.BigEndian.Uint32(broadcast)

	// Define range.
	endVal := broadcastVal - 5
	startVal := endVal - 50

	// Construct start IP.
	startIP := make(net.IP, 4)
	binary.BigEndian.PutUint32(startIP, startVal)

	// Construct end IP.
	endIP := make(net.IP, 4)
	binary.BigEndian.PutUint32(endIP, endVal)

	return startIP.String(), endIP.String()
}

func applyMetalLBConfig(ctx context.Context, t TestingT, client dynamic.Interface, start, end string) {
	ipAddressPool := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "metallb.io/v1beta1",
			"kind":       "IPAddressPool",
			"metadata": map[string]interface{}{
				"name":      "default",
				"namespace": metalLBNamespace,
			},
			"spec": map[string]interface{}{
				"addresses": []string{
					fmt.Sprintf("%s-%s", start, end),
				},
			},
		},
	}

	l2Advertisement := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "metallb.io/v1beta1",
			"kind":       "L2Advertisement",
			"metadata": map[string]interface{}{
				"name":      "default",
				"namespace": metalLBNamespace,
			},
		},
	}

	gvrPool := schema.GroupVersionResource{Group: "metallb.io", Version: "v1beta1", Resource: "ipaddresspools"}
	gvrAdv := schema.GroupVersionResource{Group: "metallb.io", Version: "v1beta1", Resource: "l2advertisements"}

	require.Eventually(t, func() bool {
		_, err := client.Resource(gvrPool).Namespace(metalLBNamespace).Create(ctx, ipAddressPool, metav1.CreateOptions{})
		if err != nil {
			// If it already exists, we can ignore or update. For now, let's assume clean state or ignore exists.
			if strings.Contains(err.Error(), "already exists") {
				return true
			}
			t.Logf("Creating IPAddressPool: %v", err)
			return false
		}
		return true
	}, 2*time.Minute, 5*time.Second, "Failed to create IPAddressPool")

	require.Eventually(t, func() bool {
		_, err := client.Resource(gvrAdv).Namespace(metalLBNamespace).Create(ctx, l2Advertisement, metav1.CreateOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				return true
			}
			t.Logf("Creating L2Advertisement: %v", err)
			return false
		}
		return true
	}, 2*time.Minute, 5*time.Second, "Failed to create L2Advertisement")
}
