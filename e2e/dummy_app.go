package e2e

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const name = "dummy-app"

func createDummyApp(ctx context.Context, log Logger, client *kubernetes.Clientset, namespace string, skipCleanup bool) (func(), error) {
	if err := createDummyAppDeployment(ctx, log, client, namespace); err != nil {
		return nil, fmt.Errorf("creating deployment: %w", err)
	}

	if err := createDummyAppService(ctx, log, client, namespace); err != nil {
		return nil, fmt.Errorf("creating service: %w", err)
	}

	if err := waitForDummyApp(ctx, log, client, namespace); err != nil {
		return nil, fmt.Errorf("waiting for dummy app: %w", err)
	}

	return func() {
		if skipCleanup {
			log.Logf("Skipping cleanup of dummy app")
			return
		}

		log.Logf("Deleting dummy app %s", name)
		err := client.CoreV1().Services(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
		if err != nil {
			log.Logf("Deleting service %s: %v", name, err)
		}

		err = client.AppsV1().Deployments(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
		if err != nil {
			log.Logf("Deleting deployment %s: %v", name, err)
		}
	}, nil
}

func createDummyAppDeployment(ctx context.Context, log Logger, client *kubernetes.Clientset, namespace string) error {
	labels := map[string]string{"app": name}

	log.Logf("Creating dummy app %s", name)

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

	if _, err := client.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("creating deployment: %w", err)
	}

	return nil
}

func createDummyAppService(ctx context.Context, log Logger, client *kubernetes.Clientset, namespace string) error {
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

	if _, err := client.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	return nil
}

func waitForDummyApp(ctx context.Context, log Logger, client *kubernetes.Clientset, namespace string) error {
	log.Logf("Waiting for dummy app to be ready")
	err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		dep, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, cond := range dep.Status.Conditions {
			if cond.Type == appsv1.DeploymentAvailable && cond.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("waiting for deployment: %w", err)
	}

	return nil
}
