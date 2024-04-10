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

// Download initiates the download process.
// It returns nil if the download completes successfully or an error if issues occur.
// TODO(azhovan): not override existing files
func (dm *DownloadManager) Download(ctx context.Context) error {
	err := dm.Downloader.ValidateRangeSupport(ctx, dm.Downloader.UpdateRangeSupportState)
	if err != nil {
		return err
	}

	sm, err := NewSegmentManager(
		dm.Downloader.DestinationDIR.String(),
		dm.Downloader.RangeSupport.ContentLength,
	)
	if err != nil {
		return err
	}

	// capture errors for each segment
	errs := make(chan error, sm.TotalSegments)

	// Use a WaitGroup to wait for all download goroutines to complete
	wg := &sync.WaitGroup{}
	wg.Add(sm.TotalSegments)
	for _, segment := range sm.Segments {
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
				errs <- err
			}
		}(segment)
	}
	wg.Wait()
	close(errs)

	// Aggregate and return any errors encountered during the download
	var allErrors []error
	for err := range errs {
		allErrors = append(allErrors, err)
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("download encountered following errors: %v", allErrors)
	}

	return sm.MergeFiles(dm.Downloader.Filename())
}
