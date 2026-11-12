// Package httpload fetches an OpenAPI document from a URL into memory.
package httpload

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Timeout bounds a single fetch. It is a package variable so tests can make
// the behaviour visible without waiting fifteen seconds.
var Timeout = 15 * time.Second

// Fetch GETs rawURL and returns the response body. Redirects are followed; a
// non-2xx response is an error that includes the status.
func Fetch(rawURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", rawURL, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("fetch %s: unexpected status %s", rawURL, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rawURL, err)
	}
	return body, nil
}
