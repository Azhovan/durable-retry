// Package download provides functionality for downloading files from a server.
package download

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/azhovan/durable-resume/pkg/logger"
)

// Downloader is a struct that handles downloading files from a source URL to a destination URL.
// It utilizes a custom HTTP Client for making requests and supports various configuration options.
type Downloader struct {
	// FileName represents the downloaded file name.
	FileName string

	// Source URL of the file to be downloaded.
	SourceURL *url.URL

	// Destination URL where the file will be saved.
	DestinationDIR *url.URL

	// RangeSupport provides information about the server's capabilities regarding range requests.
	RangeSupport RangeSupport

	// Custom HTTP Client for making requests.
	Client *Client

	// Optional Logger for logging debug and error information.
	Logger *slog.Logger
}

type RangeSupport struct {
	// SupportsRangeRequests is true if the server supports download ranges.
	SupportsRangeRequests bool

	// ContentLength is the value of the Content-Length header received from the server, measured in bytes.
	// When available, it allows the Downloader to calculate the total download time and manage segmented downloads.
	ContentLength int64

	// AcceptRanges stores the value of the Accept-Ranges header received from the server.
	// This value typically indicates the unit that can be used for range requests, such as "bytes".
	// When the server supports range requests, the Downloader can use this capability to resume downloads after interruptions.
	AcceptRanges string
}

// NewDownloader initializes a new instance of Downloader with the provided source and destination URLs.
// It returns a pointer to the Downloader and an error, if any occurs during initialization.
// Additional configuration options can be provided to customize the Downloader's behavior.
// TODO(azhovan): dst should be verified
func NewDownloader(dst, src string, options ...DownloaderOption) (*Downloader, error) {
	srcURL, err := url.Parse(src)
	if err != nil {
		return nil, err
	}
	dstURL, err := url.Parse(dst)
	if err != nil {
		return nil, err
	}

	Client, err := NewClient()
	if err != nil {
		return nil, err
	}

	dl := &Downloader{
		SourceURL:      srcURL,
		DestinationDIR: dstURL,
		Client:         Client,
		Logger:         logger.DefaultLogger(),
	}
	for _, opt := range options {
		opt(dl)
	}

	return dl, err
}

// DownloaderOption defines a function type for configuring a Downloader instance.
type DownloaderOption func(*Downloader)

// WithLogger is an option function that sets the Logger field of the Downloader instance to the given value.
func WithLogger(Logger *slog.Logger) DownloaderOption {
	return func(dl *Downloader) {
		dl.Logger = Logger
	}
}

// WithClient is an option function that sets the underlying Client field of the Downloader instance to the given value.
func WithClient(Client *Client) DownloaderOption {
	return func(dl *Downloader) {
		dl.Client = Client
	}
}

// WithFileName is an option function for configuring the name of downloaded file.
func WithFileName(name string) DownloaderOption {
	return func(dl *Downloader) {
		dl.FileName = name
	}
}

// ResponseCallback defines a callback function that processes an HTTP response.
type ResponseCallback func(*http.Response)

// UpdateRangeSupportState update the Downloader's understanding of the server's support
// for range requests based on the HTTP response received
func (dl *Downloader) UpdateRangeSupportState(response *http.Response) {
	ac := response.Header.Get("Accept-Ranges")
	cl := response.ContentLength
	if ac == "" && cl <= 0 {
		return
	}

	dl.RangeSupport.SupportsRangeRequests = true
	dl.RangeSupport.AcceptRanges = response.Header.Get("Accept-Ranges")
	dl.RangeSupport.ContentLength = response.ContentLength
}

// ValidateRangeSupport checks if the server supports range requests by making a test request.
// It returns true if range requests are supported, false otherwise, along with an error if the check fails.
func (dl *Downloader) ValidateRangeSupport(ctx context.Context, callback ResponseCallback) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, dl.SourceURL.String(), http.NoBody)
	if err != nil {
		return fmt.Errorf("creating range request: %v", err)
	}

	// apply auth method if it's been set
	if dl.Client.auth != nil {
		dl.Client.auth.Apply(req)
	}

	resp, err := dl.Client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("making range request: %v", err) //nolint:errcheck
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return ErrRangeRequestNotSupported
	}

	if callback != nil {
		callback(resp)
	}

	return nil
}

func (dl *Downloader) DownloadSegment(ctx context.Context, segment *Segment) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dl.SourceURL.String(), http.NoBody)
	if err != nil {
		return err
	}

	if dl.RangeSupport.SupportsRangeRequests {
		req.Header.Set("Range", strconv.FormatInt(segment.Start, 10)+"-"+strconv.FormatInt(segment.End, 10))
	}

	if dl.Client.auth != nil {
		dl.Client.auth.Apply(req)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	resp, err := dl.Client.httpClient.Do(req)
	if err != nil {
		segment.setErr(err)
		return err
	}
	defer resp.Body.Close()

	// the server is sending the entire content of the file.
	if resp.StatusCode == http.StatusOK {
		_, err := segment.ReadFrom(resp.Body)
		if err != nil {
			segment.setErr(err)
			return err
		}

		return segment.setDone(true)
	}

	// server has no more data.
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		return segment.setDone(true)
	}

	//  the server is sending part of the file.
	if resp.StatusCode == http.StatusPartialContent {
		_, err := segment.ReadFrom(resp.Body)
		if err != nil {
			segment.setErr(err)
			return err
		}

		return nil
	}

	return segment.setDone(false)
}
