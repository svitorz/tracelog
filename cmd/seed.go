package cmd

import "github.com/spf13/cobra"

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed optional generic data",
	Long:  "Seed optional generic data for the local MVP. Instance-specific setup must be done with `tracelog init`.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Println("[seed] No optional generic seed data is defined for the current MVP.")
		cmd.Println("[seed] Use `tracelog init` to configure the local application, environment, and event source.")
		return nil
	},
}

func init() {
	seedCmd.Example = `  # Seed optional generic data
  tracelog seed`

	rootCmd.AddCommand(seedCmd)
}
