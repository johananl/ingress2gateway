package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

	t.Logf("Sending HTTP GET requests to %s", req.URL.String())

	var res *http.Response
	require.Eventually(t, func() bool {
		res, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Logf("Sending HTTP GET: %v", err)
			return false
		}
		if res.StatusCode != 200 {
			t.Logf("Unexpected HTTP status code: got %d, want %d", res.StatusCode, 200)
			return false
		}
		return true
	}, 5*time.Minute, 1*time.Second, "Couldn't get a healthy HTTP response in time")
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading HTTP body: %w", err)
	}

	t.Logf("Got a healthy response: %s", body)

	return nil
}
