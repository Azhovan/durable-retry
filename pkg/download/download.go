// Package download provides functionality for downloading files from a server.
package download

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/azhovan/durable-resume/pkg/logger"
)

// Downloader is a struct that handles downloading files from a source URL to a destination URL.
// It utilizes a custom HTTP client for making requests and supports various configuration options.
type Downloader struct {
	// Source URL of the file to be downloaded.
	sourceURL *url.URL

	// Destination URL where the file will be saved.
	destinationURL *url.URL

	// contentLength is the value of the Content-Length header received from the server, measured in bytes.
	// When available, it allows the Downloader to calculate the total download time and manage segmented downloads.
	contentLength int64

	// acceptRanges stores the value of the Accept-Ranges header received from the server.
	// This value typically indicates the unit that can be used for range requests, such as "bytes".
	// When the server supports range requests, the Downloader can use this capability to resume downloads after interruptions.
	acceptRanges string

	// Custom HTTP client for making requests.
	client *Client

	// Optional logger for logging debug and error information.
	logger *slog.Logger
}

// NewDownloader initializes a new instance of Downloader with the provided source and destination URLs.
// It returns a pointer to the Downloader and an error, if any occurs during initialization.
// Additional configuration options can be provided to customize the Downloader's behavior.
func NewDownloader(dst, src string, options ...DownloaderOption) (*Downloader, error) {
	srcURL, err := url.Parse(src)
	if err != nil {
		return nil, err
	}
	dstURL, err := url.Parse(dst)
	if err != nil {
		return nil, err
	}

	client, err := NewClient()
	if err != nil {
		return nil, err
	}

	dl := &Downloader{
		sourceURL:      srcURL,
		destinationURL: dstURL,
		client:         client,
		logger:         logger.DefaultLogger(),
	}
	for _, opt := range options {
		opt(dl)
	}

	return dl, err
}

// DownloaderOption defines a function type for configuring a Downloader instance.
type DownloaderOption func(*Downloader)

// WithLogger is an option function that sets the Logger field of the Downloader instance to the given value.
func WithLogger(logger *slog.Logger) DownloaderOption {
	return func(dl *Downloader) {
		dl.logger = logger
	}
}

// WithClient is an option function that sets the underlying client field of the Downloader instance to the given value.
func WithClient(client *Client) DownloaderOption {
	return func(dl *Downloader) {
		dl.client = client
	}
}

// ResponseCallback defines a callback function that processes an HTTP response.
type ResponseCallback func(*http.Response)

func (dl *Downloader) UpdateRangeSupportState(response *http.Response) {
	dl.acceptRanges = response.Header.Get("Accept-Ranges")
	dl.contentLength = response.ContentLength
}

// CheckRangeSupport checks if the server supports range requests by making a test request.
// It returns true if range requests are supported, false otherwise, along with an error if the check fails.
func (dl *Downloader) CheckRangeSupport(ctx context.Context, callback ResponseCallback) (bool, error) {
	dl.logger.Debug("init server range request check", slog.Group("req", slog.String("url", dl.sourceURL.String())))

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, dl.sourceURL.String(), http.NoBody)
	if err != nil {
		dl.logger.Debug("creating range request failed",
			slog.String("error", err.Error()),
		)
		return false, fmt.Errorf("creating range request: %v", err)
	}

	// apply auth method if it's been set
	if dl.client.auth != nil {
		dl.client.auth.Apply(req)
	}

	resp, err := dl.client.httpClient.Do(req)
	if err != nil {
		dl.logger.Debug("making range request failed",
			slog.String("error", err.Error()),
			slog.Group("req", slog.String("method", req.Method), slog.String("url", req.URL.String())),
		)
		return false, fmt.Errorf("making range request: %v", err) //nolint:errcheck
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		dl.logger.Debug("server range request check finished", slog.Bool("supported", false))
		return false, ErrRangeRequestNotSupported
	}

	if callback != nil {
		dl.logger.Debug("executing range support callback")
		callback(resp)
		dl.logger.Debug("executing range support callback completed")
	}

	dl.logger.Debug("server range request check finished", slog.Bool("supported", true))
	return true, nil
}
