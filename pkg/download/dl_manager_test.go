package download

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	"github.com/azhovan/durable-resume/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestNewDownloadManager(t *testing.T) {
	t.Run("NewDownloadManager", func(t *testing.T) {
		log := logger.NewLogger(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})

		downloader, err := NewDownloader("/tmp/xxy/", "https://httpbin.org/range/512", WithLogger(log))
		if assert.NoError(t, err) {
			defer t.Cleanup(func() {
				_ = os.RemoveAll("/tmp/xxy/")
			})

			dlManager := NewDownloadManager(downloader, DefaultRetryPolicy())
			err = dlManager.Download(context.Background())
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
				_ = os.RemoveAll("/tmp/xx/")
			})

			dlManager := NewDownloadManager(downloader, DefaultRetryPolicy())
			err = dlManager.Download(context.Background())
			assert.NotNil(t, err)

			// Assert error is EOF
			assert.Regexp(t, regexp.MustCompile("unexpected EOF"), err.Error())
		}
	})
}
