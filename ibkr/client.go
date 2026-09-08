package ibkr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	config  Config
	http    *http.Client
	mutex   sync.Mutex
	session liveSession
	redis   *redis.Client
}

func NewClient(config Config) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	client := &Client{config: config, http: &http.Client{Timeout: config.Timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
	if config.Cache.Mode == "redis" {
		options, err := redis.ParseURL(config.Cache.RedisURL)
		if err != nil {
			return nil, fmt.Errorf("invalid IBKR_REDIS_URL")
		}
		options.MaxRetries = -1
		client.redis = redis.NewClient(options)
	}
	return client, nil
}
func (client *Client) Close() error {
	client.http.CloseIdleConnections()
	if client.redis != nil {
		return client.redis.Close()
	}
	return nil
}

// RequestJSON performs one signed REST operation without implicit session initialization or retries.
func (client *Client) RequestJSON(ctx context.Context, method, path string, query url.Values, body any) (json.RawMessage, error) {
	session, err := client.getLiveSession(ctx)
	if err != nil {
		return nil, err
	}
	endpoint := client.config.endpointURL(path)
	header, err := signProtected(method, endpoint, query, session.Token, client.config)
	if err != nil {
		return nil, err
	}
	return client.sendJSON(ctx, "protected-resource", method, endpoint, query, body, header)
}

type HTTPError struct {
	Phase, Method, URL string
	Status             int
	Body               string
}

func (err *HTTPError) Error() string {
	return fmt.Sprintf("%s %s %s: HTTP %d: %s", err.Phase, err.Method, err.URL, err.Status, err.Body)
}
func (client *Client) sendJSON(ctx context.Context, phase, method, endpoint string, query url.Values, body any, auth string) (json.RawMessage, error) {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	requestURL := endpoint
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", auth)
	request.Header.Set("Accept", "*/*")
	request.Header.Set("User-Agent", "ibkr/"+Version)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &HTTPError{phase, method, endpoint, response.StatusCode, string(raw)}
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("%s %s %s: invalid JSON response", phase, method, endpoint)
	}
	return json.RawMessage(raw), nil
}

var Version = "1.0.0"
