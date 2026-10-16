package hardware

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

func httpNewRequest(method, url string, body io.Reader) (*http.Request, error) {
	return http.NewRequest(method, url, body)
}

type httpClient struct {
	timeout time.Duration
	client  *http.Client
}

func newClient(timeout time.Duration) *httpClient {
	return &httpClient{
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *httpClient) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return c.client.Do(req.WithContext(ctx))
}

func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
