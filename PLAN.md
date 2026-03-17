# huragok — Design & Specification

## Purpose

huragok exists to solve a specific problem: **turning a text description into a game-ready 3D model is currently a fragmented, manual, non-reproducible process that can't be automated.**

Today, generating a 3D asset from scratch means juggling multiple disconnected tools — write a prompt in ChatGPT, download the image, upload it to a 3D generation service, download the mesh, open it in Blender, decimate it, fix the UVs, export to GLB. Every step is manual. Nothing is tracked. If you want to tweak the result, you start over. And if you want an AI agent to do it for you, it can't — there's no single tool it can call.

huragok wraps this entire workflow into a single CLI pipeline that:

1. **Works interactively** for a human at a terminal who wants to review and approve each step
2. **Works headlessly** for an AI agent (Claude Code) or CI pipeline that needs to generate assets programmatically
3. **Tracks everything** so you can resume, retry, reproduce, and audit any generation run
4. **Abstracts providers** so you're never locked into one image or 3D generation service

The goal is not to replace 3D artists. It's to make it possible for solo developers, small teams, and AI-assisted workflows to generate functional 3D assets without specialized modeling skills or expensive outsourcing.

---

## Core design principles

### CLI-first, UI-second

The CLI is the execution layer and the source of truth. Every operation can be performed from the command line. The review UI is an optional, read-only visual layer on top.

**Why:** The primary consumer of huragok is Claude Code, which invokes tools via bash. A GUI-only tool can't be automated. By making the CLI the core interface, we guarantee that everything is scriptable, composable, and agent-friendly. The review UI exists because you can't rotate a 3D model in a terminal — but it never modifies state, it only reads it.

### Staged pipeline with checkpoints

The pipeline is broken into discrete stages (prompt → image → 3D → post-process). Each stage writes its output to disk and pauses for review before advancing. Any stage can be skipped, retried, or swapped to a different provider without affecting other stages.

**Why:** 3D generation API calls are expensive (money) and slow (time). If stage 3 produces a bad mesh, you need to retry just stage 3 — not regenerate the concept art you already approved. Checkpoints also let humans catch problems early: if the concept image looks wrong, don't waste a 3D generation call on it.

### Provider-agnostic

Each pipeline stage has a pluggable provider. The prompt stage can use OpenAI, Anthropic, or any LLM. The image stage can use OpenAI, Stability, or manual upload. The 3D stage can use Hunyuan3D, Meshy, Tripo, or others.

**Why:** The generative 3D space is evolving fast. The best provider today might not be the best provider in three months. Hard-coding to Hunyuan3D (or any single provider) creates vendor lock-in in a market where quality leadership changes quarterly. The provider abstraction costs very little to implement but saves us from painful rewrites later. It also lets users A/B test providers — generate the same asset on two providers and pick the better result.

### Run persistence and reproducibility

Every `huragok create` invocation produces a "run" — a directory containing all intermediate artifacts, the configuration used, provider responses, timing, and costs. Runs are immutable records of what happened.

**Why:** Without run persistence, you can't answer basic questions like "what prompt produced this model?" or "how much did I spend this week?" You also can't resume a failed run or retry a single stage. Run persistence is what makes the tool usable over time rather than just a one-shot command.

### Cost awareness as a first-class concern

Every provider adapter reports estimated costs. The pipeline tracks cumulative cost per run. Budgets can be enforced. Cost summaries are available per run and across time ranges.

**Why:** Unlike most dev tools, huragok calls paid APIs on every invocation. A careless batch script or a misconfigured `--auto` run can burn through budget quickly. Cost tracking isn't a nice-to-have — it's a safety mechanism. In headless/agent mode especially, the tool must respect cost ceilings and fail loudly rather than silently overspend.

---

## Architecture

### Why Go

huragok is a CLI tool that orchestrates API calls, manages files, and occasionally does CPU-bound mesh processing. The language choice was evaluated against three candidates:

**Go wins on distribution.** A single static binary with zero runtime dependencies. No "install Node 18+" prerequisite, no virtual environments, no node_modules. Users download one file, put it on PATH, and it works. For a tool meant to be used by other developers and invoked by AI agents, frictionless installation is critical. `go install github.com/jorgoose/huragok@latest` or download from GitHub releases.

**Go wins on startup time.** CLI tools should feel instant. Go binaries start in single-digit milliseconds. Node.js CLI tools pay a 300-500ms boot tax on every invocation just to parse imports. For commands like `huragok runs` or `huragok config` that should return immediately, this matters. The main pipeline (`huragok create`) spends minutes waiting on APIs so startup time is negligible there — but the quick-lookup commands need to feel snappy.

**Go wins on CLI ecosystem.** cobra + viper is the gold standard for CLI tools. kubectl, docker, gh, terraform, hugo — the entire modern CLI ecosystem is built on Go. The patterns, libraries, and community knowledge for building CLI tools in Go are unmatched.

**Go wins on the Tencent integration.** Tencent Cloud has an official Go SDK (`tencentcloud-sdk-go`) that handles their HMAC-SHA256 request signing, credential management, and API versioning. Since Hunyuan3D via Tencent is the primary 3D provider, having an official SDK rather than hand-rolling auth is a significant advantage.

**Go is fast enough.** The pipeline is 95% I/O bound (waiting on HTTP responses). The one CPU-bound operation — mesh post-processing — can use meshoptimizer via cgo or call Blender as a subprocess. The performance difference between Go and Rust for this workload is negligible.

**Why not Rust:** Rust would produce an equally correct binary, but the development speed tradeoff isn't justified. Rust's strengths (zero-cost abstractions, borrow checker, fearless concurrency) don't provide meaningful benefits for a tool that's mostly glue code between HTTP APIs. The borrow checker would slow development without catching bugs that matter in this domain. Compile times would be 10-20x slower than Go for a project this size.

**Why not TypeScript:** The shared-stack argument (originating project is SvelteKit/TS) sounds appealing but doesn't hold up. huragok is a standalone tool, not a library imported by the portfolio project. TypeScript's advantages (gltf-transform integration, SDK coverage) are offset by Node.js startup latency, distribution complexity (npm + runtime dependency), and the general friction of shipping CLI tools via npm.

### Technology choices

**Language:** Go 1.22+

**Module path:** `github.com/jorgoose/huragok`

**CLI framework: cobra + viper**

cobra handles subcommands, flags, help text, and shell completions. viper handles configuration file loading with support for TOML, environment variables, and flag binding. They're designed to work together and are battle-tested in hundreds of major CLI tools.

**Interactive terminal UI: charmbracelet ecosystem (huh + lipgloss)**

The checkpoint prompts (accept/edit/regenerate/skip) need to look good and feel responsive. [huh](https://github.com/charmbracelet/huh) is a Go forms library built on bubbletea, purpose-built for exactly this kind of interactive prompt. [lipgloss](https://github.com/charmbracelet/lipgloss) handles styled terminal output (the checkpoint boxes, status messages, progress indicators). This is the same ecosystem used by the GitHub CLI.

**Why huh over simpler prompt libraries (survey, promptui):** huh provides accessible, styled, keyboard-driven forms with built-in validation. The checkpoint UI is one of the most user-facing parts of the tool — it needs to feel polished, not like a bare readline prompt. huh also integrates cleanly with lipgloss for consistent styling across the entire CLI.

**Configuration: TOML (via viper)**

Human-readable, supports comments, clean nested table syntax. viper handles TOML parsing natively. The config hierarchy (global → project → CLI flags) maps directly to viper's layered config model — viper was designed for exactly this pattern.

**Post-processing: meshoptimizer (via cgo) + qmuntal/gltf**

[qmuntal/gltf](https://github.com/qmuntal/gltf) is a well-maintained Go library for reading and writing glTF/GLB files. It handles parsing, modification, and serialization of the binary format.

For mesh simplification (the most computationally intensive post-processing operation), we use [meshoptimizer](https://github.com/zeux/meshoptimizer) via cgo bindings. meshoptimizer is the industry standard for mesh optimization — it's what gltf-transform uses internally (via WASM), but calling it natively via cgo gives us full native performance with no WASM overhead.

For users who can't or don't want to use cgo (cross-compilation constraints, minimal toolchain), Blender headless is available as a fallback for decimation. Basic operations (texture resizing, format conversion, metadata cleanup) are handled in pure Go and don't require cgo.

**Why not gltf-transform:** gltf-transform is a JavaScript library. Using it from Go would mean either shelling out to a Node.js subprocess (adding a runtime dependency, defeating the purpose of Go) or reimplementing its logic. meshoptimizer + qmuntal/gltf gives us equivalent functionality natively.

**Texture processing: Go standard library (image package)**

Go's built-in `image` package handles PNG/JPEG decoding, resizing, and re-encoding. For the operations we need (resize textures to target resolution, basic format conversion), this is sufficient without external dependencies. The `golang.org/x/image/draw` package provides higher-quality resampling (Lanczos, CatmullRom) when needed.

**Review UI: SvelteKit (embedded via go:embed)**

The review UI is a separate SvelteKit web application that gets pre-built into static HTML/JS/CSS and embedded into the Go binary using Go's `//go:embed` directive. When the user runs `huragok review`, Go serves the embedded static files via `net/http`. No Node.js runtime needed at execution time — the SvelteKit build happens during CI/release, not on the user's machine.

**Why SvelteKit for the UI even though the CLI is Go:** The review UI is a web frontend — it needs a 3D model viewer (three.js), image galleries, and interactive dashboards. These are inherently browser-based. SvelteKit is the best tool for building this kind of lightweight, fast-loading web UI. The fact that it's a different language than the CLI doesn't matter because the boundary is clean: the Go binary serves static files, the browser renders them.

**Provider SDKs:**

| Provider | SDK | Notes |
|---|---|---|
| OpenAI (prompt, image) | `github.com/sashabaranov/go-openai` | Well-maintained community SDK, widely used |
| Anthropic (prompt) | `github.com/anthropics/anthropic-sdk-go` | Official Go SDK |
| Tencent/Hunyuan3D (3D) | `github.com/tencentcloud/tencentcloud-sdk-go` | Official SDK, handles auth signing |
| Meshy (3D) | Direct HTTP via `net/http` | REST API, no official Go SDK |
| Stability AI (image) | Direct HTTP via `net/http` | REST API, straightforward |

For providers without official Go SDKs, direct HTTP is idiomatic Go. The `net/http` standard library is excellent, and wrapping a REST API in a Go client is minimal code.

**Build & distribution: goreleaser**

[goreleaser](https://goreleaser.com/) automates cross-compilation and release publishing. A single `goreleaser release` produces binaries for Linux/macOS/Windows on amd64 and arm64, creates GitHub releases with checksums, and optionally publishes to Homebrew and Scoop. This runs in GitHub Actions on tag push.

### Project structure

```
huragok/
├── cmd/
│   └── huragok/
│       └── main.go                 # Entry point — initializes cobra root command
│
├── internal/                       # Private application code (enforced by Go)
│   ├── cli/                        # Cobra command definitions
│   │   ├── root.go                 # Root command, global flags, viper setup
│   │   ├── create.go               # `huragok create` command
│   │   ├── resume.go               # `huragok resume` command
│   │   ├── runs.go                 # `huragok runs` (list, inspect, clean, costs)
│   │   ├── review.go               # `huragok review` — serves embedded UI
│   │   ├── config.go               # `huragok config` (show, init, set)
│   │   └── providers.go            # `huragok providers` — list available providers
│   │
│   ├── pipeline/                   # Pipeline orchestration
│   │   ├── pipeline.go             # Stage sequencing, resume logic, cost enforcement
│   │   ├── checkpoint.go           # Interactive checkpoint UI (huh forms)
│   │   └── types.go                # PipelineMode, StageResult, PipelineOptions
│   │
│   ├── stage/                      # Stage implementations
│   │   ├── stage.go                # Stage interface definition
│   │   ├── prompt/
│   │   │   ├── stage.go            # Prompt refinement logic
│   │   │   └── templates.go        # Prompt templates (image-optimized, 3D-optimized)
│   │   ├── image/
│   │   │   └── stage.go            # Image generation logic
│   │   ├── model3d/
│   │   │   └── stage.go            # 3D model generation logic
│   │   └── postprocess/
│   │       ├── stage.go            # Post-processing orchestration
│   │       ├── simplify.go         # Mesh decimation (meshoptimizer via cgo)
│   │       ├── texture.go          # Texture resizing and format conversion
│   │       └── cleanup.go          # Mesh cleanup (degenerate tris, normals, scale)
│   │
│   ├── provider/                   # Provider adapters
│   │   ├── provider.go             # Provider interfaces (PromptProvider, ImageProvider, etc.)
│   │   ├── registry.go             # Provider lookup by name
│   │   ├── openai/
│   │   │   ├── prompt.go           # OpenAI as prompt refinement provider
│   │   │   └── image.go            # OpenAI as image generation provider
│   │   ├── anthropic/
│   │   │   └── prompt.go           # Anthropic as prompt refinement provider
│   │   ├── hunyuan/
│   │   │   └── model3d.go          # Hunyuan3D via Tencent Cloud API
│   │   └── meshy/
│   │       └── model3d.go          # Meshy as 3D generation provider
│   │
│   ├── config/                     # Configuration
│   │   ├── config.go               # Load, merge, validate config hierarchy
│   │   ├── defaults.go             # Built-in default values
│   │   └── types.go                # Config structs with mapstructure tags
│   │
│   ├── run/                        # Run management
│   │   ├── manager.go              # Create, list, inspect, clean runs
│   │   ├── persistence.go          # Read/write run artifacts and meta.json
│   │   └── types.go                # RunMeta, RunStage, CostRecord
│   │
│   └── review/                     # Review UI server
│       ├── server.go               # HTTP server, serves embedded UI + run data API
│       └── embed.go                # go:embed directive for static UI assets
│
├── ui/                             # SvelteKit review app (source, not Go code)
│   ├── src/
│   │   ├── routes/                 # Pages: run list, run detail, model viewer
│   │   └── lib/
│   │       └── components/         # three.js viewer, image gallery, cost chart
│   ├── svelte.config.js
│   ├── package.json
│   └── vite.config.ts
│
├── go.mod
├── go.sum
├── .goreleaser.yaml                # Cross-compilation and release config
├── Makefile                        # Build, test, lint, embed-ui targets
├── PLAN.md
└── README.md
```

**Why `internal/` instead of `pkg/`:** huragok is an application, not a library. Nobody should import its internals. Go's `internal/` directory enforces this at the compiler level — code under `internal/` cannot be imported by external modules. This is a Go convention that prevents accidental coupling.

**Why the UI is a subdirectory, not a separate repo:** The review UI reads from `.huragok/runs/` and needs to understand the run data format (meta.json schema, artifact naming conventions). Keeping it in the same repo means the UI and CLI always agree on the data contract. The UI is built separately (npm) but embedded into the Go binary at compile time.

### Key interfaces

Go interfaces are satisfied implicitly — a type implements an interface by having the right methods, without declaring that it does. This makes adding new providers trivial: write a struct with the right methods, register it, done.

```go
// provider/provider.go

// PromptProvider refines a raw text prompt into an optimized generation prompt.
type PromptProvider interface {
    Name() string
    Refine(ctx context.Context, input string, pctx PromptContext) (string, error)
    EstimateCost() float64
}

// ImageProvider generates concept images from a text prompt.
type ImageProvider interface {
    Name() string
    Generate(ctx context.Context, prompt string, opts ImageOptions) ([]GeneratedImage, error)
    EstimateCost(opts ImageOptions) float64
}

// Model3DProvider generates 3D meshes from text or images.
type Model3DProvider interface {
    Name() string
    FromText(ctx context.Context, prompt string, opts ModelOptions) (*RawMesh, error)
    FromImage(ctx context.Context, images []string, opts ModelOptions) (*RawMesh, error)
    EstimateCost(opts ModelOptions) float64
    Capabilities() ProviderCapabilities
}

type ProviderCapabilities struct {
    TextTo3D  bool
    ImageTo3D bool
    MultiView bool
    MaxImages int
}
```

```go
// stage/stage.go

// Stage represents one step of the pipeline.
// Each stage knows how to execute, persist its output, and restore from a previous run.
type Stage interface {
    Name() string
    Execute(ctx context.Context, input StageInput, cfg *config.Config, run *run.Context) (*StageOutput, error)
    Persist(output *StageOutput, runDir string) error
    Restore(runDir string) (*StageOutput, error)
}
```

```go
// pipeline/types.go

type Options struct {
    Mode             PipelineMode   // full | direct
    Auto             bool           // skip interactive checkpoints
    StartFrom        string         // stage name, for resume
    ProviderOverride string         // override 3D provider for this run
    MaxCost          float64        // abort if cost would exceed this
    Variants         int            // generate N variants at 3D stage
    OutputPath       string         // copy final .glb here
    JSONOutput       bool           // print structured JSON to stdout
}
```

**Why `context.Context` on every provider method:** All provider calls are HTTP requests that can take seconds to minutes. Context enables cancellation (user presses Ctrl+C), timeouts (provider is unresponsive), and deadline propagation. This is idiomatic Go for any I/O-bound operation and gives us clean shutdown behavior for free.

**Why `error` returns instead of exceptions:** Go doesn't have exceptions. Errors are explicit return values. This forces every call site to handle the error case, which is exactly what we want — API calls fail, networks time out, providers return garbage. Explicit error handling means we never silently swallow a failure. Errors are wrapped with context (`fmt.Errorf("hunyuan3d: poll failed: %w", err)`) so the user sees a clear chain of what went wrong.

### Data flow

```
┌──────────────────────────────────────────────────────────────────────┐
│                            Pipeline                                   │
│                                                                       │
│  ~/.huragok/config.toml ─┐                                            │
│  .huragok/config.toml ───┤──▶ viper merges ──▶ config.Config          │
│  CLI flags ──────────────┘                                            │
│                                                                       │
│  ┌─────────────────────────────────────────────────────────────┐      │
│  │  Stage 1: Prompt                                             │      │
│  │  input:      string (raw user prompt)                        │      │
│  │  provider:   PromptProvider (resolved from config)           │      │
│  │  output:     string (refined prompt)                         │      │
│  │  persists:   prompt_input.txt, prompt_refined.txt            │      │
│  │  checkpoint: accept / edit / regenerate / skip               │      │
│  └──────────────────────────┬──────────────────────────────────┘      │
│                              ▼                                        │
│  ┌─────────────────────────────────────────────────────────────┐      │
│  │  Stage 2: Image  (skipped in direct mode)                    │      │
│  │  input:      string (refined prompt)                         │      │
│  │  provider:   ImageProvider (resolved from config)            │      │
│  │  output:     []GeneratedImage (file paths on disk)           │      │
│  │  persists:   images/*.png                                    │      │
│  │  checkpoint: accept / regenerate / manual upload / swap      │      │
│  └──────────────────────────┬──────────────────────────────────┘      │
│                              ▼                                        │
│  ┌─────────────────────────────────────────────────────────────┐      │
│  │  Stage 3: Model3D                                            │      │
│  │  input:      string OR []GeneratedImage (depends on mode)    │      │
│  │  provider:   Model3DProvider (resolved from config)          │      │
│  │  output:     *RawMesh (file path + face/vertex counts)       │      │
│  │  persists:   model_raw.glb                                   │      │
│  │  checkpoint: accept / regenerate / variants / back / swap    │      │
│  └──────────────────────────┬──────────────────────────────────┘      │
│                              ▼                                        │
│  ┌─────────────────────────────────────────────────────────────┐      │
│  │  Stage 4: PostProcess                                        │      │
│  │  input:      *RawMesh (file path)                            │      │
│  │  tools:      meshoptimizer (cgo) + qmuntal/gltf              │      │
│  │  output:     *ProcessedMesh (file path + stats)              │      │
│  │  persists:   model_final.glb                                 │      │
│  │  checkpoint: accept / redo with different settings            │      │
│  └──────────────────────────┬──────────────────────────────────┘      │
│                              ▼                                        │
│  Copy model_final.glb → --output path (if specified)                  │
│  Write meta.json with full run record                                 │
│  Print JSON to stdout (if --json), all logs to stderr                 │
└──────────────────────────────────────────────────────────────────────┘
```

### Concurrency model

Go's goroutines and channels map cleanly to the concurrency patterns huragok needs:

**API polling (Hunyuan3D):** The Tencent API is asynchronous — submit a job, receive a task ID, poll for completion. A goroutine polls with exponential backoff while the main goroutine shows a progress spinner. The poll goroutine sends status updates via a channel; the main goroutine renders them. Context cancellation (Ctrl+C) propagates to the poll goroutine cleanly.

**Variant generation:** When the user requests N variants (`--variants 3`), each variant is generated in its own goroutine via `errgroup`. All variants run concurrently against the provider API (subject to rate limits). Results are collected and presented for comparison.

**Graceful shutdown:** All long-running operations accept `context.Context`. When the user presses Ctrl+C, the context is cancelled, in-flight HTTP requests are aborted, partial artifacts are cleaned up, and the run is marked as `cancelled` in meta.json. This is built into Go's context system — no custom signal handling needed.

### Cost tracking architecture

Cost tracking is woven into the provider and pipeline layers, not bolted on as an afterthought.

```
Provider.EstimateCost()     Called BEFORE each API call
        │                   Returns estimated USD cost for the operation
        ▼
Pipeline.costAccumulator    Running total for the current run
        │                   Checked against --max-cost before each stage
        ▼
RunMeta.Costs               Written to meta.json after each stage completes
        │                   Itemized by stage and provider
        ▼
`huragok runs costs`        Reads meta.json across runs for summary reporting
```

**Why pre-call estimation matters:** In `--auto` mode, the tool must decide whether to proceed *before* making an API call. If the accumulated cost plus the estimated next-call cost exceeds `--max-cost`, the pipeline aborts with a non-zero exit code. This prevents runaway spending in batch scripts and agent workflows.

Cost estimates are necessarily approximate — providers don't always publish exact pricing, and some charge based on output complexity. But an approximate ceiling is far better than no ceiling.

### Cross-platform considerations

huragok is developed on Windows but must work on Linux and macOS. Go handles most cross-platform concerns automatically (file I/O, networking, path handling), but a few areas need explicit attention:

**Opening files in default viewer:** The checkpoint system opens generated images and models in the user's default application. This is `start` on Windows, `open` on macOS, and `xdg-open` on Linux. A build-tagged helper (`open_windows.go`, `open_darwin.go`, `open_linux.go`) handles this.

**Home directory and config paths:** `~/.huragok/config.toml` resolves differently on each platform. Go's `os.UserHomeDir()` handles this. On Windows, this is `%USERPROFILE%`, not `%HOME%`.

**cgo and meshoptimizer:** cgo requires a C compiler. On Linux/macOS this is typically available (gcc/clang). On Windows, MinGW or MSVC is needed. For users without a C toolchain, the Makefile provides a `CGO_ENABLED=0` build target that disables meshoptimizer and falls back to Blender for decimation. goreleaser builds both variants.

---

## Implementation phases

### Phase 1: Foundation

**Goal:** A working CLI skeleton that can create, track, and manage runs — but doesn't call any APIs yet.

**What gets built:**
- `cmd/huragok/main.go` entry point
- Cobra command tree (create, resume, runs, config, providers)
- Viper config system (load TOML, merge global + project + flags, validate)
- Run manager (create run directory, write meta.json, list/inspect/clean runs)
- Pipeline orchestrator (stage sequencing, checkpoint pause/resume logic)
- Stage interface with stub implementations that print what they would do
- Makefile with build, test, lint targets

**Why this is first:** Everything else depends on the pipeline orchestration and run management. Building this first means we can test the flow end-to-end with mock stages before integrating real API calls. It also forces us to get the data model right early — changing the run directory structure or config schema later is painful. Go's fast compile times mean the feedback loop here is tight.

**Definition of done:** `huragok create "test"` creates a run directory with meta.json, walks through the stage stubs (printing "would call prompt refinement here"), and writes a complete run record. `huragok runs` lists it. `huragok config init` creates a `.huragok/config.toml`. The binary starts in <10ms.

### Phase 2: Core stages

**Goal:** The pipeline makes real API calls and produces real artifacts.

**What gets built:**
- Prompt refinement stage + OpenAI provider adapter (`go-openai` SDK)
- Image generation stage + OpenAI provider adapter
- 3D generation stage + Hunyuan3D provider adapter (`tencentcloud-sdk-go`)
- Post-processing stage (meshoptimizer via cgo for decimation, `qmuntal/gltf` for GLB manipulation, Go `image` package for texture resizing)

**Why this order within the phase:** Prompt → Image → 3D follows the pipeline order. Each stage can be tested independently as it's built (prompt refinement doesn't need 3D generation to work). Post-processing comes last because it needs a real mesh to operate on.

**Provider-specific notes:**

*Hunyuan3D (Tencent API):* This is the most complex provider integration. The Tencent API is asynchronous — you submit a generation job, receive a task ID, and poll for completion. The adapter needs to handle: job submission via the official Go SDK, polling with exponential backoff (goroutine + channel), timeout detection via context deadline, error classification (transient vs. permanent), and downloading the result mesh. It also needs to handle both text-to-3D and image-to-3D modes since Hunyuan supports both.

*OpenAI (image generation):* Relatively straightforward via `go-openai`. Submit prompt, receive image URL or base64, download and save. The main complexity is handling the different image modes (single vs. sheet). Multi-angle mode is deferred to a later phase since it requires multi-view synthesis.

*Post-processing (meshoptimizer + qmuntal/gltf):* This runs locally, no API calls. The primary operations are mesh simplification via meshoptimizer's `simplify` function (target face count, error threshold), texture resizing via Go's image package (decode, resize with Lanczos, re-encode), and GLB cleanup via qmuntal/gltf (remove degenerate triangles, fix normals, normalize scale). Need to handle edge cases like meshes with no UVs, meshes with multiple materials, and extremely high poly counts that exhaust memory.

**Definition of done:** `huragok create "sci-fi cargo crate" --auto --pipeline full` generates a refined prompt, produces a concept image, sends it to Hunyuan3D, decimates the result, and writes a game-ready .glb to the run directory. All artifacts are persisted. Cost is tracked in meta.json.

### Phase 3: Interactive mode

**Goal:** The checkpoint system works — the user can review, approve, edit, regenerate, or skip at each stage.

**What gets built:**
- Checkpoint UI using charmbracelet/huh (styled, keyboard-driven selection prompts)
- Progress display using charmbracelet/lipgloss (styled status messages, the checkpoint boxes from the README)
- Image preview (opens in default system viewer via platform-specific `open` command)
- Model preview (opens .glb in default viewer, or prints path for manual inspection)
- Edit flow (user modifies the refined prompt inline before advancing)
- Regenerate flow (re-runs the current stage with the same inputs)
- Skip flow (advances to next stage without running current one)
- Back flow (returns to a previous stage, using persisted artifacts from earlier stages)

**Why this is its own phase:** Interactive mode is a UX layer on top of the pipeline, not a core pipeline feature. The pipeline needs to work in both interactive and headless mode. Building headless first (Phase 2) and layering interactivity on top ensures we don't accidentally couple the two. The `--auto` flag simply skips the checkpoint calls — the pipeline logic is identical.

**Definition of done:** Running `huragok create "test"` without `--auto` pauses at each stage with a styled huh prompt. The user can accept, edit, regenerate, skip, and go back. The experience feels responsive and clear. lipgloss styling is consistent throughout.

### Phase 4: Agent integration

**Goal:** Claude Code (and other agents/scripts) can invoke huragok and parse structured output.

**What gets built:**
- `--auto` flag (suppress all interactive checkpoints — already partially built in Phase 2)
- `--json` flag (structured JSON to stdout via `encoding/json`, all human-readable logs to stderr)
- Exit code system (0 success, 1 stage failure, 2 config error, 3 network error, 4 user cancelled)
- `--max-cost` enforcement (pipeline checks accumulated + estimated cost before each stage)
- Claude Code skill file (`.claude/skills/huragok.md`) with usage instructions and examples

**Why this matters:** This is the original motivation for the project. Without a clean non-interactive mode with structured output, the tool is just a fancy wrapper around API calls. With it, an AI agent can generate 3D assets as part of a larger workflow — e.g., Claude Code creates a new enemy type by generating the model, importing it into the game engine, and wiring up the code, all in one conversation.

**Definition of done:** `huragok create "cargo crate" --auto --output static/crate.glb --json` runs end-to-end with no user interaction, writes the GLB to the specified path, and prints valid JSON to stdout with run ID, stage statuses, face counts, file sizes, and cost. Exit code is 0 on success, non-zero on failure. Claude Code can invoke it via the skill file, parse the output, and use the result.

### Phase 5: Review UI

**Goal:** A local web dashboard for visually inspecting generated assets.

**What gets built:**
- SvelteKit app in `ui/` directory (run list, run detail, image gallery, three.js model viewer, cost dashboard)
- Go HTTP server in `internal/review/` that serves the embedded UI and exposes a JSON API for run data
- `//go:embed` integration — the built UI is embedded in the binary at compile time
- Makefile target (`make embed-ui`) that builds the SvelteKit app and copies output to the embed directory
- goreleaser config updated to run `make embed-ui` before building release binaries

**Why a web UI instead of a native app or TUI:**
- three.js is the most mature, portable 3D viewer available — and it runs in a browser
- No install burden for the user (no Electron, no native dependencies)
- A local server reading from `.huragok/runs/` means zero state management — the filesystem is the database
- The UI is embedded in the binary, so it still feels like a single tool — `huragok review` just works

**Review UI serves data, not just static files:** The Go server doesn't just serve the SvelteKit bundle. It also exposes a small JSON API (`/api/runs`, `/api/runs/:id`, `/api/runs/:id/artifacts/:name`) that the frontend calls to list runs, read meta.json, and serve artifact files (images, GLBs). This keeps the frontend simple — it doesn't need to understand file system paths.

**Why this phase is later:** The review UI is valuable but not load-bearing. The CLI works without it. Phases 1–4 deliver a complete, functional tool. The review UI is the quality-of-life layer that makes it pleasant to use for extended sessions.

**Definition of done:** `huragok review` opens a browser tab showing all runs. Clicking a run shows its artifacts. The three.js viewer lets you rotate and inspect models with wireframe toggle. The image gallery shows concept art at full resolution. The cost dashboard shows spend over time.

### Phase 6: Extended features

**Goal:** Quality-of-life improvements and ecosystem expansion.

**What gets built (roughly prioritized):**
- Additional 3D providers (Meshy, Tripo) — implement `Model3DProvider` interface, register in provider registry
- Additional image providers (Stability AI)
- Anthropic as a prompt refinement provider (official Go SDK)
- Multi-view synthesis support (generate consistent multi-angle views from a single concept image)
- Variant generation at the 3D stage (errgroup for concurrent generation, comparison UI)
- Batch mode (`huragok batch assets.json` — run multiple create commands from a manifest)
- Recipe/preset system (predefined configs for common asset types: weapon, vehicle, prop, character)
- `huragok diff` — compare two runs visually (useful for A/B testing providers)
- Shell completions (cobra generates bash/zsh/fish completions automatically)
- Homebrew and Scoop distribution (goreleaser publishes to package managers)

**Why these are deferred:** Each of these is independently valuable but none are required for the core workflow to function. They expand what's possible without changing the fundamental architecture.

---

## Key design decisions

### Why checkpoint review instead of automatic quality assessment

It's tempting to add an automated quality gate — run the generated mesh through a quality scorer and auto-reject bad results. We intentionally don't do this in v1.

**Reason:** "Quality" for a 3D game asset is deeply context-dependent. A low-poly stylized crate might look "bad" to an automated scorer but be exactly what the user wants. A photorealistic mesh might score high but be completely wrong for the project's art style. Human review (or informed agent review) is the only reliable quality signal right now. We may add optional automated scoring later, but it should never be the gatekeeper.

### Why TOML over JSON/YAML for configuration

JSON doesn't support comments. For a config file that users will read and edit, comments are essential — you need to explain what each option does and what the valid values are. YAML supports comments but has well-documented footprint problems (implicit type coercion, significant whitespace). TOML is explicit, supports comments, and has clear table syntax for nested configuration. Viper supports TOML natively, so there's no extra dependency.

### Why meshoptimizer via cgo instead of pure Go mesh processing

Mesh simplification is a solved problem, and meshoptimizer is the industry-standard solution used by glTF toolchains, game engines, and asset pipelines worldwide. Writing our own simplification algorithm in pure Go would be slower, buggier, and produce worse results. The cgo cost (C compiler required for building from source) is mitigated by distributing pre-built binaries via goreleaser and offering a `CGO_ENABLED=0` build that falls back to Blender.

### Why runs are directories, not a database

A run is a directory of files: images, meshes, text files, and one meta.json. There's no SQLite database, no append-only log, no binary format.

**Reason:** Files are inspectable. You can `ls` a run directory and see exactly what's in it. You can copy a run to another machine. You can delete it with `rm -rf`. You can version-control it if you want. You can point any tool at the files — open the GLB in Blender, open the images in Photoshop. A database would add a dependency, make debugging harder, and require migration logic as the schema evolves. The filesystem is the simplest persistence layer that meets our needs.

### Why the review UI is read-only

The review UI can view runs and artifacts but cannot modify them, trigger regeneration, or change configuration. All mutations go through the CLI.

**Reason:** One source of truth. If the UI and CLI can both modify state, you get synchronization bugs, race conditions, and two codepaths to maintain for every operation. The CLI is the single writer; the UI is a viewer. This is a constraint that simplifies everything. If we find that the UI needs write capabilities later, we add them carefully as CLI invocations triggered by the UI (the UI calls the CLI, not the other way around).

### Why `internal/` over `pkg/`

huragok is a CLI application, not a library. Its internals (pipeline orchestration, provider adapters, run management) are implementation details, not a public API. Go's `internal/` directory enforces this at the compiler level — no external module can import anything under `internal/`. Using `pkg/` would imply that the code is designed for external consumption, which it is not. This prevents accidental coupling and signals intent clearly.

---

## Open questions

These are decisions that don't need to be made yet but will need to be resolved during implementation.

1. **meshoptimizer Go bindings:** Need to evaluate existing cgo wrappers for meshoptimizer. If none are mature enough, we may need to write thin bindings ourselves. The alternative is calling meshoptimizer as a subprocess (the library ships a CLI tool) or using Blender as the decimation backend from the start.

2. **Tencent Hunyuan3D API specifics:** The exact API endpoints, request/response formats, and polling behavior for Hunyuan3D need to be mapped out from the Tencent Cloud documentation. The official Go SDK handles auth, but the 3D generation API may have specific quirks (job TTL, max mesh size, rate limits) that affect the adapter design.

3. **Multi-view synthesis provider:** For the multi-angle image mode, we need a model that generates consistent views from a single image. Zero123++, SV3D, and Hunyuan3D's own multi-view mode are candidates. Need to evaluate quality, cost, and API availability.

4. **Review UI build pipeline:** The SvelteKit app needs to be built and its output embedded in the Go binary. This means the CI pipeline needs Node.js for the UI build and Go for the binary build. goreleaser hooks (`before` section) can handle this, but the Dockerfile and GitHub Actions workflow need to support both runtimes.

5. **cgo cross-compilation:** goreleaser builds binaries for 6 targets (linux/darwin/windows x amd64/arm64). With cgo enabled, each target needs a C cross-compiler. This is solvable (zig cc, xgo, or Docker-based cross-compilation) but adds CI complexity. The fallback is to build CGO_ENABLED=0 binaries that lack native mesh simplification and rely on Blender.

6. **Homebrew tap naming:** For `brew install huragok`, we need a Homebrew tap (`jorgoose/tap`) and a formula. goreleaser can generate this automatically, but the tap repo needs to be created on GitHub.
