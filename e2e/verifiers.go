package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type Verifier interface {
	Verify(ctx context.Context, log Logger, ip net.IP) error
}

type HTTPGetVerifier struct {
	Host string
	Path string
}

func (v *HTTPGetVerifier) Verify(ctx context.Context, log Logger, ip net.IP) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s%s", ip.String(), v.Path), nil)
	if err != nil {
		return fmt.Errorf("constructing HTTP request: %w", err)
	}

	if v.Host != "" {
		req.Host = v.Host
	}

	client := http.Client{Timeout: 5 * time.Second}

	log.Logf("Sending HTTP GET requests to %s", req.URL.String())

	var lastErr error
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		res, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("sending HTTP GET: %w", err)
			log.Logf("Sending HTTP GET: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		if res.StatusCode != http.StatusOK {
			res.Body.Close()
			lastErr = fmt.Errorf("unexpected HTTP status code: got %d, want %d", res.StatusCode, http.StatusOK)
			log.Logf("Unexpected HTTP status code: got %d, want %d", res.StatusCode, http.StatusOK)
			time.Sleep(1 * time.Second)
			continue
		}

		// Success - read and log the response body.
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return fmt.Errorf("reading HTTP body: %w", err)
		}

		log.Logf("Got a healthy response: %s", body)
		return nil
	}

	return fmt.Errorf("couldn't get a healthy HTTP response in time: %w", lastErr)
}
