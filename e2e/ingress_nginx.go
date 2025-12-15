package e2e

import (
	"context"

	"helm.sh/helm/v4/pkg/cli"
	"k8s.io/client-go/kubernetes"
)

const (
	ingressNginxChartVersion = "4.11.0"
	ingressNginxChartRepo    = "https://kubernetes.github.io/ingress-nginx"
)

func deployIngressNginx(
	ctx context.Context,
	t TestingT,
	client *kubernetes.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) func() {
	t.Logf("Deploying ingress-nginx chart %s", ingressNginxChartVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	installChart(t, settings, ingressNginxChartRepo, "ingress-nginx", "ingress-nginx", ingressNginxChartVersion, namespace, true, nil)

	return func() {
		if skipCleanup {
			t.Logf("Skipping cleanup of ingress-nginx")
			return
		}
		t.Logf("Cleaning up ingress-nginx")
		uninstallChart(t, settings, "ingress-nginx", namespace)

		deleteNamespaceAndWait(context.Background(), t, client, namespace)
	}
}
