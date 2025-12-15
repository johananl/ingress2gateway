package e2e

import (
	"os"
	"time"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
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
	if err := cfg.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		t.Logf(format, v...)
	}); err != nil {
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
	install.Wait = true
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
	if err := cfg.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		t.Logf(format, v...)
	}); err != nil {
		t.Errorf("Initializing helm config for uninstall: %v", err)
		return
	}

	uninstall := action.NewUninstall(cfg)
	uninstall.Wait = true
	uninstall.Timeout = 5 * time.Minute

	_, err := uninstall.Run(releaseName)
	if err != nil {
		t.Errorf("Uninstalling %s: %v", releaseName, err)
	}
}
