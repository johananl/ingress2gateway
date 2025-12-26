package e2e

import (
	"context"
	"encoding/json"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

const (
	gatewayAPIVersion    = "v1.4.1"
	gatewayAPIInstallURL = "https://github.com/kubernetes-sigs/gateway-api/releases/download/" + gatewayAPIVersion + "/standard-install.yaml"
)

func deployCRDs(ctx context.Context, log Logger, client *apiextensionsclientset.Clientset, skipCleanup bool) (func(), error) {
	log.Logf("Fetching manifests from %s", gatewayAPIInstallURL)
	yamlData, err := fetchManifests(gatewayAPIInstallURL)
	if err != nil {
		return nil, fmt.Errorf("fetching manifests from %s: %w", gatewayAPIInstallURL, err)
	}

	crds, err := decodeCRDs(yamlData)
	if err != nil {
		return nil, fmt.Errorf("decoding CRDs: %w", err)
	}

	for _, crd := range crds {
		crd.TypeMeta = metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		}
		data, err := json.Marshal(crd)
		if err != nil {
			return nil, fmt.Errorf("converting CRD %s to JSON: %w", crd.Name, err)
		}

		// Use server-side apply.
		if _, err = client.ApiextensionsV1().CustomResourceDefinitions().Patch(ctx, crd.Name, types.ApplyPatchType, data, metav1.PatchOptions{
			FieldManager: "ingress2gateway-e2e",
		}); err != nil {
			return nil, fmt.Errorf("applying CRD %s: %w", crd.Name, err)
		}
		log.Logf("Applied CRD %s", crd.Name)
	}

	return func() {
		if skipCleanup {
			log.Logf("Skipping cleanup of CRDs")
			return
		}
		for _, crd := range crds {
			log.Logf("Deleting CRD %s", crd.Name)
			if err := client.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, crd.Name, metav1.DeleteOptions{}); err != nil {
				log.Logf("Deleting CRD %s: %v", crd.Name, err)
			}
		}
	}, nil
}

func decodeCRDs(yamlData []byte) ([]apiextensionsv1.CustomResourceDefinition, error) {
	objs, err := decodeManifests(yamlData)
	if err != nil {
		return nil, fmt.Errorf("decoding manifests: %w", err)
	}

	var out []apiextensionsv1.CustomResourceDefinition

	for _, obj := range objs {
		var crd apiextensionsv1.CustomResourceDefinition
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &crd); err != nil {
			return nil, fmt.Errorf("converting object: %w", err)
		}

		if crd.Name == "" {
			continue
		}
		out = append(out, crd)
	}

	return out, nil
}
