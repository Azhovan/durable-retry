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
		tests := []struct {
			destinationDIR   string
			filesize         int64
			gotTotalSegments int
			gotSegmentSize   int64

			wantTotalSegments int
			wantSegmentSize   int64
			wantErr           error
		}{
			// file size is zero, totalSegments and gotSegmentSize are ignored
			{destinationDIR: "", filesize: 0, wantSegmentSize: 0, wantTotalSegments: 1},
			// file size is -1, totalSegments and gotSegmentSize are ignored
			{destinationDIR: "/tmp/x/y", filesize: -1, wantSegmentSize: 0, wantTotalSegments: 1},
			// file size is pre-determined, gotTotalSegments =2, and it is taken into account
			{
				destinationDIR:    "/tmp/foo/bar",
				filesize:          12,
				gotTotalSegments:  2,
				wantTotalSegments: 2,
				wantSegmentSize:   6,
				wantErr:           nil,
			},
			// file size is pre-determined, segmentSize =2, and it is taken into account
			{
				destinationDIR:    "/tmp/foo/bar",
				filesize:          12,
				gotSegmentSize:    2,
				wantTotalSegments: 6,
				wantSegmentSize:   2,
				wantErr:           nil,
			},
			// file size is pre-determined, wantTotalSegments and wantSegmentSize are calculated based on the DefaultNumberOfSegments constant
			{
				destinationDIR:    "/tmp/foo/bar",
				filesize:          12,
				wantSegmentSize:   3,
				wantTotalSegments: DefaultNumberOfSegments,
				wantErr:           nil,
			},
			// file size is pre-determined, segmentSize =2, and it is taken into account
			{
				destinationDIR:    "/tmp/foo/bar",
				filesize:          12,
				gotSegmentSize:    2,
				wantSegmentSize:   2,
				wantTotalSegments: 6,
				wantErr:           nil,
			},
			// irrelevant of file size, gotTotalSegments and segmentSize cant be set at the same time
			{
				destinationDIR:   "/tmp/foo/bar",
				filesize:         12,
				gotTotalSegments: 2,
				gotSegmentSize:   2,
				wantErr:          &InvalidParamError{param: "TotalSegments, SegmentSize", message: "these two properties are mutually exclusive, set only one of them"},
			},
		}

		for i, v := range tests {
			t.Run(fmt.Sprintf("tests:#%d", i), func(t *testing.T) {
				sm, err := NewSegmentManager(
					v.destinationDIR,
					v.filesize,
					WithNumberOfSegments(v.gotTotalSegments),
					WithSegmentSize(v.gotSegmentSize),
				)
				assert.Equal(t, err, v.wantErr)
				if err == nil {
					assert.Equal(t, sm.TotalSegments, v.wantTotalSegments)
					assert.Equal(t, sm.SegmentSize, v.wantSegmentSize)
				}
			})
		}
	})
}
