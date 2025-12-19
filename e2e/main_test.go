package e2e

import (
	"context"
	"os"
	"testing"
)

// TestMain wraps all test cases and performs global setup and cleanup operations. Any code before
// m.Run() is setup code and any code after m.Run() is cleanup code.
func TestMain(m *testing.M) {
	ctx := context.Background()
	logger := &StdLogger{}

	skipCleanup := os.Getenv("SKIP_CLEANUP") == "1"
	shouldDeployMetalLB := os.Getenv("DEPLOY_METALLB") == "1"

	// We deliberately avoid setting a default kubeconfig so that we don't accidentally create e2e
	// resources on a production cluster.
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		logger.Fatalf("Environment variable KUBECONFIG must be set")
	}

	k8sClient, err := NewClientFromKubeconfigPath(kubeconfig)
	if err != nil {
		logger.Fatalf("Creating k8s client: %v", err)
	}

	apiextensionsClient, err := NewAPIExtensionsClientFromKubeconfigPath(kubeconfig)
	if err != nil {
		logger.Fatalf("Creating API extensions client: %v", err)
	}

	dynamicClient, err := NewDynamicClientFromKubeconfigPath(kubeconfig)
	if err != nil {
		logger.Fatalf("Creating dynamic client: %v", err)
	}

	cleanupMetalLB := func() {} // Default to a no-op function
	if shouldDeployMetalLB {
		cleanupMetalLB = deployMetalLB(ctx, logger, k8sClient, dynamicClient, kubeconfig, skipCleanup)
	}

	logger.Logf("Deploying Gateway API CRDs")
	cleanupCRDs := deployCRDs(ctx, logger, apiextensionsClient, skipCleanup)

	// Run test cases.
	exitCode := m.Run()

	// We can't defer because we're calling os.Exit().
	cleanupCRDs()
	cleanupMetalLB()

	os.Exit(exitCode)
}
