package e2e

import (
	"context"
	"fmt"

	"helm.sh/helm/v4/pkg/cli"
	"k8s.io/client-go/kubernetes"
)

const (
	istioVersion   = "1.24.1"
	istioChartRepo = "https://istio-release.storage.googleapis.com/charts"
)

// Installs Istio with Gateway API support using Helm. Returns a cleanup function that uninstalls
// Istio and deletes the namespace.
func deployGatewayAPIIstio(ctx context.Context,
	log Logger,
	client *kubernetes.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) (func(), error) {
	log.Logf("Deploying Istio %s", istioVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	values := map[string]interface{}{
		"global": map[string]interface{}{
			"istioNamespace": namespace,
		},
	}

	if err := installChart(
		ctx,
		log,
		settings,
		istioChartRepo,
		"istio-base",
		"base",
		istioVersion,
		namespace,
		true,
		values,
	); err != nil {
		return nil, fmt.Errorf("installing chart %s: %w", "istio-base", err)
	}

	values = map[string]interface{}{
		"global": map[string]interface{}{
			"istioNamespace": namespace,
		},
	}

	if err := installChart(
		ctx,
		log,
		settings,
		istioChartRepo,
		"istiod",
		"istiod",
		istioVersion,
		namespace,
		false,
		values,
	); err != nil {
		return nil, fmt.Errorf("installing chart %s: %w", "istiod", err)
	}

	return func() {
		if skipCleanup {
			log.Logf("Skipping cleanup of Istio")
			return
		}
		log.Logf("Cleaning up Istio")
		if err := uninstallChart(context.Background(), log, settings, "istiod", namespace); err != nil {
			log.Logf("Uninstalling chart %s: %v", "istiod", err)
		}
		if err := uninstallChart(context.Background(), log, settings, "istio-base", namespace); err != nil {
			log.Logf("Uninstalling chart %s: %v", "istio-base", err)
		}

		if err := deleteNamespaceAndWait(context.Background(), log, client, namespace); err != nil {
			log.Logf("Deleting namespace: %v", err)
		}
	}, nil
}
