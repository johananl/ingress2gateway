package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/common"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	// Import all provider implementations to register them.
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/apisix"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/cilium"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/gce"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/ingressnginx"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/istio"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/kong"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/nginx"
	_ "github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/providers/openapi3"
)

// A prefix for all namespaces used in the e2e tests.
const e2ePrefix = "i2gwe2e"

type TestCase struct {
	Ingresses             []*networkingv1.Ingress
	Providers             []string
	ProviderFlags         map[string]map[string]string
	GatewayImplementation string
	Verifiers             map[string][]Verifier
}

func runTestCase(t *testing.T, tc *TestCase) {
	t.Parallel()

	ctx := t.Context()

	// We deliberately avoid setting a default kubeconfig so that we don't accidentally create e2e
	// resources on a production cluster.
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		t.Fatal("Environment variable KUBECONFIG must be set")
	}

	skipCleanup := os.Getenv("SKIP_CLEANUP") == "1"

	k8sClient, err := NewClientFromKubeconfigPath(kubeconfig)
	require.NoError(t, err)

	gwClient, err := NewGatewayClientFromKubeconfigPath(kubeconfig)
	require.NoError(t, err)

	apiextensionsClient, err := NewAPIExtensionsClientFromKubeconfigPath(kubeconfig)
	require.NoError(t, err)

	restConfig, err := NewRestConfigFromKubeconfigPath(kubeconfig)
	require.NoError(t, err)

	seed := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate a random prefix to ensure unique namespaces for each test case.
	prefix := fmt.Sprintf("%s-%s", e2ePrefix, randString(5, seed))

	appNS := fmt.Sprintf("%s-app", prefix)
	cleanupNS, err := createNamespace(ctx, t, k8sClient, appNS, skipCleanup)
	require.NoError(t, err)
	t.Cleanup(cleanupNS)

	// Deploy ingress providers and GWAPI implementation asynchronously.
	var resources []Resource

	// Deploy Gateway API CRDs.
	crdResource := globalResourceManager.Acquire("gateway-api-crds", func() (CleanupFunc, error) {
		return deployCRDs(ctx, t, apiextensionsClient, skipCleanup)
	})
	resources = append(resources, crdResource)

	for _, p := range tc.Providers {
		var r Resource
		switch p {
		case "ingress-nginx":
			ns := fmt.Sprintf("%s-ingress-nginx", e2ePrefix)
			r = globalResourceManager.Acquire("ingress-nginx", func() (CleanupFunc, error) {
				return deployIngressNginx(ctx, t, k8sClient, kubeconfig, ns, skipCleanup)
			})
		default:
			t.Fatalf("Unknown ingress provider: %s", p)
		}
		resources = append(resources, r)
	}

	if tc.GatewayImplementation != "" {
		var r Resource
		switch tc.GatewayImplementation {
		case "istio":
			ns := fmt.Sprintf("%s-istio-system", e2ePrefix)
			r = globalResourceManager.Acquire("istio", func() (CleanupFunc, error) {
				return deployGatewayAPIIstio(ctx, t, k8sClient, kubeconfig, ns, skipCleanup)
			})
		default:
			t.Fatalf("Unknown gateway implementation: %s", tc.GatewayImplementation)
		}
		resources = append(resources, r)
	}

	// Register a single cleanup that runs all cleanups in parallel.
	t.Cleanup(func() {
		var doneChans []<-chan struct{}
		for _, r := range resources {
			doneChans = append(doneChans, r.Cleanup())
		}
		for _, ch := range doneChans {
			<-ch
		}
	})

	for _, r := range resources {
		if err := r.Wait(); err != nil {
			t.Fatalf("Resource installation failed: %v", err)
		}
	}

	// Deploy a dummy app.
	cleanupDummyApp, err := createDummyApp(ctx, t, k8sClient, appNS, skipCleanup)
	if err != nil {
		t.Fatalf("Creating dummy app: %v", err)
	}
	t.Cleanup(cleanupDummyApp)

	// Create ingress resources.
	// TODO: Prefix ingresses to improve isolation?
	cleanupIngresses, err := createIngresses(ctx, t, k8sClient, appNS, tc.Ingresses, skipCleanup)
	require.NoError(t, err)
	t.Cleanup(cleanupIngresses)

	// Set up port-forwarding to the ingress controller for verification.
	// We need to find the ingress controller service and forward to it.
	var ingressPortForwarders []*PortForwarder
	ingressAddresses := make(map[types.NamespacedName]string)

	for _, p := range tc.Providers {
		switch p {
		case "ingress-nginx":
			ingressNS := fmt.Sprintf("%s-ingress-nginx", e2ePrefix)
			svc, err := FindIngressControllerService(ctx, k8sClient, ingressNS, "ingress-nginx")
			require.NoError(t, err, "finding ingress-nginx service")

			pf, addr, err := StartPortForwardToService(ctx, k8sClient, restConfig, svc.Namespace, svc.Name, 80)
			require.NoError(t, err, "starting port-forward to ingress-nginx")
			ingressPortForwarders = append(ingressPortForwarders, pf)

			// All ingresses handled by this controller share the same address.
			for _, ing := range tc.Ingresses {
				nn := types.NamespacedName{Namespace: appNS, Name: ing.Name}
				ingressAddresses[nn] = addr
			}
			t.Logf("Port-forwarding ingress controller %s via %s", p, addr)
		default:
			t.Fatalf("Unknown ingress provider: %s", p)
		}
	}

	// Register cleanup for ingress port-forwarders (runs before other cleanups due to LIFO).
	t.Cleanup(func() {
		for _, pf := range ingressPortForwarders {
			pf.Stop()
		}
	})

	// Run verifiers against ingresses.
	verifyIngresses(ctx, t, tc, appNS, ingressAddresses)

	// Call ingress2gateway.
	// Pass an empty input file to make i2gw read ingresses from the cluster.
	res, notif, err := i2gw.ToGatewayAPIResources(ctx, appNS, "", tc.Providers, tc.ProviderFlags)
	require.NoError(t, err)

	if len(notif) > 0 {
		t.Log("Received notifications during conversion:")
		for _, table := range notif {
			t.Log(table)
		}
	}

	// TODO: Hack! Force correct gateway class since i2gw doesn't seem to infer that from the
	// ingress at the moment.
	for _, r := range res {
		for k, v := range r.Gateways {
			v.Spec.GatewayClassName = gwapiv1.ObjectName(tc.GatewayImplementation)
			r.Gateways[k] = v
		}
	}

	// Create Gateway API resources.
	cleanupGatewayResources, err := createGatewayResources(ctx, t, gwClient, appNS, res, skipCleanup)
	if err != nil {
		t.Fatalf("Creating gateway resources: %v", err)
	}
	t.Cleanup(cleanupGatewayResources)

	// Set up port-forwarding to each gateway for verification.
	var gatewayPortForwarders []*PortForwarder
	gwAddresses := make(map[types.NamespacedName]string)

	for gwName, gw := range getGateways(res) {
		ns := gw.Namespace
		if ns == "" {
			ns = appNS
		}

		// Find the service created by the Gateway controller.
		svc, err := FindGatewayService(ctx, t, k8sClient, ns, gw.Name)
		if err != nil {
			t.Fatalf("Finding gateway service for %s: %v", gwName, err)
		}

		// Wait for at least one pod to be ready before port-forwarding.
		t.Logf("Waiting for gateway %s service %s/%s to have ready pods", gwName, svc.Namespace, svc.Name)
		if err := WaitForServiceReady(ctx, k8sClient, svc.Namespace, svc.Name); err != nil {
			t.Fatalf("Waiting for gateway service %s/%s to be ready: %v", svc.Namespace, svc.Name, err)
		}

		// Start port-forward to the gateway service.
		pf, addr, err := StartPortForwardToService(ctx, k8sClient, restConfig, svc.Namespace, svc.Name, 80)
		if err != nil {
			t.Fatalf("Starting port-forward for gateway %s: %v", gwName, err)
		}

		gatewayPortForwarders = append(gatewayPortForwarders, pf)
		gwAddresses[gwName] = addr
		t.Logf("Port-forwarding gateway %s via %s", gwName, addr)
	}

	// Register cleanup for gateway port-forwarders.
	t.Cleanup(func() {
		for _, pf := range gatewayPortForwarders {
			pf.Stop()
		}
	})

	// Run verifier against GWAPI implementation.
	verifyGatewayResources(ctx, t, tc, appNS, gwAddresses)
}

func verifyIngresses(ctx context.Context, t *testing.T, tc *TestCase, namespace string, addresses map[types.NamespacedName]string) {
	for ingressName, verifiers := range tc.Verifiers {
		nn := types.NamespacedName{Namespace: namespace, Name: ingressName}
		addr, ok := addresses[nn]
		if !ok {
			t.Fatalf("Ingress %s not found in addresses", ingressName)
		}

		for _, v := range verifiers {
			err := v.Verify(ctx, t, addr)
			if err != nil {
				t.Fatalf("Ingress verification failed: %v", err)
			}
		}
	}
}

func verifyGatewayResources(ctx context.Context, t *testing.T, tc *TestCase, namespace string, gwAddresses map[types.NamespacedName]string) {
	for ingressName, verifiers := range tc.Verifiers {
		// Find the ingress to determine the expected Gateway name.
		var ingress *networkingv1.Ingress
		for _, ing := range tc.Ingresses {
			if ing.Name == ingressName {
				ingress = ing
				break
			}
		}
		if ingress == nil {
			t.Fatalf("Ingress %s not found in test case", ingressName)
		}

		ingressClass := common.GetIngressClass(*ingress)
		if ingressClass == "" {
			t.Fatalf("Ingress %s has no ingress class", ingressName)
		}

		gwName := types.NamespacedName{Namespace: namespace, Name: ingressClass}
		addr, ok := gwAddresses[gwName]
		if !ok {
			t.Fatalf("Gateway %s not found in addresses", gwName)
		}

		for _, v := range verifiers {
			err := v.Verify(ctx, t, addr)
			if err != nil {
				t.Fatalf("Gateway API verification failed: %v", err)
			}
		}
	}
}
