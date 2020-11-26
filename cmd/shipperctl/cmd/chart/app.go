package chart

import (
	"github.com/bookingcom/shipper/cmd/shipperctl/config"
	"github.com/spf13/cobra"
)

var (
	appName                  string

	renderAppCmd = &cobra.Command{
		Use:   "app bikerental",
		Short: "render Shipper Charts for an Application",
		RunE:  renderChartFromApp,
		Args: cobra.ExactArgs(1),
	}
)

func init() {
	renderCmd.AddCommand(renderAppCmd)
}

func renderChartFromApp(cmd *cobra.Command, args []string) error {
	appName = args[0]
	c, err := newChartRenderConfig()
	if err != nil {
		return err
	}
	_, shipperClient, err := config.Load(kubeConfigFile, managementClusterContext)
	if err != nil {
		return err
	}
	if err := populateFromApp(shipperClient, &c); err != nil {
		return err
	}

	rendered, err := render(c)
	if err != nil {
		return err
	}

	cmd.Println(rendered)
	return nil
}

