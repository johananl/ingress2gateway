package e2e

import (
	"context"
	"fmt"
	"os"
	"time"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/kube"
)

func installChart(
	ctx context.Context,
	log Logger,
	settings *cli.EnvSettings,
	repoURL string,
	releaseName string,
	chartName string,
	version string,
	namespace string,
	createNamespace bool,
	values map[string]interface{},
) error {
	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER")); err != nil {
		return fmt.Errorf("initializing helm config: %w", err)
	}

	// Check if release already exists.
	status := action.NewStatus(cfg)
	if _, err := status.Run(releaseName); err == nil {
		log.Logf("Release %q already exists, skipping installation", releaseName)
		return nil
	}

	install := action.NewInstall(cfg)
	install.ReleaseName = releaseName
	install.Namespace = namespace
	install.CreateNamespace = createNamespace
	install.WaitStrategy = kube.StatusWatcherStrategy
	install.Timeout = 5 * time.Minute
	install.RepoURL = repoURL
	install.Version = version

	cp, err := locateChartWithRetry(ctx, log, install, chartName, settings)
	if err != nil {
		return fmt.Errorf("locating chart: %w", err)
	}

	chartRequested, err := loader.Load(cp)
	if err != nil {
		return fmt.Errorf("loading chart: %w", err)
	}

	_, err = install.Run(chartRequested, values)
	if err != nil {
		return fmt.Errorf("installing chart: %w", err)
	}

	return nil
}

func uninstallChart(ctx context.Context, log Logger, settings *cli.EnvSettings, releaseName, namespace string) error {
	cfg := new(action.Configuration)

	if err := cfg.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER")); err != nil {
		return fmt.Errorf("Initializing helm config: %w", err)
	}

	uninstall := action.NewUninstall(cfg)
	uninstall.WaitStrategy = kube.StatusWatcherStrategy
	// The default deletion propagation mode is "background", which may cause some resources to be
	// left behind for a while, which in turn may lead to "stuck" namespace deletions after
	// removing a release.
	// Relevant issue: https://github.com/helm/helm/issues/31651
	uninstall.DeletionPropagation = "foreground"
	uninstall.Timeout = 5 * time.Minute

	_, err := uninstall.Run(releaseName)
	if err != nil {
		return fmt.Errorf("Uninstalling %s: %w", releaseName, err)
	}

	return nil
}

func locateChartWithRetry(
	ctx context.Context,
	log Logger,
	install *action.Install,
	chartName string,
	settings *cli.EnvSettings,
) (string, error) {
	var cp string
	var err error
	const maxRetries = 5

	for i := range maxRetries {
		cp, err = install.ChartPathOptions.LocateChart(chartName, settings)
		if err == nil {
			return cp, nil
		}

		// Helm masks the underlying HTTP errors and status codes so we can't easily distinguish
		// transient errors (e.g. 503) from permanent errors (e.g. 404). Rather than relying on
		// fragile string parsing, we treat all errors as transient failures. This isn't a big
		// problem since the whole retry process is fairly short.

		log.Logf("Locating chart (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(2 * time.Second)
	}

	return "", err
}
