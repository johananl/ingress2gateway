package e2e

import (
	"os"
	"time"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/kube"
)

func installChart(
	t TestingT,
	settings *cli.EnvSettings,
	repoURL string,
	releaseName string,
	chartName string,
	version string,
	namespace string,
	createNamespace bool,
	values map[string]interface{},
) {
	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER")); err != nil {
		t.Fatalf("Initializing helm config: %v", err)
	}

	// Check if release already exists.
	status := action.NewStatus(cfg)
	if _, err := status.Run(releaseName); err == nil {
		t.Logf("Release %q already exists, skipping installation", releaseName)
		return
	}

	install := action.NewInstall(cfg)
	install.ReleaseName = releaseName
	install.Namespace = namespace
	install.CreateNamespace = createNamespace
	install.WaitStrategy = kube.StatusWatcherStrategy
	install.Timeout = 5 * time.Minute
	install.RepoURL = repoURL
	install.Version = version

	// TODO: Retry on failure? Random GitHub errors can make the tests fail.
	cp, err := install.ChartPathOptions.LocateChart(chartName, settings)
	require.NoError(t, err)

	chartRequested, err := loader.Load(cp)
	require.NoError(t, err)

	_, err = install.Run(chartRequested, values)
	require.NoError(t, err)
}

func uninstallChart(t TestingT, settings *cli.EnvSettings, releaseName, namespace string) {
	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER")); err != nil {
		t.Errorf("Initializing helm config: %v", err)
		return
	}

	uninstall := action.NewUninstall(cfg)
	uninstall.WaitStrategy = kube.StatusWatcherStrategy
	uninstall.Timeout = 5 * time.Minute

	_, err := uninstall.Run(releaseName)
	if err != nil {
		t.Errorf("Uninstalling %s: %v", releaseName, err)
	}
}
