package e2e

import (
	"context"

	"helm.sh/helm/v3/pkg/cli"
	"k8s.io/client-go/kubernetes"
)

const (
	istioVersion   = "1.24.1"
	istioChartRepo = "https://istio-release.storage.googleapis.com/charts"
)

// Installs Istio with Gateway API support using Helm. Returns a cleanup function that uninstalls
// Istio and deletes the namespace.
func deployGatewayAPIIstio(ctx context.Context,
	t TestingT,
	client *kubernetes.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) func() {
	t.Logf("Deploying Istio %s", istioVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	installChart(t, settings, istioChartRepo, "istio-base", "base", istioVersion, namespace, true, map[string]interface{}{
		"global": map[string]interface{}{
			"istioNamespace": namespace,
		},
	})
	installChart(t, settings, istioChartRepo, "istiod", "istiod", istioVersion, namespace, false, map[string]interface{}{
		"global": map[string]interface{}{
			"istioNamespace": namespace,
		},
	})

	return func() {
		if skipCleanup {
			t.Logf("Skipping cleanup of Istio")
			return
		}
		t.Logf("Cleaning up Istio")
		uninstallChart(t, settings, "istiod", namespace)
		uninstallChart(t, settings, "istio-base", namespace)

		deleteNamespaceAndWait(context.Background(), t, client, namespace)
	}
}
