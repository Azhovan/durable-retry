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
}

func newDownloadCmd(output io.Writer) *cobra.Command {
	var opts = &downloadOptions{}

	var cmd = &cobra.Command{
		Use:   "download --url [ADDRESS] --out [DIRECTORY]",
		Short: "download remote file and store it in the output local directory",
		Args:  cobra.MaximumNArgs(2),
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

	cmd.Flags().StringVarP(&opts.remoteURL, "url", "u", "", "remote file address")
	cmd.Flags().StringVarP(&opts.out, "out", "o", "", "local directory to save file")

	return cmd
}
