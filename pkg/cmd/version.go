package cmd

import (
	"fmt"

	"github.com/eschercloudai/unikorn/pkg/constants"

	"github.com/spf13/cobra"
)

// newVersionCommand returns a version command that prints out application
// and versioning information.
func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print this command's version.",
		Long:  "Print this command's version.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(constants.VersionString())
		},
	}
}
