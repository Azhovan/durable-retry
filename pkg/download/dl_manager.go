package download

import (
	"context"
	"sync"
)

type DownloadManager struct {
	Downloader *Downloader
	// RetryPolicy // This could be an interface or struct for handling retries
	// ProgressTracker
	// failedSegments []*Segment // Slice to store errors encountered during download

}

func NewDownloadManager(downloader *Downloader) *DownloadManager {
	return &DownloadManager{
		Downloader: downloader,
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
			// no need to handle error here, any error that happened
			// during segment download is persisted in the segment error field.
			_ = dm.Downloader.DownloadSegment(ctx, seg)
		}(segment)
	}
	wg.Wait()

	// here is where we should handle retry policies
	// and if error is persistent still, error is returned to the caller

	return nil
}
