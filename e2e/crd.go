package e2e

import (
	"context"
	"encoding/json"

	"github.com/stretchr/testify/require"
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

func deployCRDs(ctx context.Context, t TestingT, client *apiextensionsclientset.Clientset, skipCleanup bool) func() {
	yamlData := fetchManifests(t, gatewayAPIInstallURL)
	crds := decodeCRDs(t, yamlData)

	for _, crd := range crds {
		crd.TypeMeta = metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		}
		data, err := json.Marshal(crd)
		require.NoError(t, err)

		// Use server-side apply.
		_, err = client.ApiextensionsV1().CustomResourceDefinitions().Patch(ctx, crd.Name, types.ApplyPatchType, data, metav1.PatchOptions{
			FieldManager: "ingress2gateway-e2e",
		})
		require.NoError(t, err)
		t.Logf("Applied CRD %s", crd.Name)
	}

	return func() {
		if skipCleanup {
			t.Logf("Skipping cleanup of CRDs")
			return
		}
		for _, crd := range crds {
			t.Logf("Deleting CRD %s", crd.Name)
			err := client.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, crd.Name, metav1.DeleteOptions{})
			if err != nil {
				t.Errorf("Deleting CRD %s: %v", crd.Name, err)
			}
		}
	}
}

func decodeCRDs(t TestingT, yamlData []byte) []apiextensionsv1.CustomResourceDefinition {
	objs := decodeManifests(t, yamlData)
	var out []apiextensionsv1.CustomResourceDefinition

	for _, obj := range objs {
		var crd apiextensionsv1.CustomResourceDefinition
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &crd)
		require.NoError(t, err)
		if crd.Name == "" {
			continue
		}
		out = append(out, crd)
	}

	return out
}
