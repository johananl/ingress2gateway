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
func configFromKubeconfigPath(path string) (*rest.Config, error) {
	rules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: path}

	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules,
		&clientcmd.ConfigOverrides{},
	)

	return cfg.ClientConfig()
}
