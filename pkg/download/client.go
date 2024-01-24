package download

import (
	"errors"
	"net/http"
	"net/url"
)

type Client struct {
	// url is not meant to be modified directly, hence embedded.
	url        *url.URL
	httpClient *http.Client

	auth AuthStrategy
}

var (
	ErrRangeRequestNotSupported = errors.New("server doesn't support range request")
	ErrInvalidURL               = errors.New("the client url is empty or invalid")
)

// NewClient creates a new instance of the Client struct with the provided server URL and options.
func NewClient(url *url.URL, options ...ClientOption) (*Client, error) {
	if url.String() == "" {
		return nil, ErrInvalidURL
	}

	client := &Client{
		url:        url,
		httpClient: http.DefaultClient,
	}

	for _, opt := range options {
		opt(client)
	}

	return client, nil
}

// ClientOption represents a function that configures a Client.
//
// When creating a new Client using the NewClient function, you can
// pass one or more Option functions to customize the Client's behavior.
//
// Example usage:
//
//	```go
//	client := NewClient("http://example.com", WithLogger(logger))
//	```
type ClientOption func(*Client)

// WithHTTPClient is an Option function that allows the user to provide a custom
// *http.Client to be used for making HTTP requests in the Client struct.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(client *Client) {
		client.httpClient = httpClient
	}
}

// WithAuth is an option function that allow the user to provide authentication method.
func WithAuth(auth AuthStrategy) ClientOption {
	return func(client *Client) {
		client.auth = auth
	}
}

// AuthStrategy represents an interface for applying authentication to an HTTP request.
//
// The Apply method takes a *http.Request argument and modifies it to include any necessary
// authentication headers or other credentials.
type AuthStrategy interface {
	Apply(req *http.Request)
}

var _ AuthStrategy = (*BasicAuth)(nil)
var _ AuthStrategy = (*BearerToken)(nil)
var _ AuthStrategy = (*APIToken)(nil)

// BasicAuth represents basic authentication credentials.
type BasicAuth struct {
	username, password string
}

func (b *BasicAuth) Apply(req *http.Request) {
	req.SetBasicAuth(b.username, b.password)
}

// BearerToken represents Bearer Token (like JWT) authentication credentials.
type BearerToken struct {
	Token string
}

func (b *BearerToken) Apply(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+b.Token)
}

// APIToken represents an API token that consists of an API key and an API token string.
type APIToken struct {
	APIKey, APIToken string
}

func (a *APIToken) Apply(req *http.Request) {
	req.Header.Add(a.APIKey, a.APIToken)
}
