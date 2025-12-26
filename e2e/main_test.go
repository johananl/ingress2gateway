package e2e

import (
	"context"
	"log"
	"os"
	"testing"
)

// StdLogger implements TestingT using the standard library's log package. It's intended to be used
// in TestMain.
type StdLogger struct{}

func (l *StdLogger) Logf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func (l *StdLogger) Fatalf(format string, args ...interface{}) {
	log.Fatalf(format, args...)
}

func (l *StdLogger) Errorf(format string, args ...interface{}) {
	log.Printf("ERROR: "+format, args...)
}

func (l *StdLogger) FailNow() {
	log.Fatal("FAIL NOW")
}

func (l *StdLogger) Helper() {}

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
		cleanupMetalLB, err = deployMetalLB(ctx, logger, k8sClient, dynamicClient, kubeconfig, skipCleanup)
		if err != nil {
			logger.Fatalf("Deploying MetalLB: %v", err)
		}
	}

	cleanupCRDs, err := deployCRDs(ctx, logger, apiextensionsClient, skipCleanup)
	if err != nil {
		logger.Fatalf("Deploying Gateway API CRDs: %v", err)
	}

	// Run test cases.
	exitCode := m.Run()

	// We can't defer because we're calling os.Exit().
	cleanupCRDs()
	cleanupMetalLB()

	os.Exit(exitCode)
}
