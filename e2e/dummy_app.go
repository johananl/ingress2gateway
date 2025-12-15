package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const name = "dummy-app"

// TODO: Wait for app to converge.
func createDummyApp(ctx context.Context, t *testing.T, client *kubernetes.Clientset, namespace string, skipCleanup bool) func() {
	createDummyAppDeployment(ctx, t, client, namespace)
	createDummyAppService(ctx, t, client, namespace)

	return func() {
		if skipCleanup {
			t.Log("Skipping cleanup of dummy app")
			return
		}

		t.Logf("Deleting dummy app %s", name)
		err := client.CoreV1().Services(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
		if err != nil {
			t.Errorf("Deleting service %s: %v", name, err)
		}

		err = client.AppsV1().Deployments(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
		if err != nil {
			t.Errorf("Deleting deployment %s: %v", name, err)
		}
	}
}

func createDummyAppDeployment(ctx context.Context, t *testing.T, client *kubernetes.Clientset, namespace string) {
	labels := map[string]string{"app": name}

	t.Logf("Creating dummy app %s", name)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  name,
							Image: "registry.k8s.io/e2e-test-images/agnhost:2.39",
							Args:  []string{"netexec", "--http-port=8080"},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := client.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	require.NoError(t, err)
}

func createDummyAppService(ctx context.Context, t *testing.T, client *kubernetes.Clientset, namespace string) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": name},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}

	_, err := client.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{})
	require.NoError(t, err)
}
