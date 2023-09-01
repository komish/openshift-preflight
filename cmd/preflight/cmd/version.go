package cmd

import (
	"context"

	// Note(komish): This is the temporary location of the version subcommand,
	// but this is to be migrated to this package in the future.
	"github.com/redhat-openshift-ecosystem/openshift-preflight/internal/x/cmd/preflight/version"
	"github.com/spf13/cobra"
)

// versionCommand prints the version information for this project.
// This version output is the same as what the root command's version string
// prints, but provides flags to mutate that output.
func versionCommand() *cobra.Command {
	return version.NewCommand(context.TODO())
}
