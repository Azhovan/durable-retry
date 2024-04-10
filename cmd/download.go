package cmd

import (
	"io"
	"net/url"

	"github.com/azhovan/durable-resume/pkg/download"
	"github.com/spf13/cobra"
)

type downloadOptions struct {
	remoteURL string
	out       string
	segSize   int64
	segCount  int
}

func newDownloadCmd(output io.Writer) *cobra.Command {
	var opts = &downloadOptions{}

	var cmd = &cobra.Command{
		Use:   "download --url [ADDRESS] --out [DIRECTORY]",
		Short: "download remote file and store it in a local directory",
		Args:  cobra.MaximumNArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			remoteFileURL, err := url.Parse(opts.remoteURL)
			if err != nil {
				return err
			}
			downloader, err := download.NewDownloader(opts.out, remoteFileURL.String())
			if err != nil {
				return err
			}

			dm := download.NewDownloadManager(downloader, download.DefaultRetryPolicy())
			return dm.Download(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&opts.remoteURL, "url", "u", "", "The remote file address to download.")
	cmd.Flags().StringVarP(&opts.out, "out", "o", "", "The local target directory to save file.")
	cmd.Flags().Int64VarP(&opts.segSize, "segment-size", "s", 0, "The size of each segment for download a file.")
	cmd.Flags().IntVarP(&opts.segCount, "segment-count", "n", download.DefaultNumberOfSegments, "The number of segments for download a file.")

	return cmd
}
