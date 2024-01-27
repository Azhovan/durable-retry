package download

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/azhovan/durable-resume/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestNewDownloader(t *testing.T) {
	t.Run("CheckRangeSupport", func(t *testing.T) {
		dl, err := NewDownloader("/tmp", "https://httpbin.org/range/512")
		if assert.NoError(t, err) {
			err = dl.ValidateRangeSupport(context.Background(), dl.UpdateRangeSupportState)
			if assert.NoError(t, err) {
				assert.True(t, dl.RangeSupport.SupportsRangeRequests)
			}
		}
	})
	t.Run("UpdateRangeSupportState", func(t *testing.T) {
		dl, err := NewDownloader("/tmp", "https://httpbin.org/range/512")
		if assert.NoError(t, err) {
			err := dl.ValidateRangeSupport(context.Background(), dl.UpdateRangeSupportState)
			if assert.NoError(t, err) {
				assert.Equal(t, dl.RangeSupport.AcceptRanges, "bytes")
				assert.Equal(t, dl.RangeSupport.ContentLength, int64(512))
			}
		}
	})
	t.Run("DownloadSegment", func(t *testing.T) {
		log := logger.NewLogger(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})

		dl, err := NewDownloader("/tmp/dls/download",
			"https://httpbin.org/range/512",
			WithLogger(log),
			WithFileName("test-download-file-512"),
		)
		if assert.NoError(t, err) {
			// delete all files
			defer t.Cleanup(func() {
				os.RemoveAll(dl.DestinationDIR.String())
			})

			filename := fmt.Sprintf("download-segment-%d", time.Now().Nanosecond())
			fileWriter, err := NewFileWriter(dl.DestinationDIR.String(), ""+filename)
			if assert.NoError(t, err) {

			}

			segment, err := NewSegment(SegmentParams{
				ID:             1,
				Start:          0,
				End:            512,
				MaxSegmentSize: 512, // one segment
				Writer:         fileWriter,
			})
			err = dl.DownloadSegment(context.Background(), segment)
			if assert.NoError(t, err) {
				// seek to start of the file
				_, err = fileWriter.Seek(0, io.SeekStart)
				assert.NoError(t, err)

				b := make([]byte, 512)
				_, err = fileWriter.Read(b)
				assert.NoError(t, err)
				assert.Equal(t, 512, len(b))
			}
		}
	})
	t.Run("NewSegmentManager", func(t *testing.T) {
		t.Run("invalid instantiation", func(t *testing.T) {
			_, err := NewSegmentManager("/tmp/d/l/segments", 512)
			var er *InvalidParamError
			assert.ErrorAs(t, err, &er)
		})
		t.Run("valid instantiation", func(t *testing.T) {
			sm, err := NewSegmentManager("/tmp/d/l/segments", 512, WithNumberOfSegments(1))
			if assert.NoError(t, err) {
				assert.Equal(t, "/tmp/d/l/segments", sm.DestinationDir)
				assert.Equal(t, int64(512), sm.FileSize)
				assert.Equal(t, int64(512), sm.SegmentSize)
				assert.Equal(t, 1, sm.TotalSegments)
				assert.NotNil(t, sm.Segments)
			}
		})
	})
}
