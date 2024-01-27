// Package download provides functionality for downloading files from a server.
package download

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"

	"github.com/azhovan/durable-resume/pkg/logger"
)

// Downloader is a struct that handles downloading files from a source URL to a destination URL.
// It utilizes a custom HTTP client for making requests and supports various configuration options.
type Downloader struct {
	// fileName represents the downloaded file name.
	fileName string

	// Source URL of the file to be downloaded.
	sourceURL *url.URL

	// Destination URL where the file will be saved.
	destinationDIR *url.URL

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
		destinationDIR: dstURL,
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
		dl.segmentManager.TotalSegments = count
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

// WithFileName is an option function for configuring the name of downloaded file.
func WithFileName(name string) DownloaderOption {
	return func(dl *Downloader) {
		dl.fileName = name
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
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, dl.sourceURL.String(), http.NoBody)
	if err != nil {
		return fmt.Errorf("creating range request: %v", err)
	}

	// apply auth method if it's been set
	if dl.client.auth != nil {
		dl.client.auth.Apply(req)
	}

	resp, err := dl.client.httpClient.Do(req)
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

func (dl *Downloader) DownloadFile(ctx context.Context, callback ResponseCallback) error {
	if err := dl.ValidateRangeSupport(ctx, callback); err != nil {
		return err
	}

	// if filename is not provided, use tha last part of path in url
	if dl.fileName == "" {
		dl.fileName = path.Base(dl.sourceURL.String())
	}

	// range request is supported, adjust the number of segments dynamically if necessary
	segmentSize := dl.rangeSupport.ContentLength / int64(dl.segmentManager.TotalSegments)
	if dl.rangeSupport.SupportsRangeRequests {
		if segmentSize > DefaultSegmentSize {
			dl.segmentManager.SegmentSize = segmentSize
		}
	} else { // performs a standard, non-segmented download using a single goroutine.
		dl.segmentManager.Segments = make(map[int]*Segment, 1)
		dl.segmentManager.TotalSegments = 1
	}

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

		// create a new temporary file for each segment,
		fileWriter, err := NewFileWriter(
			dl.destinationDIR.String(),
			fmt.Sprintf("%s-part-%d", dl.fileName, i),
		)
		if err != nil {
			return err
		}

		segment, err := NewSegment(SegmentParams{
			ID:             i,
			Start:          start,
			End:            end,
			MaxSegmentSize: segmentSize,
			Writer:         fileWriter,
		})
		if err != nil {
			return err
		}
		dl.SetSegment(segment)

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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dl.sourceURL.String(), http.NoBody)
	if err != nil {
		return err
	}

	if dl.rangeSupport.SupportsRangeRequests {
		req.Header.Set("Range", strconv.FormatInt(segment.Start, 10)+"-"+strconv.FormatInt(segment.End, 10))
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

	//  server is sending part of the file.
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

// SetSegment sets the given segment in the Downloader's SegmentManager.
func (dl *Downloader) SetSegment(segment *Segment) {
	dl.segmentManager.Segments[segment.ID] = segment
}
