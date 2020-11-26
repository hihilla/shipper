package chart

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/bookingcom/shipper/cmd/shipperctl/config"
	"github.com/bookingcom/shipper/cmd/shipperctl/configurator"
	shipper "github.com/bookingcom/shipper/pkg/apis/shipper/v1alpha1"
	shipperchart "github.com/bookingcom/shipper/pkg/chart"
	"github.com/bookingcom/shipper/pkg/chart/repo"
	shipperclientset "github.com/bookingcom/shipper/pkg/client/clientset/versioned"
)

var (
	namespace                string
	kubeConfigFile           string
	managementClusterContext string

	chartName    string
	chartVersion string
	chartRepoUrl string
	values       string

	Command = &cobra.Command{
		Use:   "chart",
		Short: "operate on Shipper Charts",
	}

	renderCmd = &cobra.Command{
		Use:   "render",
		Short: "render Helm Charts for an Application or Release",
		RunE:  renderChart,
	}
)

type ChartRenderConfig struct {
	ChartSpec   shipper.Chart
	ChartValues *shipper.ChartValues
	Namespace   string
	ReleaseName string
}

func init() {
	const kubeConfigFlagName = "kubeconfig"
	config.RegisterFlag(Command.PersistentFlags(), &kubeConfigFile)
	if err := Command.MarkPersistentFlagFilename(kubeConfigFlagName, "yaml"); err != nil {
		Command.Printf("warning: could not mark %q for filename autocompletion: %s\n", kubeConfigFlagName, err)
	}
	Command.PersistentFlags().StringVar(&managementClusterContext, "management-cluster-context", "", "The name of the context to use to communicate with the management cluster. defaults to the current one")
	Command.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "namespace where the app lives")

	renderCmd.Flags().StringVar(&chartName, "chart-name", "", "The name of the chart as specified in the application manifest")
	if err := renderCmd.MarkFlagRequired("chart-name"); err != nil {
		renderCmd.Printf("warning: could not mark flag %q as required: %s\n", "chart-name", err)
	}
	renderCmd.Flags().StringVar(&chartVersion, "chart-version", "", "The version of the chart as specified in the application manifest (semver are accepted)")
	if err := renderCmd.MarkFlagRequired("chart-version"); err != nil {
		renderCmd.Printf("warning: could not mark flag %q as required: %s\n", "chart-version", err)
	}
	renderCmd.Flags().StringVar(&chartRepoUrl, "chart-repo", "", "The repository URL of the chart as specified in the application manifest")
	if err := renderCmd.MarkFlagRequired("chart-repo"); err != nil {
		renderCmd.Printf("warning: could not mark flag %q as required: %s\n", "chart-repo", err)
	}

	renderCmd.Flags().StringVar(&values, "values", values, "Values to apply to the chart in a JSON format")

	Command.AddCommand(renderCmd)
}

func renderChart(cmd *cobra.Command, args []string) error {
	c, err := newChartRenderConfig()
	if err != nil {
		return err
	}
	if err := populateFromFlags(&c); err != nil {
		return err
	}

	cmd.Printf("Here is the chart config: \n%v\n", c)
	cmd.Printf("Here is the chart values: \n%v\n", c.ChartValues)

	rendered, err := render(c)
	if err != nil {
		return err
	}

	cmd.Println(rendered)
	return nil
}

func populateFromFlags(c *ChartRenderConfig) error {
	chartSpec, err := chartSpec()
	if err != nil {
		return err
	}
	c.ChartSpec = chartSpec

	chartValues := shipper.ChartValues{}
	if err := yaml.Unmarshal(bytes.NewBufferString(values).Bytes(), &chartValues); err != nil {
		return err
 	}

	c.ChartValues = &chartValues
	c.ReleaseName = "foobar-foobar-0"
	return nil
}

func chartSpec() (shipper.Chart, error) {
	chartSpec := shipper.Chart{
		Name:    chartName,
		Version: chartVersion,
		RepoURL: chartRepoUrl,
	}
	chartResolver := newResolveChartVersionFunc()
	cv, err := chartResolver(&chartSpec)
	if err != nil {
		return chartSpec, err
	}
	chartSpec.Version = cv.Version
	return chartSpec, nil
}

func render(c ChartRenderConfig) (string, error) {
	chartFetcher := newFetchChartFunc()
	chart, err := chartFetcher(&c.ChartSpec)
	if err != nil {
		return "", err
	}

	rendered, err := shipperchart.Render(
		chart,
		c.ReleaseName,
		c.Namespace,
		c.ChartValues,
	)
	if err != nil {
		return "", err
	}

	return strings.Join(rendered, "%s\n---\n"), nil
}

func newChartRenderConfig() (ChartRenderConfig, error) {
	c := ChartRenderConfig{
	}
	clientConfig, _, err := configurator.ClientConfig(kubeConfigFile, managementClusterContext)
	if err != nil {
		return c, err
	}
	if namespace == "" {
		namespace, _, err = clientConfig.Namespace()
		if err != nil {
			return c, err
		}
	}
	c.Namespace = namespace

	return c, nil
}

func populateFormRelease(shipperClient shipperclientset.Interface, c *ChartRenderConfig) error {
	rel, err := shipperClient.ShipperV1alpha1().Releases(namespace).Get(releaseName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	c.ReleaseName = releaseName
	c.ChartSpec = rel.Spec.Environment.Chart
	c.ChartValues = rel.Spec.Environment.Values
	return nil
}

func populateFromApp(shipperClient shipperclientset.Interface, c *ChartRenderConfig) error {
	app, err := shipperClient.ShipperV1alpha1().Applications(namespace).Get(appName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	c.ReleaseName = fmt.Sprintf("%s-%s-%d", appName, "foobar", 0)
	c.ChartSpec = app.Spec.Template.Chart
	c.ChartValues = app.Spec.Template.Values
	return nil
}

func newFetchChartFunc() repo.ChartFetcher {
	stopCh := make(<-chan struct{})

	repoCatalog := repo.NewCatalog(
		repo.DefaultFileCacheFactory(filepath.Join(os.TempDir(), "chart-cache")),
		repo.DefaultRemoteFetcher,
		stopCh)

	return repo.FetchChartFunc(repoCatalog)
}

func newResolveChartVersionFunc() repo.ChartVersionResolver {
	stopCh := make(<-chan struct{})

	repoCatalog := repo.NewCatalog(
		repo.DefaultFileCacheFactory(filepath.Join(os.TempDir(), "chart-cache")),
		repo.DefaultRemoteFetcher,
		stopCh)

	return repo.ResolveChartVersionFunc(repoCatalog)
}
