package cmd

import (
	"fmt"

	"github.com/jlewi/cloud-assistant/app/pkg/ai"
	"github.com/jlewi/cloud-assistant/app/pkg/application"
	"github.com/spf13/cobra"
)

func NewEvalCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "eval <proto-file> <target-url>",
		Short: "Run evaluation using a proto file and target URL",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := application.NewApp()
			if err := app.LoadConfig(cmd); err != nil {
				return err
			}
			if app.Config.CloudAssistant == nil {
				return fmt.Errorf("CloudAssistant config is nil")
			}
			app.Config.CloudAssistant.TargetURL = args[1]
			blocks, err := ai.EvalFromProto(args[0], app.Config.CloudAssistant)
			if err != nil {
				return err
			}
			fmt.Println("\nBlocks received:")
			for _, block := range blocks {
				fmt.Printf("block id: %s contents: %s\n", block.Id, block.Contents)
			}
			fmt.Println("\nBlocks:")
			fmt.Println(blocks)
			return nil
		},
	}
	return &cmd
}
