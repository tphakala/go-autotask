package autotask

import (
	"log/slog"
	"net/http"
)

type ClientOption func(*Client)

func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

func WithLogger(l *slog.Logger) ClientOption {
	return func(c *Client) { c.logger = l }
}

func WithBaseURL(url string) ClientOption {
	return func(c *Client) { c.baseURL = url }
}

func WithUserAgent(ua string) ClientOption {
	return func(c *Client) { c.userAgent = ua }
}

func WithImpersonation(resourceID int64) ClientOption {
	return func(c *Client) { c.impersonationID = resourceID }
}

func WithMiddleware(m Middleware) ClientOption {
	return func(c *Client) {
		c.middlewares = append(c.middlewares, m)
	}
}
