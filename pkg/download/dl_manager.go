package download

import (
	"context"
	"fmt"
	"sync"
)

type DownloadManager struct {
	Downloader  *Downloader
	RetryPolicy *RetryPolicy
	Err         chan error
	// ProgressTracker
}

func NewDownloadManager(downloader *Downloader, retryPolicy *RetryPolicy) *DownloadManager {
	return &DownloadManager{
		Downloader:  downloader,
		RetryPolicy: retryPolicy,
	}
}

func (dm *DownloadManager) StartDownload(ctx context.Context) error {
	if err := dm.Downloader.ValidateRangeSupport(ctx, dm.Downloader.UpdateRangeSupportState); err != nil {
		return err
	}

	smManager, err := NewSegmentManager(
		dm.Downloader.DestinationDIR.String(),
		dm.Downloader.RangeSupport.ContentLength,
	)
	if err != nil {
		return err
	}

	dm.Err = make(chan error, smManager.TotalSegments)

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

			// this resumes to previous state, if segment had data in it
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

	var allErrors []error
	for err := range dm.Err {
		allErrors = append(allErrors, err)
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("download encountered following errors: %v", allErrors)
	}

	return nil
}
