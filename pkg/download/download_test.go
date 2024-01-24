package download

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDownloader(t *testing.T) {
	t.Run("test client", func(t *testing.T) {
		dl, err := NewDownloader("/tmp", "https://httpbin.org/range/1024")
		assert.NoError(t, err)

		supported, err := dl.CheckRangeSupport(context.Background(), dl.UpdateRangeSupportState)
		assert.NoError(t, err)
		assert.True(t, supported)
	})

}
