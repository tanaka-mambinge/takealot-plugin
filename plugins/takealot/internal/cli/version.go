package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// These values are replaced for release builds with Go linker flags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

type versionOutput struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print CLI build information",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			result := versionOutput{Version: Version, Commit: Commit, BuildDate: BuildDate}
			if options.json {
				return writeJSON(command.OutOrStdout(), result)
			}
			return writeVersion(command.OutOrStdout(), result)
		},
	}
}

func writeVersion(writer io.Writer, result versionOutput) error {
	_, err := fmt.Fprintf(writer, "takealot %s\n", result.Version)
	return err
}
