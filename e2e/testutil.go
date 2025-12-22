package e2e

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
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

	seed := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate a random prefix to ensure unique namespaces for each test case.
	prefix := fmt.Sprintf("%s-%s", e2ePrefix, randString(5, seed))

	appNS := fmt.Sprintf("%s-app", prefix)
	cleanupNS := createNamespace(ctx, t, k8sClient, appNS, skipCleanup)
	t.Cleanup(cleanupNS)

	// Deploy ingress providers.
	for _, p := range tc.Providers {
		var key string
		var installFunc func() func()

		switch p {
		case "ingress-nginx":
			key = "ingress-nginx"
			ns := fmt.Sprintf("%s-ingress-nginx", e2ePrefix)
			installFunc = func() func() {
				logger := &StdLogger{}
				return deployIngressNginx(ctx, logger, k8sClient, kubeconfig, ns, skipCleanup)
			}
		default:
			t.Fatalf("Unknown ingress provider: %s", p)
		}
		release := globalResourceManager.Acquire(key, installFunc)
		t.Cleanup(release)
	}

	// Deploy a GWAPI implementation.
	if tc.GatewayImplementation != "" {
		var key string
		var installFunc func() func()

		switch tc.GatewayImplementation {
		case "istio":
			key = "istio"
			ns := fmt.Sprintf("%s-istio-system", e2ePrefix)
			installFunc = func() func() {
				logger := &StdLogger{}
				return deployGatewayAPIIstio(ctx, logger, k8sClient, kubeconfig, ns, skipCleanup)
			}
		default:
			t.Fatalf("Unknown Gateway Implementation: %s", tc.GatewayImplementation)
		}
		release := globalResourceManager.Acquire(key, installFunc)
		t.Cleanup(release)
	}

	// Deploy a dummy app.
	cleanupDummyApp := createDummyApp(ctx, t, k8sClient, appNS, skipCleanup)
	t.Cleanup(cleanupDummyApp)

	// Create ingress resources.
	cleanupIngresses := createIngresses(ctx, t, k8sClient, appNS, tc.Ingresses, skipCleanup)
	t.Cleanup(cleanupIngresses)

	// Wait for ingresses to get IP addresses.
	addresses := waitForIngressAddresses(ctx, t, k8sClient, appNS, tc.Ingresses)
	t.Logf("Got ingress addresses: %v", addresses)

	// Run verifiers against ingresses.
	verifyIngresses(ctx, t, tc, appNS, addresses)

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
	cleanupGatewayResources := createGatewayResources(ctx, t, gwClient, appNS, res, skipCleanup)
	t.Cleanup(cleanupGatewayResources)

	// Wait for gateways to get IP addresses.
	gwAddresses := waitForGatewayAddresses(ctx, t, gwClient, appNS, getGateways(res))
	t.Logf("Got gateway addresses: %v", gwAddresses)

	// Run verifier against GWAPI implementation.
	verifyGatewayResources(ctx, t, tc, appNS, gwAddresses)
}

func verifyIngresses(ctx context.Context, t *testing.T, tc *TestCase, namespace string, addresses map[types.NamespacedName]net.IP) {
	for ingressName, verifiers := range tc.Verifiers {
		nn := types.NamespacedName{Namespace: namespace, Name: ingressName}
		ip, ok := addresses[nn]
		if !ok {
			t.Fatalf("Ingress %s not found in addresses", ingressName)
		}

		for _, v := range verifiers {
			err := v.Verify(ctx, t, ip)
			if err != nil {
				t.Fatalf("Ingress verification failed: %v", err)
			}
		}
	}
}

func verifyGatewayResources(ctx context.Context, t *testing.T, tc *TestCase, namespace string, gwAddresses map[types.NamespacedName]net.IP) {
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
		ip, ok := gwAddresses[gwName]
		if !ok {
			t.Fatalf("Gateway %s not found in addresses", gwName)
		}

		for _, v := range verifiers {
			err := v.Verify(ctx, t, ip)
			if err != nil {
				t.Fatalf("Gateway API verification failed: %v", err)
			}
		}
	}
}

// TestingT is a subset of testing.T that we use in our helpers.
// It allows us to pass *testing.T or a custom logger (for TestMain).
type TestingT interface {
	Logf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	FailNow()
	Helper()
}

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
