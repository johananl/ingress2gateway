package e2e

import (
	"context"
	"log"
	"os"
	"testing"
)

// TestMain wraps all test cases and performs global setup and cleanup operations. Any code before
// m.Run() is setup code and any code after m.Run() is cleanup code.
func TestMain(m *testing.M) {
	ctx := context.Background()
	logger := LoggerFunc(log.Printf)

	skipCleanup := os.Getenv("SKIP_CLEANUP") == "1"

	// We deliberately avoid setting a default kubeconfig so that we don't accidentally create e2e
	// resources on a production cluster.
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		log.Fatalf("Environment variable KUBECONFIG must be set")
	}

	apiextensionsClient, err := NewAPIExtensionsClientFromKubeconfigPath(kubeconfig)
	if err != nil {
		log.Fatalf("Creating API extensions client: %v", err)
	}

	cleanupCRDs, err := deployCRDs(ctx, logger, apiextensionsClient, skipCleanup)
	if err != nil {
		log.Fatalf("Deploying Gateway API CRDs: %v", err)
	}

	// Run test cases.
	exitCode := m.Run()

	// We can't defer because we're calling os.Exit().
	cleanupCRDs()

	os.Exit(exitCode)
}
