package analyze

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewCommandAnalyze(streams genericclioptions.IOStreams, flags *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use: "analyze",
		Short: "Analyze the SRE weekly report",
		Long: "Perform analysis on the SRE weekly reports",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	cmd.AddCommand(NewCmdOrg(streams, flags))
	return cmd
}
