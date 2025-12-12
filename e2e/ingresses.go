package e2e

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func createIngresses(ctx context.Context, t *testing.T, client *kubernetes.Clientset, ns string, ingresses []*networkingv1.Ingress, skipCleanup bool) func() {
	for _, ingress := range ingresses {
		y, err := toYAML(ingress)
		require.NoError(t, err)
		t.Logf("Creating ingress:\n%s", y)

		_, err = client.NetworkingV1().Ingresses(ns).Create(ctx, ingress, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	return func() {
		if skipCleanup {
			t.Log("Skipping cleanup of ingresses")
			return
		}
		for _, ingress := range ingresses {
			t.Logf("Deleting ingress %s", ingress.Name)
			err := client.NetworkingV1().Ingresses(ns).Delete(context.Background(), ingress.Name, metav1.DeleteOptions{})
			if err != nil {
				t.Logf("Deleting ingress %s: %v", ingress.Name, err)
			}
		}
	}
}

func waitForIngressAddresses(ctx context.Context, t *testing.T, client *kubernetes.Clientset, ns string, ingresses []*networkingv1.Ingress) map[types.NamespacedName]net.IP {
	addresses := map[types.NamespacedName]net.IP{}
	for _, ingress := range ingresses {
		t.Logf("Waiting for ingress %s to get an address", ingress.Name)
		var address string
		err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
			ing, err := client.NetworkingV1().Ingresses(ns).Get(ctx, ingress.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if len(ing.Status.LoadBalancer.Ingress) > 0 {
				if ing.Status.LoadBalancer.Ingress[0].IP != "" {
					address = ing.Status.LoadBalancer.Ingress[0].IP
					return true, nil
				}
				if ing.Status.LoadBalancer.Ingress[0].Hostname != "" {
					address = ing.Status.LoadBalancer.Ingress[0].Hostname
					return true, nil
				}
			}
			return false, nil
		})
		require.NoError(t, err)
		nn := types.NamespacedName{Namespace: ns, Name: ingress.Name}
		addresses[nn] = net.ParseIP(address)
	}

	return addresses
}
