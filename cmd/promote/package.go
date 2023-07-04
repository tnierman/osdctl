package promote

import (
	"github.com/openshift/osdctl/internal/utils/globalflags"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/util"
)

func NewCmdPackage(flags *genericclioptions.ConfigFlags, globalOpts *globalflags.GlobalOptions) *cobra.Command {
	ops := NewPackageOptions()
	cmd := &cobra.Command{
		Use: "package",
		Short: "Utilities to promote package-operator packages",
		Args: cobra.NoArgs,
		DisableAutoGenTag: true,
		Example: `
		# Promote a package
		osdctl promote package --serviceName <service-name> --gitHash <git-hash> --osd
		`,
		RunE: func(cmd *cobra.Command, args []string) error {
			util.CheckErr(ops.promotePackage())
			return nil
		},
	}

	cmd.Flags().StringVarP(&ops.gitHash, "gitHash", "g", "", "Git hash of the package to promote")
	cmd.Flags().StringVarP(&ops.serviceName, "service", "s", "", "Service/Operator getting promoted")
	return cmd
}

type packageOptions struct {
	gitHash string
	serviceName string
}

func NewPackageOptions() packageOptions {
	return packageOptions{}
}

func (p *packageOptions) promotePackage() error {
	return nil
}
