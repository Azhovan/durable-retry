package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

func newRoot() *cobra.Command {
	var rootCmd = &cobra.Command{
		Short: "A robust solution for downloading files over the internet",
		Use:   "dr download --url [ADDRESS] --out [DIRECTORY]",
	}

	rootCmd.AddCommand(newDownloadCmd(os.Stdout))

	return rootCmd
}

func Execute() error {
	return newRoot().Execute()
}
