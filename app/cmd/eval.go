package cmd

import (
	"github.com/jlewi/cloud-assistant/app/pkg/ai"
	"github.com/jlewi/cloud-assistant/app/pkg/application"
	"github.com/spf13/cobra"
)

func NewEvalCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "eval <yaml-file> <target-url>",
		Short: "Run evaluation using a YAML file and target URL",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := application.NewApp()
			if err := app.LoadConfig(cmd); err != nil {
				return err
			}
			app.Config.CloudAssistant.TargetURL = args[1]
			_, err := ai.EvalFromYAML(args[0], app.Config.CloudAssistant)
			if err != nil {
				return err
			}

			return nil
		},
	}
	return &cmd
}
