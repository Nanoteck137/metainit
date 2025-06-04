package cmd

import (
	"log"

	"github.com/nanoteck137/metainit"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     metainit.AppName,
	Version: metainit.Version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	rootCmd.SetVersionTemplate(metainit.VersionTemplate(metainit.AppName))
}
