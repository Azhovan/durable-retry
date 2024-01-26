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

	// rangeSupport provides information about the server's capabilities regarding range requests.
	rangeSupport RangeSupport

	// Custom HTTP client for making requests.
	client *Client

	// Optional logger for logging debug and error information.
	logger *slog.Logger

	// SegmentManager encapsulates all the information about the downloading segments.
	segmentManager SegmentManager
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

// SegmentManager manages the segments involved in a file download process.
// It encapsulates all the information and operations related to the segments
// which are parts of the file being downloaded.
type SegmentManager struct {
	// Segments is a map where each key represents a unique segment index,
	// and the corresponding value is a pointer to a Segment struct.
	// Each Segment struct contains data representing a specific part of the
	// file being downloaded. This map is populated dynamically as the file
	// download progresses.
	//
	// TODO(azhovan): consider dynamic adjustment of segment size
	Segments map[int]*Segment

	// SegmentSize specifies the maximum size, in bytes, that each segment can contain.
	// It determines the volume of data each segment fetches in a single request and
	// can be adjusted to optimize data transfer based on network capabilities and server
	// limitations. Smaller segment sizes can be more efficient in low-bandwidth situations,
	// while larger sizes may enhance performance over high-speed connections.
	SegmentSize int64

	// TotalSegments represents the total number of segments
	// that the downloader will use for the download process.
	// This number controls the concurrency level of the download,
	// with each segment being downloaded simultaneously.
	// A higher value for TotalSegments can increase download speeds
	// but may also lead to increased memory and network resource usage.
	// Conversely, a lower value may be more resource-efficient.
	TotalSegments int
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
		segmentManager: SegmentManager{
			Segments:      make(map[int]*Segment, DefaultNumberOfSegments),
			SegmentSize:   DefaultSegmentSize,
			TotalSegments: DefaultNumberOfSegments,
		},
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
		dl.segmentManager.Segments = make(map[int]*Segment, count)
	}
}

// WithSegmentSize is an option function for configuring the size of each segment in a Downloader instance.
// This function sets the size of individual segments into which the file will be divided during the download process.
func WithSegmentSize(size int64) DownloaderOption {
	return func(dl *Downloader) {
		dl.segmentManager.SegmentSize = size
	}
}

// ResponseCallback defines a callback function that processes an HTTP response.
type ResponseCallback func(*http.Response)

func (dl *Downloader) UpdateRangeSupportState(response *http.Response) {
	ac := response.Header.Get("Accept-Ranges")
	cl := response.ContentLength
	if ac == "" && cl <= 0 {
		return
	}

	dl.rangeSupport.SupportsRangeRequests = true
	dl.rangeSupport.AcceptRanges = response.Header.Get("Accept-Ranges")
	dl.rangeSupport.ContentLength = response.ContentLength
}

// ValidateRangeSupport checks if the server supports range requests by making a test request.
// It returns true if range requests are supported, false otherwise, along with an error if the check fails.
func (dl *Downloader) ValidateRangeSupport(ctx context.Context, callback ResponseCallback) error {
	dl.logger.Debug("init server range request check", slog.Group("req", slog.String("url", dl.sourceURL.String())))

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, dl.sourceURL.String(), http.NoBody)
	if err != nil {
		dl.logger.Debug("creating range request failed",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("creating range request: %v", err)
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
		return fmt.Errorf("making range request: %v", err) //nolint:errcheck
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		dl.logger.Debug("server range request check finished", slog.Bool("supported", false))
		return ErrRangeRequestNotSupported
	}

	if callback != nil {
		dl.logger.Debug("executing range support callback")
		callback(resp)
		dl.logger.Debug("executing range support callback completed")
	}

	dl.logger.Debug("server range request check finished", slog.Bool("supported", true))
	return nil
}

func (dl *Downloader) DownloadFile(ctx context.Context, callback ResponseCallback) error {
	dl.logger.Debug("range request validation started")

	if err := dl.ValidateRangeSupport(ctx, callback); err != nil {
		dl.logger.Debug("file download failed",
			slog.Group("req", slog.String("url", dl.sourceURL.String())),
			slog.String("error", err.Error()))

		return err
	}

	dl.logger.Debug("range request validation finished")

	// range request is supported, adjust the number of segments dynamically
	if dl.rangeSupport.SupportsRangeRequests {
		totalSegments := int(dl.rangeSupport.ContentLength / dl.segmentManager.SegmentSize)
		if totalSegments > DefaultNumberOfSegments {
			dl.logger.Debug("adjusting the total segment counts",
				slog.Int("oldValue", dl.segmentManager.TotalSegments),
				slog.Int("newValue", totalSegments))
			dl.segmentManager.TotalSegments = totalSegments
		} else {
			segmentSize := dl.rangeSupport.ContentLength / DefaultNumberOfSegments
			dl.logger.Debug("adjusting the segment size using DefaultNumberOfSegments",
				slog.Int64("oldValue", dl.segmentManager.SegmentSize),
				slog.Int64("newValue", segmentSize))
			dl.segmentManager.SegmentSize = segmentSize
		}

	} else {
		// performs a standard, non-segmented download using a single goroutine.
		dl.segmentManager.Segments = make(map[int]*Segment, 1)
	}

	dl.logger.Debug("range request support",
		slog.Bool("SupportsRangeRequests", dl.rangeSupport.SupportsRangeRequests),
		slog.Int64("SegmentSize", dl.segmentManager.SegmentSize),
		slog.Int("TotalSegments", dl.segmentManager.TotalSegments),
	)

	segmentSize := dl.segmentManager.SegmentSize

	for i := 0; i < dl.segmentManager.TotalSegments; i++ {
		var start, end = int64(0), int64(0)
		if dl.rangeSupport.SupportsRangeRequests {
			start = int64(i) * segmentSize
			end = start + segmentSize - 1
			// Set the end range can be set for last segment.
			// in other cases we are relying on the server returned status code
			if i == dl.segmentManager.TotalSegments-1 {
				end = dl.rangeSupport.ContentLength - 1
			}
		}

		segment, err := NewSegment(i, start, end, segmentSize)
		if err != nil {
			return err
		}

		dl.logger.Debug("segment created", slog.Group("segment",
			slog.Int("index", i),
			slog.Int64("size", segmentSize),
			slog.Int64("start", start),
			slog.Int64("end", end),
		))

		dl.segmentManager.Segments[i] = segment

		// this should be handled in concurrent mode
		// this should be handled in concurrent mode
		// this should be handled in concurrent mode
		err = dl.download(ctx, segment)
		// this is not correct, I should handle this correctly ...
		// this is not correct, I should handle this correctly ...
		// this is not correct, I should handle this correctly ...
		if err != nil {
			return err
		}
	}

	return nil
}

func (dl *Downloader) download(ctx context.Context, segment *Segment) error {
	dl.logger.Debug("segment download started")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dl.sourceURL.String(), http.NoBody)
	if err != nil {
		dl.logger.Debug("segment download request creation failed",
			slog.Group("req",
				slog.String("url", req.URL.String()),
				slog.String("method", req.Method),
			),
			slog.String("error", err.Error()))
		return err
	}

	if dl.rangeSupport.SupportsRangeRequests {
		req.Header.Set("Range", strconv.FormatInt(segment.start, 10)+"-"+strconv.FormatInt(segment.end, 10))
	}

	if dl.client.auth != nil {
		dl.logger.Debug("remote server authentication")
		dl.client.auth.Apply(req)
	}

	dl.logger.Debug("sending http request to server started")

	resp, err := dl.client.httpClient.Do(req)
	if err != nil {
		dl.logger.Debug("segment download failed",
			slog.Group("req",
				slog.String("url", req.URL.String()),
				slog.String("method", req.Method),
			),
			slog.String("error", err.Error()))

		segment.setErr(err)
		return err
	}
	defer resp.Body.Close()
	dl.logger.Debug("sending http request to server finished")

	// server is sending the entire content of the file.
	if resp.StatusCode == http.StatusOK {
		dl.logger.Debug("downloading entire file's content finished")

		dl.logger.Debug("copying entire file's content into segment buffer started")
		_, err = io.Copy(segment, resp.Body)
		if err != nil {
			dl.logger.Debug("copying entire file's content into segment buffer failed", slog.String("error", err.Error()))
			segment.setErr(err)
			return err
		}

		dl.logger.Debug("copying entire file's content into segment buffer finished")
		return segment.setDone(true)
	}

	// server has no more data.
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		dl.logger.Debug("server has no more data.")
		return segment.setDone(true)
	}

	//  server is sending part of the file.
	if resp.StatusCode == http.StatusPartialContent {
		dl.logger.Debug("copying part of the file that server sent")

		dl.logger.Debug("copying part of the file that server sen into segment buffer started")
		_, err = io.Copy(segment, resp.Body)
		if err != nil {
			dl.logger.Debug("copying part of the file that server sen into segment buffer failed", slog.String("error", err.Error()))
			segment.setErr(err)
			return err
		}

		dl.logger.Debug("copying part of the file that server sen into segment buffer finished")
		return nil
	}

	dl.logger.Debug("segment download failed",
		slog.Group("req",
			slog.String("url", req.URL.String()),
			slog.String("method", req.Method),
			slog.Int("statusCode", resp.StatusCode),
		))
	return segment.setDone(false)
}
