package cmd

import (
	"fmt"

	"github.com/jlewi/cloud-assistant/app/pkg/ai"
	"github.com/spf13/cobra"
)

func NewEvalCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:   "eval",
		Short: "Start evaluation",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := ai.AgentInference("placeholder")
			if err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	}
	return &cmd
}
