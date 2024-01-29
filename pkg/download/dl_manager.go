// Package download provides a framework for downloading files
// in segments with support for retries in case of errors.
package download

import (
	"context"
	"fmt"
	"sync"
)

// DownloadManager coordinates the segmented downloading of a file.
// It uses a Downloader for actual download operations and applies a RetryPolicy
// for handling transient errors in the download process.
type DownloadManager struct {
	// Downloader is responsible for the actual downloading of file segments.
	Downloader *Downloader

	// RetryPolicy defines the strategy for retrying download attempts in case of failure.
	RetryPolicy *RetryPolicy

	// Err is a channel for collecting errors from individual download segments.
	Err chan error
	// TODO(azhovan): ProgressTracker
}

// NewDownloadManager creates a new instance of DownloadManager with the specified downloader
// and retry policy. It returns a pointer to the DownloadManager.
func NewDownloadManager(downloader *Downloader, retryPolicy *RetryPolicy) *DownloadManager {
	return &DownloadManager{
		Downloader:  downloader,
		RetryPolicy: retryPolicy,
	}
}

// StartDownload initiates the download process.
// It returns nil if the download completes successfully or an error if issues occur.
func (dm *DownloadManager) StartDownload(ctx context.Context) error {
	// Validate server's support for range requests
	if err := dm.Downloader.ValidateRangeSupport(ctx, dm.Downloader.UpdateRangeSupportState); err != nil {
		return err
	}

	// Initialize SegmentManager for handling file segments
	smManager, err := NewSegmentManager(
		dm.Downloader.DestinationDIR.String(),
		dm.Downloader.RangeSupport.ContentLength,
	)
	if err != nil {
		return err
	}

	dm.Err = make(chan error, smManager.TotalSegments)

	// Use a WaitGroup to wait for all download goroutines to complete
	wg := &sync.WaitGroup{}
	wg.Add(smManager.TotalSegments)
	for _, segment := range smManager.Segments {
		go func(seg *Segment) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			// Attempt to download the segment with retries
			err = dm.RetryPolicy.Retry(ctx, seg.ID, func() error {
				return dm.Downloader.DownloadSegment(ctx, seg)
			})
			if err != nil {
				dm.Err <- err
			}
		}(segment)
	}
	wg.Wait()
	close(dm.Err)

	// Aggregate and return any errors encountered during the download
	var allErrors []error
	for err := range dm.Err {
		allErrors = append(allErrors, err)
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("download encountered following errors: %v", allErrors)
	}

	return nil
}
