package download

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/azhovan/durable-resume/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestNewDownloader(t *testing.T) {
	t.Run("CheckRangeSupport", func(t *testing.T) {
		dl, err := NewDownloader("/tmp", "https://httpbin.org/range/1024")
		if assert.NoError(t, err) {
			err = dl.ValidateRangeSupport(context.Background(), dl.UpdateRangeSupportState)
			if assert.NoError(t, err) {
				assert.True(t, dl.rangeSupport.SupportsRangeRequests)
			}
		}
	})

	t.Run("UpdateRangeSupportState", func(t *testing.T) {
		dl, err := NewDownloader("/tmp", "https://httpbin.org/range/1024")
		if assert.NoError(t, err) {
			err := dl.ValidateRangeSupport(context.Background(), dl.UpdateRangeSupportState)
			if assert.NoError(t, err) {
				assert.Equal(t, dl.rangeSupport.AcceptRanges, "bytes")
				assert.Equal(t, dl.rangeSupport.ContentLength, int64(1024))
			}
		}
	})

	t.Run("DownloadFile", func(t *testing.T) {
		log := logger.NewLogger(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})

		dl, err := NewDownloader("/tmp/dl",
			"https://httpbin.org/range/1024",
			WithLogger(log),
			WithNumberOfSegments(5),
			WithFileName("test-download-file-1024"),
		)
		assert.NoError(t, err)

		// delete all files
		defer t.Cleanup(func() {
			os.RemoveAll("/tmp/dl/")
		})

		err = dl.DownloadFile(context.Background(), dl.UpdateRangeSupportState)
		assert.NoError(t, err)

		// read downloaded file
		f, err := os.Open("/tmp/dl/")
		assert.NoError(t, err)

		entries, err := f.ReadDir(-1)
		assert.NoError(t, err)
		assert.Equal(t, 5, len(entries))

	})
}
