package cmd

import (
	"github.com/kolonialno/test-environment-manager/pkg"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(runCmd)
}

// RootCmd is ised as the main entrypoint for this application
var RootCmd = &cobra.Command{
	Use:   pkg.App.Name,
	Short: pkg.App.Description,
	Args:  cobra.NoArgs,
}
