package cmd

import (
	"fmt"
	"io"
	"net/url"

	"github.com/azhovan/durable-resume/pkg/download"
	"github.com/spf13/cobra"
)

type downloadOptions struct {
	remoteURL string

	segSize  int64
	segCount int

	dstDIR   string
	filename string
}

func newDownloadCmd(output io.Writer) *cobra.Command {
	var opts = &downloadOptions{}

	var cmd = &cobra.Command{
		Use:   "download --url [ADDRESS] --out [DIRECTORY]",
		Short: "download remote file and store it in a local directory",
		Args:  cobra.MaximumNArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := url.ParseRequestURI(opts.remoteURL)
			if err != nil {
				return fmt.Errorf("invalid remote url: %v", err)
			}

			downloader, err := download.NewDownloader(
				opts.dstDIR,
				src.String(),
				download.WithFileName(opts.filename),
			)
			if err != nil {
				return err
			}

			dm := download.NewDownloadManager(downloader, download.DefaultRetryPolicy())

			fmt.Println("Downloading ...")
			err = dm.Download(cmd.Context(), download.WithSegmentSize(opts.segSize), download.WithNumberOfSegments(opts.segCount))
			if err != nil {
				return err
			}
			fmt.Println("Download completed.")

			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.remoteURL, "url", "u", "", "The remote file address to download.")
	cmd.Flags().StringVarP(&opts.dstDIR, "out", "o", "", "The local file target directory to save file.")
	cmd.Flags().Int64VarP(&opts.segSize, "segment-size", "s", 0, "The size of each segment for download a file.")
	cmd.Flags().IntVarP(&opts.segCount, "segment-count", "n", download.DefaultNumberOfSegments, "The number of segments for download a file.")
	cmd.Flags().StringVarP(&opts.filename, "file", "f", "", "The downloaded file name")

	return cmd
}
