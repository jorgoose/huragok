package main

import (
	"fmt"
	"os"

	"github.com/jorgoose/huragok/internal/create"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "huragok",
		Short: "AI-powered 3D asset generation",
		Long:  "huragok wraps the workflow of going from a text description to a game-ready 3D model into a single CLI pipeline.",
	}

	createCmd := &cobra.Command{
		Use:   "create [prompt]",
		Short: "Generate a 3D model from a text description",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			output, _ := cmd.Flags().GetString("output")
			if output == "" {
				output = "output.glb"
			}
			return create.Run(cmd.Context(), args[0], output)
		},
	}

	createCmd.Flags().StringP("output", "o", "", "output path for the generated .glb file")

	root.AddCommand(createCmd)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
