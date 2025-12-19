package e2e

import (
	"context"
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

	// Determine IP range.
	cidr := getClusterCIDR(ctx, t, client)
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

func getClusterCIDR(ctx context.Context, t TestingT, client *kubernetes.Clientset) string {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, nodes.Items)

	for _, addr := range nodes.Items[0].Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}
	t.Fatalf("Could not find node InternalIP")
	return ""
}

func getIPRange(t TestingT, ipStr string) (string, string) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		t.Fatalf("Invalid IP: %s", ipStr)
	}

	ip4 := ip.To4()
	if ip4 == nil {
		t.Fatalf("IPv6 not supported in this helper yet")
	}

	// We assume a /16 network for Kind/Docker.
	// We use the last octet 200-250 of the last subnet in the /16.
	// e.g. 172.18.0.2 -> 172.18.255.200 - 172.18.255.250
	return fmt.Sprintf("%d.%d.255.200", ip4[0], ip4[1]), fmt.Sprintf("%d.%d.255.250", ip4[0], ip4[1])
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
