package main

import (
	"errors"
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
		Use:           "create [prompt]",
		Short:         "Generate a 3D model from a text description",
		Args:          cobra.RangeArgs(0, 1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			outputPath, _ := cmd.Flags().GetString("output")
			if outputPath == "" {
				outputPath = "output.glb"
			}
			jsonOut, _ := cmd.Flags().GetBool("json")
			from, _ := cmd.Flags().GetString("from")

			prompt := ""
			if len(args) > 0 {
				prompt = args[0]
			}
			if prompt == "" && from == "" {
				return fmt.Errorf("either a prompt argument or --from <image> is required")
			}
			if prompt != "" && from != "" {
				fmt.Fprintln(os.Stderr, "warning: --from provided, ignoring prompt argument")
				prompt = ""
			}

			result, runErr := create.Run(cmd.Context(), create.Options{
				Prompt:     prompt,
				OutputPath: outputPath,
				JSON:       jsonOut,
				From:       from,
			})

			if jsonOut && result != nil {
				_ = result.WriteJSON(os.Stdout)
			}
			if runErr != nil && jsonOut {
				fmt.Fprintln(os.Stderr, "Error:", runErr)
			}
			return runErr
		},
	}

	createCmd.Flags().StringP("output", "o", "", "output path for the generated .glb file")
	createCmd.Flags().Bool("json", false, "print structured JSON result to stdout")
	createCmd.Flags().String("from", "", "use this image instead of generating one with DALL-E")

	root.AddCommand(createCmd)

	if err := root.Execute(); err != nil {
		var pe *create.PipelineError
		if errors.As(err, &pe) {
			os.Exit(pe.Code)
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(create.ExitConfig)
	}
}
