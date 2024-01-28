package download

import (
	"context"
)

type DownloadManager struct {
	Downloader *Downloader
	// RetryPolicy // This could be an interface or struct for handling retries
	// ProgressTracker
	// Errors []error // Slice to store errors encountered during download

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

	fileSize := dm.Downloader.RangeSupport.ContentLength
	dstDIR := dm.Downloader.DestinationDIR.String()
	segmentManager, err := NewSegmentManager(dstDIR, fileSize)
	if err != nil {
		return err
	}

	// filename := path.Basen(dm.Downloader.sourceURL.String())

	for _, segment := range segmentManager.Segments {
		err := dm.Downloader.DownloadSegment(ctx, segment)
		if err != nil {
			// handle error
		}
	}

	return nil
}
