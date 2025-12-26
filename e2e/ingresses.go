package e2e

import (
	"context"
	"fmt"
	"net"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func createIngresses(ctx context.Context, log Logger, client *kubernetes.Clientset, ns string, ingresses []*networkingv1.Ingress, skipCleanup bool) (func(), error) {
	for _, ingress := range ingresses {
		y, err := toYAML(ingress)
		if err != nil {
			return nil, fmt.Errorf("converting ingress to YAML: %w", err)
		}

		log.Logf("Creating ingress:\n%s", y)

		_, err = client.NetworkingV1().Ingresses(ns).Create(ctx, ingress, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating ingress: %w", err)
		}
	}

	return func() {
		if skipCleanup {
			log.Logf("Skipping cleanup of ingresses")
			return
		}
		for _, ingress := range ingresses {
			log.Logf("Deleting ingress %s", ingress.Name)
			err := client.NetworkingV1().Ingresses(ns).Delete(context.Background(), ingress.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Logf("Deleting ingress %s: %v", ingress.Name, err)
			}
		}
	}, nil
}

func waitForIngressAddresses(ctx context.Context, log Logger, client *kubernetes.Clientset, ns string, ingresses []*networkingv1.Ingress) (map[types.NamespacedName]net.IP, error) {
	addresses := map[types.NamespacedName]net.IP{}
	for _, ingress := range ingresses {
		log.Logf("Waiting for ingress %s to get an address", ingress.Name)
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
		if err != nil {
			return nil, fmt.Errorf("waiting for ingress %s to get an address: %w", ingress.Name, err)
		}

		nn := types.NamespacedName{Namespace: ns, Name: ingress.Name}
		addresses[nn] = net.ParseIP(address)
	}

	return addresses, nil
}
