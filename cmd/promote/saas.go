package promote

import (
	"fmt"
	"os"
	"sort"

	"github.com/openshift/osdctl/cmd/promote/git"
	"github.com/openshift/osdctl/internal/utils/globalflags"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type saasOptions struct {
	list bool
	osd  bool
	hcp  bool

	serviceName string
	gitHash     string
}

// newCmdSaas implementes the saas command to interact with promoting SaaS services/operators
func NewCmdSaas(flags *genericclioptions.ConfigFlags, globalOpts *globalflags.GlobalOptions) *cobra.Command {
	ops := newSaasOptions(flags, globalOpts)
	saasCmd := &cobra.Command{
		Use:               "saas",
		Short:             "Utilities to promote SaaS services/operators",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		Example: `
		# List all SaaS services/operators
		osdctl promote saas --list

		# Promote a SaaS service/operator
		osdctl promote saas --serviceName <service-name> --gitHash <git-hash> --osd
		or
		osdctl promote saas --serviceName <service-name> --gitHash <git-hash> --hcp`,
		Run: func(cmd *cobra.Command, args []string) {
			ops.validateSaasFlow()
			git.BootstrapOsdCtlForAppInterfaceAndServicePromotions()

			if ops.list {
				if ops.serviceName != "" || ops.gitHash != "" || ops.osd || ops.hcp {
					fmt.Printf("Error: --list cannot be used with any other flags\n\n")
					cmd.Help()
					os.Exit(1)
				}
				listServiceNames()
				os.Exit(0)
			}

			if !(ops.osd || ops.hcp) && ops.serviceName != "" {
				fmt.Printf("Error: --serviceName cannot be used without either --osd or --hcp\n\n")
				cmd.Help()
				os.Exit(1)
			}

			err := servicePromotion(ops.serviceName, ops.gitHash, ops.osd, ops.hcp)
			if err != nil {
				fmt.Printf("Error while promoting service: %v\n", err)
				os.Exit(1)
			}

			os.Exit(0)

		},
	}

	saasCmd.Flags().BoolVarP(&ops.list, "list", "l", false, "List all SaaS services/operators")
	saasCmd.Flags().StringVarP(&ops.serviceName, "serviceName", "", "", "SaaS service/operator getting promoted")
	saasCmd.Flags().StringVarP(&ops.gitHash, "gitHash", "g", "", "Git hash of the SaaS service/operator commit getting promoted")
	saasCmd.Flags().BoolVarP(&ops.osd, "osd", "", false, "OSD service/operator getting promoted")
	saasCmd.Flags().BoolVarP(&ops.hcp, "hcp", "", false, "Git hash of the SaaS service/operator commit getting promoted")

	return saasCmd
}

func newSaasOptions(flags *genericclioptions.ConfigFlags, globalOpts *globalflags.GlobalOptions) *saasOptions {
	return &saasOptions{}
}

func (o *saasOptions) validateSaasFlow() {
	if o.serviceName == "" || o.gitHash == "" {
		fmt.Printf("Usage: For SaaS services/operators, please provide --serviceName and --gitHash\n")
		fmt.Printf("--serviceName is the name of the service, i.e. saas-managed-cluster-config\n")
		fmt.Printf("--gitHash is the target git commit in the service, if not specified defaults to HEAD of master\n\n")
		return
	}
}

func listServiceNames() error {
	services, err := GetServiceNames(OSDSaasDir, BPSaasDir, CADSaasDir)
	if err != nil {
		return err
	}

	sort.Strings(services)
	fmt.Println("### Available service names ###")
	for _, service := range services {
		fmt.Println(service)
	}

	return nil
}

func servicePromotion(serviceName, gitHash string, osd, hcp bool) error {
	services, err := GetServiceNames(OSDSaasDir, BPSaasDir, CADSaasDir)
	if err != nil {
		return err
	}

	err = validateServiceName(services, serviceName)
	if err != nil {
		return err
	}

	saasDir, err := getSaasDir(serviceName, osd, hcp)
	if err != nil {
		return err
	}
	fmt.Printf("SAAS Directory: %v\n", saasDir)

	serviceData, err := os.ReadFile(saasDir)
	if err != nil {
		return fmt.Errorf("failed to read SAAS file: %v", err)
	}

	currentGitHash, serviceRepo, err := git.GetCurrentGitHashFromAppInterface(serviceData, serviceName)
	if err != nil {
		return fmt.Errorf("failed to get current git hash or service repo: %v", err)
	}
	fmt.Printf("Current Git Hash: %v\nGit Repo: %v\n\n", currentGitHash, serviceRepo)

	promotionGitHash, err := git.CheckoutAndCompareGitHash(serviceRepo, gitHash, currentGitHash)
	if err != nil {
		return fmt.Errorf("failed to checkout and compare git hash: %v", err)
	} else if promotionGitHash == "" {
		fmt.Printf("Unable to find a git hash to promote. Exiting.\n")
		os.Exit(6)
	}
	fmt.Printf("Service: %s will be promoted to %s\n", serviceName, promotionGitHash)

	err = git.UpdateAndCommitChangesForAppInterface(serviceName, saasDir, currentGitHash, promotionGitHash)
	if err != nil {
		fmt.Printf("FAILURE: %v\n", err)
	}

	return nil
}