package eks

import "github.com/spf13/cobra"

func newScale() *cobra.Command {
	ac := &cobra.Command{
		Use:   "scale <subcommand>",
		Short: "Scale commands",
	}
	ac.PersistentFlags().StringVarP(&path, "name", "n", "", "nodegroup name")
	ac.AddCommand(
		newScaleMNG(),
	)
	return ac
}
