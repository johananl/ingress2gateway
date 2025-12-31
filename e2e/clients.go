package e2e

import (
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	gwclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// NewClientFromKubeconfigPath accepts a path to a kubeconfig file and returns a k8s client set.
func NewClientFromKubeconfigPath(path string) (*kubernetes.Clientset, error) {
	cc, err := configFromKubeconfigPath(path)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(cc)
}

// NewRestConfigFromKubeconfigPath accepts a path to a kubeconfig file and returns a rest.Config.
// This is useful for operations that need direct access to the rest config, such as port-forwarding.
func NewRestConfigFromKubeconfigPath(path string) (*rest.Config, error) {
	return configFromKubeconfigPath(path)
}

// NewGatewayClientFromKubeconfigPath accepts a path to a kubeconfig file and returns a Gateway API
// client set.
func NewGatewayClientFromKubeconfigPath(path string) (*gwclientset.Clientset, error) {
	cc, err := configFromKubeconfigPath(path)
	if err != nil {
		return nil, err
	}

	return gwclientset.NewForConfig(cc)
}

// NewAPIExtensionsClientFromKubeconfigPath accepts a path to a kubeconfig file and returns an
// API extensions client set.
func NewAPIExtensionsClientFromKubeconfigPath(path string) (*apiextensionsclientset.Clientset, error) {
	cc, err := configFromKubeconfigPath(path)
	if err != nil {
		return nil, err
	}

	return apiextensionsclientset.NewForConfig(cc)
}

// NewDynamicClientFromKubeconfigPath accepts a path to a kubeconfig file and returns a dynamic client.
func NewDynamicClientFromKubeconfigPath(path string) (dynamic.Interface, error) {
	cc, err := configFromKubeconfigPath(path)
	if err != nil {
		return nil, err
	}

	return dynamic.NewForConfig(cc)
}

// Accepts a path to a kubeconfig file and returns a rest config.
// Configures increased QPS and Burst for parallel test execution.
func configFromKubeconfigPath(path string) (*rest.Config, error) {
	rules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: path}

	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules,
		&clientcmd.ConfigOverrides{},
	)

	restConfig, err := cfg.ClientConfig()
	if err != nil {
		return nil, err
	}

	// Increase rate limits for parallel test execution.
	// Default QPS is 5, Burst is 10, which is too low for parallel e2e tests
	// that make many API calls concurrently.
	restConfig.QPS = 50
	restConfig.Burst = 100

	return restConfig, nil
}
