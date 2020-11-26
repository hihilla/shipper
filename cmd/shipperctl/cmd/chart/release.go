package chart

import (
	"github.com/bookingcom/shipper/cmd/shipperctl/config"
	"github.com/spf13/cobra"
)

var (
	releaseName string

	renderRelCmd = &cobra.Command{
		Use:   "release bikerental-7abf46d4-0",
		Short: "render Shipper Charts for a Release",
		RunE:  renderChartFromRel,
		Args: cobra.ExactArgs(1),
	}
)

func init() {
	renderCmd.AddCommand(renderRelCmd)
}

func renderChartFromRel(cmd *cobra.Command, args []string) error {
	releaseName = args[0]
	c, err := newChartRenderConfig()
	if err != nil {
		return err
	}
	_, shipperClient, err := config.Load(kubeConfigFile, managementClusterContext)
	if err != nil {
		return err
	}

	if err := populateFormRelease(shipperClient, &c); err != nil {
		return err
	}

	rendered, err := render(c)
	if err != nil {
		return err
	}

	cmd.Println(rendered)
	return nil
}
