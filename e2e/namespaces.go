package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func createNamespace(ctx context.Context, t *testing.T, client *kubernetes.Clientset, ns string, skipCleanup bool) func() {
	// Check if namespace already exists. This should be very rare since we use a random suffix,
	// but we check just in case to avoid flaky tests do to conflicts.
	_, err := client.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err == nil {
		t.Fatalf("Namespace %s already exists", ns)
	}

	t.Logf("Creating namespace %s", ns)
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	return func() {
		if skipCleanup {
			t.Logf("Skipping cleanup of namespace %s", ns)
			return
		}
		t.Logf("Cleaning up namespace %s", ns)
		deleteNamespaceAndWait(context.Background(), t, client, ns)
	}
}

func deleteNamespaceAndWait(ctx context.Context, t TestingT, client *kubernetes.Clientset, ns string) {
	err := client.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{})
	if err != nil {
		t.Logf("Deleting namespace %s: %v", ns, err)
		return
	}

	err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := client.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Errorf("Waiting for namespace %s to delete: %v", ns, err)
	}
}
