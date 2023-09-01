package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/redhat-openshift-ecosystem/openshift-preflight/version"
	"github.com/spf13/cobra"
)

const flagAsJSON = "as-json"

func NewCommand(ctx context.Context) *cobra.Command {
	cmd := cobra.Command{
		Use:   "version",
		Short: "Version information for this tool.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if asJSON, _ := cmd.Flags().GetBool(flagAsJSON); asJSON {
				return printVersionAsJSON(cmd.OutOrStdout(), version.Version)
			}

			fmt.Fprintln(cmd.OutOrStdout(), version.Version.String())
			return nil
		},
	}

	cmd.SetContext(ctx)
	cmd.Flags().Bool(flagAsJSON, false, "Returns version metadata as a JSON blob")
	return &cmd
}

// printVersionASJSON prints v as a JSON blob to w.
func printVersionAsJSON(w io.Writer, v version.VersionContext) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	fmt.Fprintln(w, string(b))
	return nil
}
