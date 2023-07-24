package report

import (
	"github.com/openshift/osdctl/cmd/report/analyze"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewCmdReport(streams genericclioptions.IOStreams, flags *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use: "report",
		Short: "Interact with SRE weekly reports",
		Long: "Perform operations and analyze trends within the SRE weekly report",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	cmd.AddCommand(analyze.NewCommandAnalyze(streams, flags))

	return cmd
}
