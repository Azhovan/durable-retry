// Package download provides functionality for downloading files from a server.
package download

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

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

	// segments is a map where each key represents a segment index, and each value is a Segment struct.
	// Each Segment struct contains a slice of bytes representing a part of the file being downloaded.
	//
	// The number of segments controls the concurrency level of the download process.
	// By default, the concurrency level is set to two, allowing two parts of the file to be downloaded simultaneously.
	// This concurrency level can be adjusted to optimize download speed or to comply with server limitations.
	// When set to one, the downloader performs a standard, non-segmented download using a single goroutine.
	// If the server does not support segmented streaming, the downloader automatically falls back to this mode.
	//
	// TODO(azhovan): consider dynamic adjustment of segment size
	segments map[int]*Segment

	// segmentSize specifies the maximum size in bytes that a segment can handle.
	// It's used to control the volume of data fetched in a single request and can be adjusted for optimization.
	segmentSize int64
}

const (
	// DefaultNumberOfSegments is the default number of segments into which a file is divided for downloading.
	// This value can be adjusted based on network conditions, file size, or other specific requirements.
	DefaultNumberOfSegments = 2

	// DefaultSegmentSize defines the default size of each segment in a file download.
	DefaultSegmentSize = 4 << 20 // 4MB per segment
)

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

	client, err := NewClient()
	if err != nil {
		return nil, err
	}

	dl := &Downloader{
		sourceURL:      srcURL,
		destinationURL: dstURL,
		client:         client,
		logger:         logger.DefaultLogger(),
		segments:       make(map[int]*Segment, DefaultNumberOfSegments),
		segmentSize:    DefaultSegmentSize,
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

// WithNumberOfSegments is an option function used to configure the number of segments for a Downloader instance.
// This function sets up the initial segment map with the specified size, where each segment represents a part
// of the file to be downloaded. Each segment in the map will be downloaded concurrently.
//
// Note:
// - A higher segment count can increase download speed by downloading parts of the file in parallel, but
//   it can also lead to increased memory and network resource usage.
// - Conversely, a lower segment size might be more efficient in terms of resource usage, especially in
//   environments with bandwidth limitations or less capable servers.
// - It is important to find a balance that suits the specific requirements of the environment and the file size.

func WithNumberOfSegments(count int) DownloaderOption {
	return func(dl *Downloader) {
		dl.segments = make(map[int]*Segment, count)
	}
}

// WithSegmentSize is an option function for configuring the size of each segment in a Downloader instance.
// This function sets the size of individual segments into which the file will be divided during the download process.
func WithSegmentSize(size int64) DownloaderOption {
	return func(dl *Downloader) {
		dl.segmentSize = size
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

func (dl *Downloader) DownloadFile(ctx context.Context, callback ResponseCallback) error {
	supportsRange, err := dl.CheckRangeSupport(ctx, callback)
	if err != nil {
		return err
	}

	// performs a standard, non-segmented download using a single goroutine.
	if !supportsRange {
		dl.segments = make(map[int]*Segment, 1)
	}

	if supportsRange {
		segSize := dl.contentLength / int64(len(dl.segments))
		if segSize > DefaultSegmentSize {
			dl.segmentSize = segSize
		}
	}

	// determining segment size from content-length sent by server.
	// when file is very small, default segment size is used.

	for i := 0; i < len(dl.segments); i++ {
		var start, end = int64(0), int64(0)

		if supportsRange {
			start = int64(i) * dl.segmentSize
			end = start + dl.segmentSize - 1
			// when the file size is predetermined, end range can be set for last segment.
			// in other cases we are relying on the server returned status code
			if dl.contentLength != 0 && i == len(dl.segments)-1 {
				end = int64(dl.contentLength) - 1
			}
		}

		segment, err := NewSegment(i, start, end, dl.segmentSize)
		if err != nil {
			return err
		}
		dl.segments[i] = segment

		// this should be handled in concurrent mode
		err = dl.download(ctx, segment, supportsRange)
		// this is not correct, I should handle this correctly ...
		if err != nil {
			return err
		}
	}

	return nil
}

func (dl *Downloader) download(ctx context.Context, segment *Segment, supportsRange bool) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dl.sourceURL.String(), http.NoBody)
	if err != nil {
		return err
	}

	if supportsRange {
		req.Header.Set("Range", strconv.FormatInt(segment.start, 10)+"-"+strconv.FormatInt(segment.end, 10))
	}

	if dl.client.auth != nil {
		dl.client.auth.Apply(req)
	}

	resp, err := dl.client.httpClient.Do(req)
	if err != nil {
		segment.setErr(err)
		return err
	}
	defer resp.Body.Close()

	// server is sending the entire content of the file.
	if resp.StatusCode == http.StatusOK {
		_, err = io.Copy(segment, resp.Body)
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

	//  server is sending part of the file.
	if resp.StatusCode == http.StatusPartialContent {
		_, err = io.Copy(segment, resp.Body)
		if err != nil {
			segment.setErr(err)
			return err
		}

		return nil
	}

	// there is more data to come.
	return segment.setDone(false)
}
