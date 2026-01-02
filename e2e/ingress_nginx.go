package e2e

import (
	"context"
	"fmt"
	"time"

	"helm.sh/helm/v4/pkg/cli"
	"k8s.io/client-go/kubernetes"
)

const (
	ingressNginxChartVersion = "4.11.0"
	ingressNginxChartRepo    = "https://kubernetes.github.io/ingress-nginx"
)

func deployIngressNginx(
	ctx context.Context,
	log Logger,
	client *kubernetes.Clientset,
	kubeconfigPath string,
	namespace string,
	skipCleanup bool,
) (func(), error) {
	log.Logf("Deploying ingress-nginx chart %s", ingressNginxChartVersion)

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	if err := installChart(
		ctx,
		log,
		settings,
		ingressNginxChartRepo,
		"ingress-nginx",
		"ingress-nginx",
		ingressNginxChartVersion,
		namespace,
		true,
		nil,
	); err != nil {
		return nil, fmt.Errorf("installing chart: %w", err)
	}

	return func() {
		if skipCleanup {
			log.Logf("Skipping cleanup of ingress-nginx")
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Logf("Cleaning up ingress-nginx")
		if err := uninstallChart(cleanupCtx, log, settings, "ingress-nginx", namespace); err != nil {
			log.Logf("Uninstalling chart: %v", err)
		}

		if err := deleteNamespaceAndWait(cleanupCtx, log, client, namespace); err != nil {
			log.Logf("Deleting namespace: %v", err)
		}
	}, nil
}
