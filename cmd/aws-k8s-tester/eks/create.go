package eks

import "github.com/spf13/cobra"

func newCreate() *cobra.Command {
	ac := &cobra.Command{
		Use:   "create <subcommand>",
		Short: "Create commands",
	}
	ac.AddCommand(
		newCreateConfig(),
		newCreateCluster(),
		newCreateHollowNodes(),
		newCreateCSRs(),
		newCreateConfigMaps(),
		newCreateSecrets(),
		newCreateStresser(),
		newCreateStresserV2(),
		newCreateClusterLoader(),
	)
	return ac
}
