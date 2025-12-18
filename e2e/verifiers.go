package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
)

type Verifier interface {
	Verify(ctx context.Context, t *testing.T, ip net.IP) error
}

type HTTPGetVerifier struct {
	Host string
	Path string
}

func (v *HTTPGetVerifier) Verify(ctx context.Context, t *testing.T, ip net.IP) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s%s", ip.String(), v.Path), nil)
	if err != nil {
		return fmt.Errorf("constructing HTTP request: %w", err)
	}

	if v.Host != "" {
		req.Host = v.Host
	}

	t.Logf("Sending HTTP GET request to %s", req.URL.String())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Sending HTTP GET: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return fmt.Errorf("Unexpected HTTP status code: got %d, want %d", res.StatusCode, 200)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading HTTP body: %w", err)
	}

	t.Logf("Got a response: %s", body)

	return nil
}
