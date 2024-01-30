package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
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
		server := httptest.NewServer(http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
			// Simulate the support of the range request and have a body of 123 bytes.
			// This triggers the EOF error, causing the download to fail.
			wr.Header().Set("Content-Length", "123")
			wr.Header().Set("Accept-Ranges", "bytes")
			wr.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		downloader, err := NewDownloader("/tmp/xx/", server.URL)
		if assert.NoError(t, err) {
			defer t.Cleanup(func() {
				os.RemoveAll("/tmp/xx/")
			})

			dlManager := NewDownloadManager(downloader, DefaultRetryPolicy())
			err = dlManager.StartDownload(context.Background())
			assert.NotNil(t, err)

			// Assert error is EOF
			assert.Regexp(t, regexp.MustCompile("unexpected EOF"), err.Error())
		}
	})
}
