package e2e

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/http"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

// Generates a random alphanumeric string of length n using the RNG r.
func randString(n int, r *rand.Rand) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"

	b := make([]byte, n)

	for i := range b {
		b[i] = letters[r.Intn(len(letters))]
	}

	return string(b)
}

// Converts a k8s object to a YAML string.
func toYAML(obj interface{}) (string, error) {
	b, err := yaml.Marshal(obj)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func fetchManifests(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("getting manifests: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response data: %w", err)
	}

	return data, nil
}

func decodeManifests(data []byte) ([]unstructured.Unstructured, error) {
	var out []unstructured.Unstructured
	decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)

	for {
		var obj unstructured.Unstructured
		err := decoder.Decode(&obj)
		if err != nil {
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("decoding object: %w", err)
			}
		}
		if obj.Object == nil {
			continue
		}
		out = append(out, obj)
	}

	return out, nil
}
