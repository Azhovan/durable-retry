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

		// wd, err := os.Getwd()
		// assert.NoError(t, err)

		dl, err := NewDownloader("./",
			"https://httpbin.org/range/1024",
			WithLogger(log),
			WithNumberOfSegments(5),
		)
		assert.NoError(t, err)

		err = dl.DownloadFile(context.Background(), dl.UpdateRangeSupportState)
		assert.NoError(t, err)
	})
}
