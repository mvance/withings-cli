package cli

import (
	"fmt"

	"github.com/mreimbold/withings-cli/internal/auth"
	"github.com/mreimbold/withings-cli/internal/services/measures"
	"github.com/spf13/cobra"
)

func newMeasuresCommand() *cobra.Command {
	var opts measures.Options

	//nolint:exhaustruct // Cobra command defaults are intentional.
	measuresCmd := &cobra.Command{
		Use:   "measures",
		Short: "Body measures (weight, blood pressure, composition)",
	}
	//nolint:exhaustruct // Cobra command defaults are intentional.
	measuresGetCmd := &cobra.Command{
		Use:   "get",
		Short: "Fetch body measures",
		RunE: func(cmd *cobra.Command, _ []string) error {
			appOpts, err := readGlobalOptions(cmd.Root().PersistentFlags())
			if err != nil {
				return err
			}

			accessToken, err := auth.EnsureAccessToken(cmd.Context(), appOpts)
			if err != nil {
				return fmt.Errorf("ensure access token: %w", err)
			}

			return measures.Run(cmd.Context(), opts, appOpts, accessToken)
		},
	}

	measuresCmd.AddCommand(measuresGetCmd)

	addTimeRangeFlags(measuresGetCmd, &opts.TimeRange)
	addPaginationFlags(measuresGetCmd, &opts.Pagination)
	addUserIDFlag(measuresGetCmd, &opts.User)
	addLastUpdateFlag(measuresGetCmd, &opts.LastUpdate)

	measuresGetCmd.Flags().StringVar(
		&opts.Types,
		"type",
		emptyString,
		"measure types (comma-separated)",
	)
	measuresGetCmd.Flags().StringVar(
		&opts.Category,
		"category",
		emptyString,
		"category: real or goal",
	)
	measuresGetCmd.Flags().StringVar(
		&opts.Units,
		"units",
		"metric",
		"unit system: metric or imperial",
	)

	return measuresCmd
}
