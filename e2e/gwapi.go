package e2e

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
	gwclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

func createGatewayResources(ctx context.Context, t *testing.T, client *gwclientset.Clientset, ns string, res []i2gw.GatewayResources, skipCleanup bool) func() {
	var cleanupFuncs []func()
	for _, r := range res {
		cleanupFuncs = append(cleanupFuncs, createGatewayClasses(ctx, t, client, r.GatewayClasses, skipCleanup))
		cleanupFuncs = append(cleanupFuncs, createGateways(ctx, t, client, ns, r.Gateways, skipCleanup))
		cleanupFuncs = append(cleanupFuncs, createHTTPRoutes(ctx, t, client, ns, r.HTTPRoutes, skipCleanup))
		cleanupFuncs = append(cleanupFuncs, createGRPCRoutes(ctx, t, client, ns, r.GRPCRoutes, skipCleanup))
		cleanupFuncs = append(cleanupFuncs, createTLSRoutes(ctx, t, client, ns, r.TLSRoutes, skipCleanup))
		cleanupFuncs = append(cleanupFuncs, createTCPRoutes(ctx, t, client, ns, r.TCPRoutes, skipCleanup))
		cleanupFuncs = append(cleanupFuncs, createUDPRoutes(ctx, t, client, ns, r.UDPRoutes, skipCleanup))
		cleanupFuncs = append(cleanupFuncs, createBackendTLSPolicies(ctx, t, client, ns, r.BackendTLSPolicies, skipCleanup))
		cleanupFuncs = append(cleanupFuncs, createReferenceGrants(ctx, t, client, ns, r.ReferenceGrants, skipCleanup))
	}

	return func() {
		if skipCleanup {
			t.Log("Skipping cleanup of gateway resources")
			return
		}
		for _, f := range cleanupFuncs {
			f()
		}
	}
}

func createGateways(ctx context.Context, t *testing.T, client *gwclientset.Clientset, ns string, gws map[types.NamespacedName]v1.Gateway, skipCleanup bool) func() {
	for name, gw := range gws {
		// Ensure the namespace is set correctly.
		if gw.Namespace == "" {
			gw.Namespace = ns
		}

		y, err := toYAML(&gw)
		require.NoError(t, err)
		t.Logf("Creating Gateway:\n%s", y)

		_, err = client.GatewayV1().Gateways(gw.Namespace).Create(
			ctx,
			&gw,
			metav1.CreateOptions{},
		)
		require.NoError(t, err, "failed to create Gateway %s", name.String())
	}

	return func() {
		if skipCleanup {
			return
		}
		for _, gw := range gws {
			namespace := gw.Namespace
			if namespace == "" {
				namespace = ns
			}
			t.Logf("Deleting Gateway %s/%s", namespace, gw.Name)
			err := client.GatewayV1().Gateways(namespace).Delete(context.Background(), gw.Name, metav1.DeleteOptions{})
			if err != nil {
				t.Errorf("Deleting Gateway %s: %v", gw.Name, err)
			}
		}
	}
}

func createGatewayClasses(ctx context.Context, t *testing.T, client *gwclientset.Clientset, gcs map[types.NamespacedName]v1.GatewayClass, skipCleanup bool) func() {
	for name, gc := range gcs {
		y, err := toYAML(&gc)
		require.NoError(t, err)
		t.Logf("Creating GatewayClass:\n%s", y)

		_, err = client.GatewayV1().GatewayClasses().Create(
			ctx,
			&gc,
			metav1.CreateOptions{},
		)
		require.NoError(t, err, "failed to create GatewayClass %s", name.String())
	}

	return func() {
		if skipCleanup {
			return
		}
		for _, gc := range gcs {
			t.Logf("Deleting GatewayClass %s", gc.Name)
			err := client.GatewayV1().GatewayClasses().Delete(context.Background(), gc.Name, metav1.DeleteOptions{})
			if err != nil {
				t.Errorf("Deleting GatewayClass %s: %v", gc.Name, err)
			}
		}
	}
}

func createHTTPRoutes(ctx context.Context, t *testing.T, client *gwclientset.Clientset, ns string, routes map[types.NamespacedName]v1.HTTPRoute, skipCleanup bool) func() {
	for name, route := range routes {
		// Ensure the namespace is set correctly.
		if route.Namespace == "" {
			route.Namespace = ns
		}

		y, err := toYAML(&route)
		require.NoError(t, err)
		t.Logf("Creating HTTPRoute:\n%s", y)

		_, err = client.GatewayV1().HTTPRoutes(route.Namespace).Create(
			ctx,
			&route,
			metav1.CreateOptions{},
		)
		require.NoError(t, err, "failed to create HTTPRoute %s", name.String())
	}

	return func() {
		if skipCleanup {
			return
		}
		for _, route := range routes {
			namespace := route.Namespace
			if namespace == "" {
				namespace = ns
			}
			t.Logf("Deleting HTTPRoute %s/%s", namespace, route.Name)
			err := client.GatewayV1().HTTPRoutes(namespace).Delete(context.Background(), route.Name, metav1.DeleteOptions{})
			if err != nil {
				t.Errorf("Deleting HTTPRoute %s: %v", route.Name, err)
			}
		}
	}
}

func createGRPCRoutes(ctx context.Context, t *testing.T, client *gwclientset.Clientset, ns string, routes map[types.NamespacedName]v1.GRPCRoute, skipCleanup bool) func() {
	for name, route := range routes {
		// Ensure the namespace is set correctly.
		if route.Namespace == "" {
			route.Namespace = ns
		}

		y, err := toYAML(&route)
		require.NoError(t, err)
		t.Logf("Creating GRPCRoute:\n%s", y)

		_, err = client.GatewayV1().GRPCRoutes(route.Namespace).Create(
			ctx,
			&route,
			metav1.CreateOptions{},
		)
		require.NoError(t, err, "failed to create GRPCRoute %s", name.String())
	}

	return func() {
		if skipCleanup {
			return
		}
		for _, route := range routes {
			namespace := route.Namespace
			if namespace == "" {
				namespace = ns
			}
			t.Logf("Deleting GRPCRoute %s/%s", namespace, route.Name)
			err := client.GatewayV1().GRPCRoutes(namespace).Delete(context.Background(), route.Name, metav1.DeleteOptions{})
			if err != nil {
				t.Errorf("Deleting GRPCRoute %s: %v", route.Name, err)
			}
		}
	}
}

func createTLSRoutes(ctx context.Context, t *testing.T, client *gwclientset.Clientset, ns string, routes map[types.NamespacedName]v1alpha2.TLSRoute, skipCleanup bool) func() {
	for name, route := range routes {
		// Ensure the namespace is set correctly.
		if route.Namespace == "" {
			route.Namespace = ns
		}

		y, err := toYAML(&route)
		require.NoError(t, err)
		t.Logf("Creating TLSRoute:\n%s", y)

		_, err = client.GatewayV1alpha2().TLSRoutes(route.Namespace).Create(
			ctx,
			&route,
			metav1.CreateOptions{},
		)
		require.NoError(t, err, "failed to create TLSRoute %s", name.String())
	}

	return func() {
		if skipCleanup {
			return
		}
		for _, route := range routes {
			namespace := route.Namespace
			if namespace == "" {
				namespace = ns
			}
			t.Logf("Deleting TLSRoute %s/%s", namespace, route.Name)
			err := client.GatewayV1alpha2().TLSRoutes(namespace).Delete(context.Background(), route.Name, metav1.DeleteOptions{})
			if err != nil {
				t.Errorf("Deleting TLSRoute %s: %v", route.Name, err)
			}
		}
	}
}

func createTCPRoutes(ctx context.Context, t *testing.T, client *gwclientset.Clientset, ns string, routes map[types.NamespacedName]v1alpha2.TCPRoute, skipCleanup bool) func() {
	for name, route := range routes {
		// Ensure the namespace is set correctly.
		if route.Namespace == "" {
			route.Namespace = ns
		}

		y, err := toYAML(&route)
		require.NoError(t, err)
		t.Logf("Creating TCPRoute:\n%s", y)

		_, err = client.GatewayV1alpha2().TCPRoutes(route.Namespace).Create(
			ctx,
			&route,
			metav1.CreateOptions{},
		)
		require.NoError(t, err, "failed to create TCPRoute %s", name.String())
	}

	return func() {
		if skipCleanup {
			return
		}
		for _, route := range routes {
			namespace := route.Namespace
			if namespace == "" {
				namespace = ns
			}
			t.Logf("Deleting TCPRoute %s/%s", namespace, route.Name)
			err := client.GatewayV1alpha2().TCPRoutes(namespace).Delete(context.Background(), route.Name, metav1.DeleteOptions{})
			if err != nil {
				t.Errorf("Deleting TCPRoute %s: %v", route.Name, err)
			}
		}
	}
}

func createUDPRoutes(ctx context.Context, t *testing.T, client *gwclientset.Clientset, ns string, routes map[types.NamespacedName]v1alpha2.UDPRoute, skipCleanup bool) func() {
	for name, route := range routes {
		// Ensure the namespace is set correctly.
		if route.Namespace == "" {
			route.Namespace = ns
		}

		y, err := toYAML(&route)
		require.NoError(t, err)
		t.Logf("Creating UDPRoute:\n%s", y)

		_, err = client.GatewayV1alpha2().UDPRoutes(route.Namespace).Create(
			ctx,
			&route,
			metav1.CreateOptions{},
		)
		require.NoError(t, err, "failed to create UDPRoute %s", name.String())
	}

	return func() {
		if skipCleanup {
			return
		}
		for _, route := range routes {
			namespace := route.Namespace
			if namespace == "" {
				namespace = ns
			}
			t.Logf("Deleting UDPRoute %s/%s", namespace, route.Name)
			err := client.GatewayV1alpha2().UDPRoutes(namespace).Delete(context.Background(), route.Name, metav1.DeleteOptions{})
			if err != nil {
				t.Errorf("Deleting UDPRoute %s: %v", route.Name, err)
			}
		}
	}
}

func createBackendTLSPolicies(ctx context.Context, t *testing.T, client *gwclientset.Clientset, ns string, policies map[types.NamespacedName]v1.BackendTLSPolicy, skipCleanup bool) func() {
	for name, policy := range policies {
		// Ensure the namespace is set correctly.
		if policy.Namespace == "" {
			policy.Namespace = ns
		}

		y, err := toYAML(&policy)
		require.NoError(t, err)
		t.Logf("Creating BackendTLSPolicy:\n%s", y)

		_, err = client.GatewayV1().BackendTLSPolicies(policy.Namespace).Create(
			ctx,
			&policy,
			metav1.CreateOptions{},
		)
		require.NoError(t, err, "failed to create BackendTLSPolicy %s", name.String())
	}

	return func() {
		if skipCleanup {
			return
		}
		for _, policy := range policies {
			namespace := policy.Namespace
			if namespace == "" {
				namespace = ns
			}
			t.Logf("Deleting BackendTLSPolicy %s/%s", namespace, policy.Name)
			err := client.GatewayV1().BackendTLSPolicies(namespace).Delete(context.Background(), policy.Name, metav1.DeleteOptions{})
			if err != nil {
				t.Errorf("Deleting BackendTLSPolicy %s: %v", policy.Name, err)
			}
		}
	}
}

func createReferenceGrants(ctx context.Context, t *testing.T, client *gwclientset.Clientset, ns string, grants map[types.NamespacedName]v1beta1.ReferenceGrant, skipCleanup bool) func() {
	for name, grant := range grants {
		// Ensure the namespace is set correctly.
		if grant.Namespace == "" {
			grant.Namespace = ns
		}

		y, err := toYAML(&grant)
		require.NoError(t, err)
		t.Logf("Creating ReferenceGrant:\n%s", y)

		_, err = client.GatewayV1beta1().ReferenceGrants(grant.Namespace).Create(
			ctx,
			&grant,
			metav1.CreateOptions{},
		)
		require.NoError(t, err, "failed to create ReferenceGrant %s", name.String())
	}

	return func() {
		if skipCleanup {
			return
		}
		for _, grant := range grants {
			namespace := grant.Namespace
			if namespace == "" {
				namespace = ns
			}
			t.Logf("Deleting ReferenceGrant %s/%s", namespace, grant.Name)
			err := client.GatewayV1beta1().ReferenceGrants(namespace).Delete(context.Background(), grant.Name, metav1.DeleteOptions{})
			if err != nil {
				t.Errorf("Deleting ReferenceGrant %s: %v", grant.Name, err)
			}
		}
	}
}

// Extracts all Gateway resources from the specified GatewayResources slice and returns them as a
// map of namespaced names to Gateways.
func getGateways(res []i2gw.GatewayResources) map[types.NamespacedName]gwapiv1.Gateway {
	gateways := make(map[types.NamespacedName]gwapiv1.Gateway)

	for _, r := range res {
		for k, v := range r.Gateways {
			gateways[k] = v
		}
	}

	return gateways
}

func waitForGatewayAddresses(ctx context.Context, t *testing.T, client *gwclientset.Clientset, ns string, gateways map[types.NamespacedName]v1.Gateway) map[types.NamespacedName]net.IP {
	addresses := map[types.NamespacedName]net.IP{}

	for name, gw := range gateways {
		// Ensure the namespace is set correctly.
		if gw.Namespace == "" {
			gw.Namespace = ns
		}
		t.Logf("Waiting for Gateway %s/%s to get an address", gw.Namespace, gw.Name)
		var address string
		err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
			g, err := client.GatewayV1().Gateways(gw.Namespace).Get(ctx, gw.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if len(g.Status.Addresses) > 0 {
				if g.Status.Addresses[0].Value != "" {
					address = g.Status.Addresses[0].Value
					return true, nil
				}
			}
			return false, nil
		})
		require.NoError(t, err)
		addresses[name] = net.ParseIP(address)
	}

	return addresses
}
