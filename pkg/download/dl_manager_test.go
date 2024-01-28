package download

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDownloadManager(t *testing.T) {
	downloader, err := NewDownloader("/tmp/xx/", "https://httpbin.org/range/512")
	if assert.NoError(t, err) {
		dlManager := NewDownloadManager(downloader)
		err = dlManager.StartDownload(context.Background())
		assert.NoError(t, err)
	}
}
