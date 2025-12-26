package e2e

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func createNamespace(ctx context.Context, log Logger, client *kubernetes.Clientset, ns string, skipCleanup bool) (func(), error) {
	// Check if namespace already exists. This should be very rare since we use a random suffix,
	// but we check just in case to avoid flaky tests do to conflicts.
	_, err := client.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err == nil {
		return nil, fmt.Errorf("namespace %s already exists", ns)
	}

	log.Logf("Creating namespace %s", ns)
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating namespace %s: %w", ns, err)
	}

	return func() {
		if skipCleanup {
			log.Logf("Skipping cleanup of namespace %s", ns)
			return
		}
		log.Logf("Cleaning up namespace %s", ns)
		if err := deleteNamespaceAndWait(context.Background(), log, client, ns); err != nil {
			log.Logf("Deleting namespace %s: %v", ns, err)
		}
	}, nil
}

func deleteNamespaceAndWait(ctx context.Context, log Logger, client *kubernetes.Clientset, ns string) error {
	if err := client.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("deleting namespace %s: %w", ns, err)
	}

	if err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := client.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}); err != nil {
		return fmt.Errorf("waiting for namespace %s to delete: %w", ns, err)
	}

	return nil
}
