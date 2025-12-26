package e2e

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"helm.sh/helm/v4/pkg/cli"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
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

func deployMetalLB(
	ctx context.Context,
	log Logger,
	client *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	kubeconfigPath string,
	skipCleanup bool,
) (func(), error) {
	log.Logf("Deploying metallb chart %s", metalLBChartVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	if err := installChart(
		ctx,
		log,
		settings,
		metalLBChartRepo,
		"metallb",
		"metallb",
		metalLBChartVersion,
		metalLBNamespace,
		true,
		nil,
	); err != nil {
		return nil, fmt.Errorf("installing chart: %w", err)
	}

	cidr, err := detectNodeCIDR(ctx, log, client)
	if err != nil {
		return nil, fmt.Errorf("detecting node CIDR: %w", err)
	}

	start, end, err := getIPRange(cidr)
	if err != nil {
		return nil, fmt.Errorf("calculating IP range: %w", err)
	}

	log.Logf("Configuring MetalLB with IP range: %s-%s", start, end)

	if err := applyMetalLBConfig(ctx, log, dynamicClient, start, end); err != nil {
		return nil, fmt.Errorf("applying MetalLB config: %w", err)
	}

	return func() {
		if skipCleanup {
			log.Logf("Skipping cleanup of MetalLB")
			return
		}
		log.Logf("Cleaning up MetalLB")
		if err := uninstallChart(context.Background(), log, settings, "metallb", metalLBNamespace); err != nil {
			log.Logf("Uninstalling chart: %v", err)
		}

		if err := deleteNamespaceAndWait(context.Background(), log, client, metalLBNamespace); err != nil {
			log.Logf("Deleting namespace: %v", err)
		}
	}, nil
}

func detectNodeCIDR(ctx context.Context, log Logger, client *kubernetes.Clientset) (string, error) {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("listing nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		return "", fmt.Errorf("no nodes found")
	}

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
		return "", fmt.Errorf("could not find node with IPv4 InternalIP")
	}

	localAddr, err := findEgressNIC(internalIP)
	if err != nil {
		return "", fmt.Errorf("finding egress NIC: %w", err)
	}

	if localAddr == nil {
		return "", fmt.Errorf("nil local address")
	}

	ipNet, err := ipNetworkForIP(localAddr.IP)
	if err != nil {
		return "", fmt.Errorf("getting IP network from local address: %w", err)
	}

	if ipNet == nil {
		return "", fmt.Errorf("nil IP network")
	}

	ones, _ := ipNet.Mask.Size()
	return fmt.Sprintf("%s/%d", internalIP, ones), nil
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

// Accepts a CIDR, carves out a range of 50 addresses to be used by MetalLB and returns the range's
// start and end addresses as strings.
func getIPRange(cidr string) (string, string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", "", fmt.Errorf("invalid CIDR: %s", cidr)
	}

	ip4 := ipnet.IP.To4()
	if ip4 == nil {
		return "", "", fmt.Errorf("IPv6 support not implemented")
	}

	ones, bits := ipnet.Mask.Size()
	if bits != 32 {
		return "", "", fmt.Errorf("IPv6 support not implemented")
	}

	// Ensure the CIDR is big enough.
	if ones > 26 {
		return "", "", fmt.Errorf("subnet too small for MetalLB range")
	}

	broadcast := net.IP(make([]byte, 4))
	for i := range ip4 {
		broadcast[i] = ip4[i] | ^ipnet.Mask[i]
	}

	broadcastVal := binary.BigEndian.Uint32(broadcast)

	endVal := broadcastVal - 5
	startVal := endVal - 50

	startIP := make(net.IP, 4)
	binary.BigEndian.PutUint32(startIP, startVal)

	endIP := make(net.IP, 4)
	binary.BigEndian.PutUint32(endIP, endVal)

	return startIP.String(), endIP.String(), nil
}

func applyMetalLBConfig(ctx context.Context, log Logger, client dynamic.Interface, start, end string) error {
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

	if err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := client.Resource(gvrPool).Namespace(metalLBNamespace).Create(ctx, ipAddressPool, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				return true, nil
			}
			log.Logf("Creating IPAddressPool: %v", err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return fmt.Errorf("creating IPAddressPool: %w", err)
	}

	if err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := client.Resource(gvrAdv).Namespace(metalLBNamespace).Create(ctx, l2Advertisement, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				return true, nil
			}
			log.Logf("Creating L2Advertisement: %v", err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return fmt.Errorf("creating L2Advertisement: %w", err)
	}

	return nil
}
