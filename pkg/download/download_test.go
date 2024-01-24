package download

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDownloader(t *testing.T) {
	t.Run("CheckRangeSupport", func(t *testing.T) {
		dl, err := NewDownloader("/tmp", "https://httpbin.org/range/1024")
		assert.NoError(t, err)

		supported, err := dl.CheckRangeSupport(context.Background(), dl.UpdateRangeSupportState)
		if assert.NoError(t, err) {
			assert.True(t, supported)
		}
	})

	t.Run("UpdateRangeSupportState", func(t *testing.T) {
		dl, err := NewDownloader("/tmp", "https://httpbin.org/range/1024")
		if assert.NoError(t, err) {
			supported, err := dl.CheckRangeSupport(context.Background(), dl.UpdateRangeSupportState)
			if assert.NoError(t, err) {
				assert.True(t, supported)
				assert.Equal(t, dl.acceptRanges, "bytes")
				assert.Equal(t, dl.contentLength, int64(1024))
			}
		}
	})
}
