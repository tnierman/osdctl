package analyze

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/fatih/color"
	accountsmgmtv1 "github.com/openshift-online/ocm-sdk-go/accountsmgmt/v1"
	clustersmgmtv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/osdctl/pkg/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewCmdOrg(streams genericclioptions.IOStreams, flags *genericclioptions.ConfigFlags) *cobra.Command {
	analyzer := NewOrgAnalyzer(streams)
	cmd := &cobra.Command{
		Use: "org <report file path>",
		Aliases: []string{"orgs", "organization", "organizations"},
		Short: "Analyze noise by organization",
		Long: "Perform analysis on SRE weekly report raw data. Report should be provided as a .csv file from the 'Incident's Details' tab",
		DisableAutoGenTag: true,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			err := validateAnalyzer(analyzer)
			if err != nil {
				return fmt.Errorf("invalid argument provided: %w", err)
			}
			err = analyzer.Analyze(args[0])
			if err != nil {
				return fmt.Errorf("encountered unrecoverable error while analyzing file '%s': %w", args[0], err)
			}
			err = analyzer.Summarize()
			if err != nil {
				return fmt.Errorf("unable to summarize statistics: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&analyzer.Output, "output", "o", "long", "Specify how the results are displayed. Options are 'short', 'yaml', 'json', 'long'. If not specified, 'long' is used")
	cmd.Flags().IntVarP(&analyzer.NumOrgs, "number", "n", 5, "Specify the number of organizations displayed when summarizing data. Values <= 0 indicate that all results should be displayed. Only used when '--output/-o' is 'short' or 'long' ('json' and 'yaml' will always print the full analysis). Default is 5")
	cmd.Flags().StringVar(&analyzer.SearchRegex, "search", "", "Specify a pattern to match against. Patterns are validated using https://github.com/google/re2/wiki/Syntax. Only organizations whose name or ID matches the provided pattern will be included in the final results")
	return cmd
}

func validateAnalyzer(o OrgAnalyzer) error {
	switch o.Output {
	case "yaml":
	case "json":
	case "short":
	case "long":
	default:
		return fmt.Errorf("invalid output format provided with '--output/-o'. Valid options are one of 'short', 'long', 'yaml', or 'json'")
	}

	return nil
}

type OrgAnalyzer struct {
	// OrganizationStatsList records the current statistics for each organization
	OrganizationStatsList `json:"stats" yaml:"stats"`
	// SkippedEntries tracks the number of report entries which will not be included in the final tally.
	// These include entries whose clusters were deleted, org could not be found, contained malformed data, etc
	SkippedEntries int `json:"skippedEntries" yaml:"skippedEntries"`
	// TotalEntries tracks the total number of report entries which were analyzed.
	// This figure includes those entries which were skipped
	TotalEntries int `json:"totalEntries" yaml:"totalEntries"`

	// Flags

	// Output specifies the output type
	// Only 'yaml', 'json', 'short' are valid
	Output string
	// NumOrgs defines the number of organizations which should be summarized. (ie - if numOrgs = 3, then
	// the top 3 organizations will be displayed when calling Summarize() ). This value is only used when
	// the output = 'short'
	NumOrgs int
	// SearchRegex defines the search pattern applied to orgs. Any orgs' statistics not matching the pattern will not be added to the Analyzer
	// Regex patterns must conform to the following conventions: https://github.com/google/re2/wiki/Syntax
	SearchRegex string

	// Streams
	genericclioptions.IOStreams `json:"-" yaml:"-"`
}

//func NewOrgAnalyzer(streams genericclioptions.IOStreams, flags *genericclioptions.ConfigFlags) OrgAnalyzer {
func NewOrgAnalyzer(streams genericclioptions.IOStreams) OrgAnalyzer {
	o := OrgAnalyzer{
		OrganizationStatsList: NewOrganizationStatsList(),
		IOStreams: streams,
	}
	return o
}

// Println appends a newline then prints the given msg using the OrgAnalyzer's IOStreams
func (o OrgAnalyzer) Println(msg string) {
	utils.StreamPrintln(o.IOStreams, msg)
}

// Print prints the given msg using the OrgAnalyzer's IOStreams
func (o OrgAnalyzer) Print(msg string) {
	utils.StreamPrint(o.IOStreams, msg)
}

// Errorln appends a newline then prints the given error msg using the OrgAnalyzer's IOStreams
func (o OrgAnalyzer) Errorln(msg string) {
	utils.StreamErrorln(o.IOStreams, msg)
}

// Analyze ingests the provided report, adding its data to the OrgAnalyzer's statistics
// If defined, only those orgs whose name or ID matches the OrgAnalyzer's SearchRegex pattern
// have their metrics ingested
func (o *OrgAnalyzer) Analyze(reportFilePath string) error {
	report, err := o.openReport(reportFilePath)
	if err != nil {
		return fmt.Errorf("failed to open report file '%s': %w", reportFilePath, err)
	}

	ocmClient, err := utils.CreateConnection()
	if err != nil {
		return fmt.Errorf("failed to create ocm client: %w", err)
	}

	// Skip header entry
	_, err = report.Read()
	if err != nil {
		o.Errorln(fmt.Sprintf("failed to read header entry: %v", err))
	}
	for {
		entry, err := report.Read()
		if err == io.EOF {
			// Done reading
			break
		}
		o.TotalEntries++
		if err != nil {
			o.Errorln(fmt.Sprintf("failed to read entry: %v\nSkipping.", err))
			o.SkippedEntries++
			continue
		}

		clusterID, alert := o.parseEntry(entry)
		cluster, err := utils.GetCluster(ocmClient, clusterID)
		if err != nil {
			o.Errorln(fmt.Sprintf("failed to retrieve cluster '%s' from OCM: %v\nSkipping.", clusterID, err))
			o.SkippedEntries++
			continue
		}
		orgID, err := utils.GetOrgIDFromCluster(ocmClient, *cluster)
		if err != nil {
			o.Errorln(fmt.Sprintf("failed to retrieve organization ID for cluster '%s' from OCM: %v", cluster.ID(), err))
			o.SkippedEntries++
			continue
		}
		org, err := utils.GetOrgFromID(ocmClient, orgID)
		if err != nil {
			o.Errorln(fmt.Sprintf("failed to retrieve organization with ID '%s' from OCM: %v", orgID, err))
			o.SkippedEntries++
			continue
		}
		if o.SearchRegex != "" {
			matchName, err := regexp.MatchString(o.SearchRegex, org.Name())
			if err != nil {
				o.Errorln(fmt.Sprintf("failed to compare organization name '%s' to provided search string '%s': %v", org.Name(), o.SearchRegex, err))
				o.SkippedEntries++
				continue
			}

			matchID, err := regexp.MatchString(o.SearchRegex, org.ID())
			if err != nil {
				o.Errorln(fmt.Sprintf("failed to compare organization ID '%s' to provided search string '%s': %v", org.ID(), o.SearchRegex, err))
				o.SkippedEntries++
				continue
			}
			if !matchName && !matchID {
				// Do not index this as a SkippedEntry: we know that this cluster belongs to an org which the user wishes to ignore.
				continue
			}
		}
		o.AddStatistic(org, *cluster, alert)
	}
	return nil
}

// getReportData opens the analyzer's reportFile and reads the file's entire contents
// so that it does not need to be closed after use
func (o OrgAnalyzer) openReport(reportFilePath string) (*csv.Reader, error) {
	data, err := os.ReadFile(reportFilePath)
	if err != nil {
		return &csv.Reader{}, err
	}

	reader := csv.NewReader(strings.NewReader(string(data)))
	return reader, nil
}

// parseEntry retrieves the clusterID and alert from a report entry
func (o OrgAnalyzer) parseEntry(entry []string) (clusterID string, alert string) {
	alert = entry[6]
	clusterID = entry[5]
	return clusterID, alert
}

// Summarize prints the OrgAnalyzer's results. The format returned is based on the OrgAnalyzer's .output field.
// Data is sent via the OrgAnalyzer's IOStreams
func (o OrgAnalyzer) Summarize() error {
	switch o.Output {
	case "yaml":
		err := o.printYamlSummary()
		if err != nil {
			return fmt.Errorf("failed to print yaml summary: %v", err)
		}
	case "json":
		err := o.printJsonSummary()
		if err != nil {
			return fmt.Errorf("failed to print json summary: %v", err)
		}
	case "short":
		o.printShortSummary()
	case "long":
		o.printLongSummary()
	default:
		return fmt.Errorf("Invalid output format requested")
	}
	return nil
}

func (o OrgAnalyzer) printShortSummary() {
	color.Set(color.Bold)
	o.Println("Results")
	o.Println("")
	color.Unset()

	skipPercentage := o.CalculateSkipPercentage()
	o.Println(fmt.Sprintf("%d out of %d total entries (%.2f percent) were skipped", o.SkippedEntries, o.TotalEntries, skipPercentage))
	// This is just an arbitrary threshold
	if skipPercentage > 0.3 {
		o.Errorln(color.RedString("High skip percentage detected: results may be skewed"))
	}
	topOrgs := o.TopOrgs(o.NumOrgs)
	for _, org := range topOrgs {
		o.Println(fmt.Sprintf("%s: %s [%s]", color.GreenString("Organization"), org.Organization.Name(), org.Organization.ID()))
		o.Println(fmt.Sprintf("\t%s: %d across %d cluster(s)\n", color.BlueString("Total Alerts"), org.TotalAlerts, len(org.Clusters)))
	}
}

func (o OrgAnalyzer) printLongSummary() {
	color.Set(color.Bold)
	o.Println("Results")
	o.Println("")
	color.Unset()

	skipPercentage := o.CalculateSkipPercentage()
	o.Println(fmt.Sprintf("%d out of %d total entries (%.2f percent) were skipped", o.SkippedEntries, o.TotalEntries, skipPercentage))
	// This is just an arbitrary threshold
	if skipPercentage > 0.3 {
		o.Errorln(color.RedString("High skip percentage detected: results may be skewed"))
	}
	topOrgs := o.TopOrgs(o.NumOrgs)
	for _, org := range topOrgs {
		o.Println(fmt.Sprintf("%s: %s [%s]", color.GreenString("Organization"), org.Organization.Name(), org.Organization.ID()))
		o.Println(fmt.Sprintf("\t%s: %s", color.GreenString("EBS Account ID"), org.Organization.EbsAccountID()))
		o.Println(fmt.Sprintf("\t%s: %d across %d cluster(s)", color.BlueString("Total Alerts"), org.TotalAlerts, len(org.Clusters)))
		o.Println(fmt.Sprintf("\t- %.2f percent of all alerts", o.CalculatePercentageOfTotal(org.TotalAlerts)))
		o.Println(fmt.Sprintf("\t- %.2f percent of analyzed (not skipped) alerts", o.CalculatePercentageOfAnalyzed(org.TotalAlerts)))
		o.Println(fmt.Sprintf("\t%s:", color.GreenString("Clusters")))
		for _, cluster := range org.Clusters {
			o.Println(fmt.Sprintf("\t- %s // %s", color.BlueString(cluster.Cluster.Name()), cluster.Cluster.ID()))
			o.Println(fmt.Sprintf("\t  %d alerts across %d symptoms", cluster.TotalAlerts, len(cluster.Alerts)))
			o.Println(fmt.Sprintf("\t\t* %.2f percent of all alerts", o.CalculatePercentageOfTotal(cluster.TotalAlerts)))
			o.Println(fmt.Sprintf("\t\t* %.2f percent of analyzed (not skipped) alerts", o.CalculatePercentageOfAnalyzed(cluster.TotalAlerts)))
			o.Println(fmt.Sprintf("\t  %s:", color.GreenString("Alerts")))
			for alert, count:= range cluster.Alerts {
				o.Println(fmt.Sprintf("\t\t* %s -- %d", alert, count))
			}
		}
		o.Println("")
	}
}

func (o OrgAnalyzer) printYamlSummary() error {
	out, err := yaml.Marshal(&o)
	if err != nil {
		return err
	}
	o.Println(fmt.Sprintf("%s", out))
	return nil
}

func (o OrgAnalyzer) printJsonSummary() error {
	out, err := json.Marshal(&o)
	if err != nil {
		return err
	}
	o.Println(fmt.Sprintf("%s", out))
	return nil
}

func (o OrgAnalyzer) CalculateSkipPercentage() float32 {
	return (float32(o.SkippedEntries) / float32(o.TotalEntries)) * 100
}

func (o OrgAnalyzer) CalculatePercentageOfTotal(count int) float32 {
	return (float32(count)/float32(o.TotalEntries)) * 100
}

func (o OrgAnalyzer) CalculatePercentageOfAnalyzed(count int) float32 {
	analyzedAlerts := o.TotalEntries - o.SkippedEntries
	return (float32(count)/float32(analyzedAlerts)) * 100
}

type OrganizationStatsList struct {
	Items map[string]OrganizationStats `json:"organizations" yaml:"organizations"`
}

func NewOrganizationStatsList() OrganizationStatsList {
	o := OrganizationStatsList{
		Items: map[string]OrganizationStats{},
	}
	return o
}

// AddStatistic adds the provided statistics to the proper organization within OrganizationStatsList
func (o *OrganizationStatsList) AddStatistic(org accountsmgmtv1.Organization, cluster clustersmgmtv1.Cluster, alert string) {
	orgStats, found := o.Items[org.ID()]
	if !found {
		orgStats = NewOrganizationStats(org)
	}
	orgStats.AddStatistic(cluster, alert)
	o.Items[org.ID()] = orgStats
}

// TopOrgs returns a slice containing the Organizations with the most alerts in the
// OrganizationStatsList in reverse order (sorry).
// If the provided numOrgs is <= 0, then all organizations are returned
func (o OrganizationStatsList) TopOrgs(numOrgs int) []OrganizationStats {
	stats := []OrganizationStats{}
	for _, orgStats := range o.Items {
		stats = append(stats, orgStats)
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].TotalAlerts < stats[j].TotalAlerts {
			return true
		}
		return false
	})
	if numOrgs <= 0 || numOrgs > len(stats) {
		return stats
	}
	return stats[len(stats)-numOrgs:]
}

type OrganizationStats struct {
	Organization accountsmgmtv1.Organization      `json:"organization" yaml:"organization"`
	TotalAlerts int                  `json:"totalAlerts" yaml:"totalAlerts"`
	Clusters map[string]ClusterStats `json:"clusters" yaml:"clusters"`
}

func NewOrganizationStats(org accountsmgmtv1.Organization) OrganizationStats {
	o := OrganizationStats{
		Organization: org,
		TotalAlerts: 0,
		Clusters: map[string]ClusterStats{},
	}
	return o
}

func (o *OrganizationStats) AddStatistic(cluster clustersmgmtv1.Cluster, alert string) {
	o.TotalAlerts++
	clusterStats, found := o.Clusters[cluster.ID()]
	if !found {
		clusterStats = NewClusterStats(cluster)
	}
	clusterStats.AddStatistic(alert)
	o.Clusters[cluster.ID()] = clusterStats
}

func (o OrganizationStats) TopClusters(numClusters int) []ClusterStats {
	stats := []ClusterStats{}
	for _, clusterStats := range o.Clusters {
		stats = append(stats, clusterStats)
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].TotalAlerts < stats[j].TotalAlerts {
			return true
		}
		return false
	})
	return stats[len(stats)-numClusters:]
}

type ClusterStats struct {
	Cluster clustersmgmtv1.Cluster `json:"cluster" yaml:"cluster"`
	TotalAlerts int        `json:"totalAlerts" yaml:"totalAlerts"`
	Alerts AlertStats      `json:"alerts" yaml:"alerts"`
}

type AlertStats map[string]int

func NewClusterStats(cluster clustersmgmtv1.Cluster) ClusterStats {
	c := ClusterStats {
		Cluster: cluster,
		Alerts: AlertStats{},
	}
	return c
}

func (c *ClusterStats) AddStatistic(alertName string) {
	c.TotalAlerts++
	_, found := c.Alerts[alertName]
	if !found {
		c.Alerts[alertName] = 1
		return
	}
	c.Alerts[alertName]++
}
