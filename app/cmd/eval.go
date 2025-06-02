package cmd

import (
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
			app.Config.CloudAssistant.TargetURL = args[1]
			blocks, err := ai.EvalFromProto(args[0], app.Config.CloudAssistant)
			if err != nil {
				return err
			}
			cmd.Println("\nBlocks received:")
			for _, block := range blocks {
				cmd.Printf("block id: %s contents: %s\n", block.Id, block.Contents)
			}
			cmd.Println("\nBlocks:")
			cmd.Println(blocks)
			return nil
		},
	}
	return &cmd
}
