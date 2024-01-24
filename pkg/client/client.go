package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/azhovan/durable-retry/pkg/logger"
)

type Client struct {
	// url is not meant to be modified directly, hence embedded.
	url        url.URL
	httpClient *http.Client

	// an optional logger, can be nil
	Logger *slog.Logger
}

var (
	ErrRangeRequestNotSupported = errors.New("server doesn't support range request")
	ErrInvalidURL               = errors.New("the client url is empty or invalid")
)

// NewClient creates a new instance of the Client struct with the provided server URL and options.
// It returns a pointer to the Client and an error, if any.
// If the serverURL is not a valid URL, NewClient returns nil and an error.
func NewClient(serverURL string, options ...Option) (*Client, error) {
	if serverURL == "" {
		return nil, ErrInvalidURL
	}

	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, err
	}

	client := &Client{
		url:        *u,
		httpClient: http.DefaultClient,
		Logger:     logger.DefaultLogger(),
	}

	for _, opt := range options {
		opt(client)
	}

	return client, nil
}

// Option represents a function that configures a Client.
//
// When creating a new Client using the NewClient function, you can
// pass one or more Option functions to customize the Client's behavior.
//
// Example usage:
//
//	```go
//	client := NewClient("http://example.com", WithLogger(logger))
//	```
type Option func(*Client)

// WithLogger is an Option function that sets the Logger field of the Client to the given logger instance.
func WithLogger(logger *slog.Logger) Option {
	return func(client *Client) {
		client.Logger = logger
	}
}

// WithHTTPClient is an Option function that allows the user to provide a custom
// *http.Client to be used for making HTTP requests in the Client struct.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		client.httpClient = httpClient
	}
}

// CheckServerRangeRequest checks if the server supports range requests by making a test request.
// If the range request cannot be created, an error is returned.
func (c *Client) CheckServerRangeRequest(ctx context.Context) (bool, error) {
	c.Logger.Debug("initializing server range request check",
		slog.Group("req",
			slog.String("url", c.url.String()),
			slog.String("range", "bytes=0-49"),
		))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url.String(), http.NoBody)
	if err != nil {
		c.Logger.Debug("creating range request failed",
			slog.String("error", err.Error()),
		)
		return false, fmt.Errorf("creating range request: %v", err)
	}
	// The Range header specifies a subset of a resource to fetch, instead of retrieving the entire resource.
	// In here, we use the Range header to request a specific part of the file, aiming to retrieve only
	// a portion of its content. This serves to confirm that the server correctly handles requests for partial content.
	// The range "bytes=0-49" requests the first 50 bytes of the file. This specific range is chosen arbitrarily
	// for testing purposes; any other valid range would also be appropriate for this test.
	req.Header.Set("Range", "bytes=0-49")

	res, err := c.httpClient.Do(req)
	if err != nil {
		c.Logger.Debug("making range request failed",
			slog.String("error", err.Error()),
			slog.Group("req", slog.String("method", req.Method), slog.String("url", req.URL.String())),
		)
		return false, fmt.Errorf("making range request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusPartialContent {
		c.Logger.Debug("server range request check finished", slog.Bool("supported", true))
		return true, nil
	}

	c.Logger.Debug("server range request check finished", slog.Bool("supported", false))
	return false, ErrRangeRequestNotSupported
}
