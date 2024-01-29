package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDownloadManager(t *testing.T) {
	t.Run("NewDownloadManager", func(t *testing.T) {
		downloader, err := NewDownloader("/tmp/xx/", "https://httpbin.org/range/512")
		if assert.NoError(t, err) {
			defer t.Cleanup(func() {
				os.RemoveAll("/tmp/xx/")
			})

			dlManager := NewDownloadManager(downloader, DefaultRetryPolicy())
			err = dlManager.StartDownload(context.Background())
			assert.NoError(t, err)
		}
	})
	t.Run("NewDownloadManager with err", func(t *testing.T) {
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
			// pretend to support range request
			wr.Header().Set("Content-Length", "123")
			wr.Header().Set("Accept-Ranges", "bytes")
			wr.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		server.Start()

		downloader, err := NewDownloader("/tmp/xx/", server.URL)
		if assert.NoError(t, err) {
			defer t.Cleanup(func() {
				os.RemoveAll("/tmp/xx/")
			})

			dlManager := NewDownloadManager(downloader, DefaultRetryPolicy())
			err = dlManager.StartDownload(context.Background())
			assert.NotNil(t, err)
		}
	})
}
