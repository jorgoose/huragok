package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/jorgoose/huragok/internal/cliresult"
	"github.com/jorgoose/huragok/internal/create"
	"github.com/jorgoose/huragok/internal/resume"
	"github.com/jorgoose/huragok/internal/runs"
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

			maxCost, _ := cmd.Flags().GetFloat64("max-cost")

			result, runErr := create.Run(cmd.Context(), create.Options{
				Prompt:     prompt,
				OutputPath: outputPath,
				JSON:       jsonOut,
				From:       from,
				MaxCostUSD: maxCost,
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
	createCmd.Flags().Float64("max-cost", 0, "abort before the Hunyuan call if cumulative cost would exceed this many USD (0 = no cap)")

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

			maxCost, _ := cmd.Flags().GetFloat64("max-cost")

			result, runErr := resume.Run(cmd.Context(), resume.Options{
				ParentRunID: args[0],
				Stage:       stage,
				OutputPath:  outputPath,
				JSON:        jsonOut,
				MaxCostUSD:  maxCost,
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
	resumeCmd.Flags().Float64("max-cost", 0, "abort if Hunyuan call would exceed this many USD (0 = no cap)")

	runsCmd := &cobra.Command{
		Use:           "runs",
		Short:         "List previous huragok runs",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")
			list, err := runs.List(".huragok")
			if err != nil {
				return failConfig(err, jsonOut)
			}
			if jsonOut {
				return writeJSON(os.Stdout, list)
			}
			writeRunsTable(os.Stdout, list)
			return nil
		},
	}
	runsCmd.Flags().Bool("json", false, "print structured JSON output to stdout")

	inspectCmd := &cobra.Command{
		Use:           "inspect <run-id>",
		Short:         "Show details and artifacts for a run",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")
			meta, err := runs.Read(".huragok", args[0])
			if err != nil {
				return failConfig(err, jsonOut)
			}
			artifacts, err := runs.ListArtifacts(".huragok", args[0])
			if err != nil {
				return failConfig(err, jsonOut)
			}
			runDir, _ := runs.RunDir(".huragok", args[0])
			absDir, absErr := filepath.Abs(runDir)
			if absErr != nil {
				absDir = runDir
			}
			if jsonOut {
				return writeJSON(os.Stdout, map[string]any{
					"meta":      meta,
					"artifacts": artifactPaths(absDir, artifacts),
				})
			}
			writeInspect(os.Stdout, meta, absDir, artifacts)
			return nil
		},
	}
	inspectCmd.Flags().Bool("json", false, "print structured JSON output to stdout")
	runsCmd.AddCommand(inspectCmd)

	root.AddCommand(createCmd)
	root.AddCommand(resumeCmd)
	root.AddCommand(runsCmd)

	if err := root.Execute(); err != nil {
		var pe *cliresult.PipelineError
		if errors.As(err, &pe) {
			os.Exit(pe.Code)
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(cliresult.ExitConfig)
	}
}

// failConfig prints err to stderr (when not in JSON mode) and returns a
// *cliresult.PipelineError with ExitConfig code. Used by read-only commands
// that don't go through create/resume's finalize() display path.
func failConfig(err error, jsonOut bool) error {
	if !jsonOut {
		fmt.Fprintln(os.Stderr, "Error:", err)
	}
	return &cliresult.PipelineError{Code: cliresult.ExitConfig, Stage: "config", Err: err}
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeRunsTable(w io.Writer, list []runs.Meta) {
	if len(list) == 0 {
		fmt.Fprintln(w, "no runs found in .huragok/runs/")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tCREATED\tSTATUS\tCOST\tPROMPT")
	for _, m := range list {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			m.RunID,
			m.CreatedAt.Local().Format("2006-01-02 15:04"),
			displayStatus(m.Status),
			displayCost(totalCost(m)),
			truncate(displayPrompt(m), 50),
		)
	}
	tw.Flush()
	fmt.Fprintf(w, "\n(%d run%s)\n", len(list), pluralS(len(list)))
}

func writeInspect(w io.Writer, m *runs.Meta, runDir string, artifacts []string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "Run:\t%s\n", m.RunID)
	fmt.Fprintf(tw, "Created:\t%s\n", m.CreatedAt.Local().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(tw, "Status:\t%s\n", displayStatus(m.Status))
	fmt.Fprintf(tw, "Prompt:\t%s\n", displayPrompt(*m))
	if m.OutputPath != "" {
		fmt.Fprintf(tw, "Output:\t%s\n", m.OutputPath)
	}
	if total := totalCost(*m); total > 0 {
		fmt.Fprintf(tw, "Cost:\t%s\n", displayCost(total))
	}
	if m.ImageSource != "" {
		fmt.Fprintf(tw, "Image source:\t%s\n", m.ImageSource)
	}
	if m.ParentRunID != "" {
		fmt.Fprintf(tw, "Parent run:\t%s\n", m.ParentRunID)
	}
	tw.Flush()

	if len(m.Stages) > 0 {
		fmt.Fprintln(w, "\nStages:")
		stw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, name := range []string{"image", "model3d"} {
			s, ok := m.Stages[name]
			if !ok {
				continue
			}
			fmt.Fprintf(stw, "  %s\t%s\t%s\t%s\n",
				name,
				displayStatus(s.Status),
				displayElapsed(s.ElapsedMs),
				displayCost(s.CostEstimateUSD),
			)
		}
		stw.Flush()
	}

	if len(artifacts) > 0 {
		fmt.Fprintf(w, "\nArtifacts (%d):\n", len(artifacts))
		for _, name := range artifacts {
			fmt.Fprintf(w, "  %s\n", filepath.Join(runDir, name))
		}
	}
}

func artifactPaths(runDir string, names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = filepath.Join(runDir, n)
	}
	return out
}

func totalCost(m runs.Meta) float64 {
	var t float64
	for _, s := range m.Stages {
		t += s.CostEstimateUSD
	}
	return t
}

func displayStatus(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func displayCost(c float64) string {
	if c == 0 {
		return "-"
	}
	return fmt.Sprintf("$%.2f", c)
}

func displayPrompt(m runs.Meta) string {
	if m.Prompt != "" {
		return m.Prompt
	}
	if m.ImageSource == "user" {
		return "(--from image)"
	}
	if m.ParentRunID != "" {
		return "(resumed)"
	}
	return "-"
}

func displayElapsed(ms int64) string {
	if ms == 0 {
		return "-"
	}
	return time.Duration(ms * int64(time.Millisecond)).Round(100 * time.Millisecond).String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
