package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/jorgoose/huragok/internal/cliresult"
	"github.com/jorgoose/huragok/internal/create"
	"github.com/jorgoose/huragok/internal/resume"
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

	resumeCmd := &cobra.Command{
		Use:           "resume <run-id>",
		Short:         "Re-run a stage of a previous run, reusing its artifacts",
		Long:          "Re-runs a stage of a previous run against its existing concept image. Currently only --from model3d is supported, which re-calls Hunyuan3D without paying for a new DALL-E image.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			outputPath, _ := cmd.Flags().GetString("output")
			if outputPath == "" {
				outputPath = "output.glb"
			}
			jsonOut, _ := cmd.Flags().GetBool("json")
			stage, _ := cmd.Flags().GetString("from")

			result, runErr := resume.Run(cmd.Context(), resume.Options{
				ParentRunID: args[0],
				Stage:       stage,
				OutputPath:  outputPath,
				JSON:        jsonOut,
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

	resumeCmd.Flags().StringP("output", "o", "", "output path for the regenerated .glb")
	resumeCmd.Flags().Bool("json", false, "print structured JSON result to stdout")
	resumeCmd.Flags().String("from", "model3d", "stage to resume from (currently only model3d)")

	root.AddCommand(createCmd)
	root.AddCommand(resumeCmd)

	if err := root.Execute(); err != nil {
		var pe *cliresult.PipelineError
		if errors.As(err, &pe) {
			os.Exit(pe.Code)
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(cliresult.ExitConfig)
	}
}
