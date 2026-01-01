package e2e

import (
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestIngressNginx(t *testing.T) {
	t.Run("ToIstio", func(t *testing.T) {
		t.Run("basic conversion", func(t *testing.T) {
			runTestCase(t, &TestCase{
				// TODO: Should this be called "emitter"? Do we need more than one?
				GatewayImplementation: "istio",
				Providers:             []string{"ingress-nginx"},
				ProviderFlags: map[string]map[string]string{
					"ingress-nginx": {
						"ingress-class": "nginx",
					},
				},
				Ingresses: []*networkingv1.Ingress{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-ingress1",
						},
						Spec: networkingv1.IngressSpec{
							IngressClassName: ptr.To("nginx"),
							Rules: []networkingv1.IngressRule{
								{
									Host: "basic.example.com",
									IngressRuleValue: networkingv1.IngressRuleValue{
										HTTP: &networkingv1.HTTPIngressRuleValue{
											Paths: []networkingv1.HTTPIngressPath{
												{
													Path:     "/",
													PathType: ptr.To(networkingv1.PathTypePrefix),
													Backend: networkingv1.IngressBackend{
														Service: &networkingv1.IngressServiceBackend{
															Name: "dummy-app",
															Port: networkingv1.ServiceBackendPort{
																Number: 80,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Verifiers: map[string][]Verifier{
					"test-ingress1": {
						&HTTPGetVerifier{
							Host: "basic.example.com",
							Path: "/",
						},
					},
				},
			})
		})
	})
}
