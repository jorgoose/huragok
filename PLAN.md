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

### Technology choices

**Language: TypeScript (Node.js)**

- The originating project (halo-portfolio) is SvelteKit/TypeScript — shared language reduces friction
- gltf-transform (the primary post-processing library) is a JavaScript/TypeScript library — native integration, no subprocess overhead
- OpenAI, Anthropic, and most AI provider SDKs have first-class TypeScript support
- The review UI will be built with SvelteKit — sharing a language between CLI and UI means shared types, shared utilities, and one `node_modules`
- npm distribution makes installation trivial (`npm install -g huragok` or `npx huragok`)

**CLI framework: commander.js**

Widely used, stable, good TypeScript support. Handles subcommands (`huragok create`, `huragok runs`, `huragok review`) cleanly. No magic, easy to understand.

**Configuration: TOML**

- Human-readable and human-editable (unlike JSON)
- Supports nested tables cleanly (unlike INI/dotenv)
- Well-suited for the config hierarchy (global → project → CLI overrides)
- Standard library support via `@iarna/toml` or `smol-toml`

**Post-processing: gltf-transform**

- Industry-standard GLB/glTF manipulation library
- Handles decimation, texture resizing, mesh optimization, format conversion
- Runs in-process (no Blender dependency for common operations)
- Blender headless is available as an optional backend for advanced operations (complex UV unwrapping, non-glTF format export), but is not required

**Review UI: SvelteKit**

- Lightweight, fast, server-side rendering
- three.js integration for 3D model viewing is well-documented
- Shares TypeScript types with the CLI
- Launched as a local dev server — no build step needed for the user

### Project structure

```
huragok/
├── src/
│   ├── cli/                    # CLI entry point and command definitions
│   │   ├── index.ts            # Main entry, commander setup
│   │   ├── create.ts           # `huragok create` command
│   │   ├── resume.ts           # `huragok resume` command
│   │   ├── runs.ts             # `huragok runs` command
│   │   ├── review.ts           # `huragok review` command (launches UI server)
│   │   ├── config.ts           # `huragok config` command
│   │   └── providers.ts        # `huragok providers` command
│   │
│   ├── pipeline/               # Pipeline orchestration
│   │   ├── pipeline.ts         # Sequencing, checkpoint logic, resume logic
│   │   ├── checkpoint.ts       # Interactive review (accept/edit/regenerate/skip)
│   │   └── types.ts            # Pipeline-level types (PipelineMode, StageResult)
│   │
│   ├── stages/                 # Stage implementations
│   │   ├── prompt/
│   │   │   ├── stage.ts        # Prompt refinement stage logic
│   │   │   └── templates.ts    # Prompt templates (image-optimized, 3D-optimized)
│   │   ├── image/
│   │   │   └── stage.ts        # Image generation stage logic
│   │   ├── model3d/
│   │   │   └── stage.ts        # 3D generation stage logic
│   │   └── postprocess/
│   │       ├── stage.ts        # Post-processing stage logic
│   │       └── operations.ts   # Individual operations (decimate, bake, cleanup)
│   │
│   ├── providers/              # Provider adapters (one dir per provider)
│   │   ├── types.ts            # Provider interfaces (PromptProvider, ImageProvider, etc.)
│   │   ├── openai/
│   │   │   ├── prompt.ts       # OpenAI as a prompt refinement provider
│   │   │   └── image.ts        # OpenAI as an image generation provider
│   │   ├── anthropic/
│   │   │   └── prompt.ts       # Anthropic as a prompt refinement provider
│   │   ├── hunyuan/
│   │   │   └── model3d.ts      # Hunyuan3D as a 3D generation provider
│   │   ├── meshy/
│   │   │   └── model3d.ts      # Meshy as a 3D generation provider
│   │   └── registry.ts         # Provider registry (lookup by name)
│   │
│   ├── config/                 # Configuration loading and resolution
│   │   ├── config.ts           # Load, merge, resolve config hierarchy
│   │   ├── defaults.ts         # Built-in default values
│   │   └── schema.ts           # Config validation / types
│   │
│   ├── runs/                   # Run management
│   │   ├── manager.ts          # Create, list, inspect, clean runs
│   │   ├── persistence.ts      # Read/write run artifacts to disk
│   │   └── types.ts            # RunMeta, RunStatus, etc.
│   │
│   └── review/                 # Review UI (SvelteKit app)
│       ├── src/
│       │   ├── routes/         # Pages: run list, run detail, model viewer
│       │   └── lib/
│       │       └── components/ # Three.js viewer, image gallery, cost chart
│       ├── svelte.config.js
│       └── package.json
│
├── bin/
│   └── huragok.js              # Shebang entry point (#!/usr/bin/env node)
│
├── package.json
├── tsconfig.json
├── PLAN.md
└── README.md
```

### Key interfaces

These interfaces define the contracts between components. Providers implement them; stages consume them.

```typescript
// --- Provider interfaces ---

interface PromptProvider {
  name: string;
  refine(input: string, context: PromptContext): Promise<string>;
  estimateCost(): number;
}

interface ImageProvider {
  name: string;
  generate(prompt: string, options: ImageOptions): Promise<GeneratedImage[]>;
  estimateCost(options: ImageOptions): number;
}

interface Model3DProvider {
  name: string;
  fromText(prompt: string, options: ModelOptions): Promise<RawMesh>;
  fromImage(images: string[], options: ModelOptions): Promise<RawMesh>;
  estimateCost(options: ModelOptions): number;
  capabilities(): {
    textTo3D: boolean;
    imageTo3D: boolean;
    multiView: boolean;
    maxImages: number;
  };
}

// --- Stage interface ---

interface Stage<TInput, TOutput> {
  name: StageName;
  execute(input: TInput, config: ResolvedConfig, run: RunContext): Promise<TOutput>;
  // Serialize output to the run directory for persistence/resume
  persist(output: TOutput, runDir: string): Promise<void>;
  // Restore output from a previous run (for resume)
  restore(runDir: string): Promise<TOutput>;
}

// --- Pipeline orchestration ---

interface PipelineOptions {
  mode: 'full' | 'direct';
  auto: boolean;              // skip interactive checkpoints
  startFrom?: StageName;      // for resume
  providerOverrides?: Partial<ProviderConfig>;
  maxCost?: number;
  variants?: number;
  outputPath?: string;
}
```

**Why these interfaces matter:** They enforce clean boundaries. A new provider (e.g., Meshy) only needs to implement `Model3DProvider` — it doesn't need to know about the pipeline, the config system, or the run manager. A new stage only needs to implement `Stage<TInput, TOutput>` — it doesn't care which provider is behind it. This separation is what makes the system extensible without cascading changes.

### Data flow

```
┌─────────────────────────────────────────────────────────────────┐
│                         Pipeline                                 │
│                                                                  │
│  config.toml ──▶ ResolvedConfig                                  │
│                       │                                          │
│  CLI args ────────────┘                                          │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  Stage 1: Prompt                                          │    │
│  │  input: string (raw prompt)                               │    │
│  │  provider: PromptProvider (from config)                    │    │
│  │  output: string (refined prompt)                          │    │
│  │  persists: prompt_input.txt, prompt_refined.txt           │    │
│  │  checkpoint: accept / edit / regenerate / skip            │    │
│  └─────────────────────────┬────────────────────────────────┘    │
│                             ▼                                    │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  Stage 2: Image  (skipped in direct mode)                 │    │
│  │  input: string (refined prompt)                           │    │
│  │  provider: ImageProvider (from config)                     │    │
│  │  output: GeneratedImage[] (file paths)                    │    │
│  │  persists: images/*.png                                   │    │
│  │  checkpoint: accept / regenerate / manual upload / swap   │    │
│  └─────────────────────────┬────────────────────────────────┘    │
│                             ▼                                    │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  Stage 3: Model3D                                         │    │
│  │  input: string OR GeneratedImage[] (depends on mode)      │    │
│  │  provider: Model3DProvider (from config)                   │    │
│  │  output: RawMesh (file path + metadata)                   │    │
│  │  persists: model_raw.glb                                  │    │
│  │  checkpoint: accept / regenerate / variants / back / swap │    │
│  └─────────────────────────┬────────────────────────────────┘    │
│                             ▼                                    │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  Stage 4: PostProcess                                     │    │
│  │  input: RawMesh (file path)                               │    │
│  │  provider: gltf-transform (+ optional Blender)            │    │
│  │  output: ProcessedMesh (file path + stats)                │    │
│  │  persists: model_final.glb                                │    │
│  │  checkpoint: accept / redo with different settings         │    │
│  └─────────────────────────┬────────────────────────────────┘    │
│                             ▼                                    │
│  Copy model_final.glb → --output path (if specified)             │
│  Write meta.json with full run record                            │
│  Print JSON to stdout (if --json)                                │
└──────────────────────────────────────────────────────────────────┘
```

### Cost tracking architecture

Cost tracking is woven into the provider and pipeline layers, not bolted on as an afterthought.

```
Provider.estimateCost()     Called BEFORE each API call
        │                   Returns estimated USD cost for the operation
        ▼
Pipeline.costAccumulator    Running total for the current run
        │                   Checked against --max-cost before each stage
        ▼
RunMeta.costs               Written to meta.json after each stage completes
        │                   Itemized by stage and provider
        ▼
`huragok runs costs`        Reads meta.json across runs for summary reporting
```

**Why pre-call estimation matters:** In `--auto` mode, the tool must decide whether to proceed *before* making an API call. If the accumulated cost plus the estimated next-call cost exceeds `--max-cost`, the pipeline aborts with a non-zero exit code. This prevents runaway spending in batch scripts and agent workflows.

Cost estimates are necessarily approximate — providers don't always publish exact pricing, and some charge based on output complexity. But an approximate ceiling is far better than no ceiling.

---

## Implementation phases

### Phase 1: Foundation

**Goal:** A working CLI skeleton that can create, track, and manage runs — but doesn't call any APIs yet.

**What gets built:**
- CLI entry point with commander.js (create, runs, config subcommands)
- Configuration system (load TOML, merge hierarchy, validate)
- Run manager (create run directory, write meta.json, list/inspect/clean runs)
- Pipeline orchestrator (stage sequencing, checkpoint pause/resume logic)
- Stage interface and empty stage stubs

**Why this is first:** Everything else depends on the pipeline orchestration and run management. Building this first means we can test the flow end-to-end with mock stages before integrating real API calls. It also forces us to get the data model right early — changing the run directory structure or config schema later is painful.

**Definition of done:** `huragok create "test"` creates a run directory with meta.json, walks through the stage stubs (printing "would call prompt refinement here"), and writes a complete run record. `huragok runs` lists it. `huragok config init` creates a .huragok/config.toml.

### Phase 2: Core stages

**Goal:** The pipeline makes real API calls and produces real artifacts.

**What gets built:**
- Prompt refinement stage + OpenAI provider adapter
- Image generation stage + OpenAI provider adapter
- 3D generation stage + Hunyuan3D provider adapter (Tencent API)
- Post-processing stage using gltf-transform (decimation, texture resize, mesh cleanup, scale normalization)

**Why this order within the phase:** Prompt → Image → 3D follows the pipeline order. Each stage can be tested independently as it's built (prompt refinement doesn't need 3D generation to work). Post-processing comes last because it needs a real mesh to operate on.

**Provider-specific notes:**

*Hunyuan3D (Tencent API):* This is the most complex provider integration. The Tencent API uses a polling model — you submit a job, receive a task ID, and poll for completion. The adapter needs to handle: job submission, polling with backoff, timeout detection, error handling, and downloading the result mesh. It also needs to handle both text-to-3D and image-to-3D modes, since Hunyuan supports both.

*OpenAI (image generation):* Relatively straightforward. Submit prompt, receive image URL or base64, download and save. The main complexity is handling the different image modes (single vs. sheet). Multi-angle mode is deferred to a later phase since it requires multi-view synthesis.

*gltf-transform (post-processing):* This runs locally, no API calls. The main operations are mesh simplification (weld + simplify), texture resizing (resize textures to target resolution), and format cleanup (draco compression, dedup). Need to handle edge cases like meshes with no UVs, meshes with multiple materials, and extremely high poly counts that cause memory issues during decimation.

**Definition of done:** `huragok create "sci-fi cargo crate" --pipeline full` generates a refined prompt, produces a concept image, sends it to Hunyuan3D, decimates the result, and writes a game-ready .glb to the run directory. All artifacts are persisted. Cost is tracked in meta.json.

### Phase 3: Interactive mode

**Goal:** The checkpoint system works — the user can review, approve, edit, regenerate, or skip at each stage.

**What gets built:**
- Terminal-based checkpoint UI (styled prompts with keyboard shortcuts)
- Image preview (opens in default system viewer via `open` / `xdg-open` / `start`)
- Model preview (opens .glb in default viewer, or prints path for manual inspection)
- Edit flow (user modifies the refined prompt inline before advancing)
- Regenerate flow (re-runs the current stage with the same inputs)
- Skip flow (advances to next stage without running current one)
- Back flow (returns to a previous stage)

**Why this is its own phase:** Interactive mode is a UX layer on top of the pipeline, not a core pipeline feature. The pipeline needs to work in both interactive and headless mode. Building headless first (Phase 2) and layering interactivity on top ensures we don't accidentally couple the two.

**Definition of done:** Running `huragok create "test"` without `--auto` pauses at each stage with a styled prompt. The user can accept, edit, regenerate, skip, and go back. The experience feels responsive and clear.

### Phase 4: Agent integration

**Goal:** Claude Code (and other agents/scripts) can invoke huragok and parse structured output.

**What gets built:**
- `--auto` flag (suppress all interactive checkpoints)
- `--json` flag (structured JSON output to stdout, all logs to stderr)
- Exit code system (0 success, 1 stage failure, 2 config error, 3 network error, 4 user cancelled)
- `--max-cost` enforcement (abort before API call if budget would be exceeded)
- Claude Code skill file (`.claude/skills/huragok.md`) with usage instructions and examples

**Why this matters:** This is the original motivation for the project. Without a clean non-interactive mode with structured output, the tool is just a fancy wrapper around API calls. With it, an AI agent can generate 3D assets as part of a larger workflow — e.g., Claude Code creates a new enemy type by generating the model, importing it into the game engine, and wiring up the code, all in one conversation.

**Definition of done:** `huragok create "cargo crate" --auto --output static/crate.glb --json` runs end-to-end with no user interaction, writes the GLB to the specified path, and prints valid JSON to stdout. Claude Code can invoke it via the skill file, parse the output, and use the result.

### Phase 5: Review UI

**Goal:** A local web dashboard for visually inspecting generated assets.

**What gets built:**
- SvelteKit app served by `huragok review`
- Run list page (browse all runs, filter by date/status/prompt)
- Run detail page (view all artifacts for a single run)
- Image gallery component (full-resolution concept art viewing, zoom, compare)
- Three.js model viewer component (rotate, zoom, wireframe toggle, raw vs. final comparison)
- Cost dashboard (spending over time, per-provider breakdown)
- Variant comparison view (side-by-side model viewer for variant picks)

**Why a web UI instead of a native app or TUI:**
- three.js is the most mature, portable 3D viewer available — and it runs in a browser
- No install burden for the user (no Electron, no native dependencies)
- SvelteKit shares the TypeScript stack with the CLI
- A local server reading from `.huragok/runs/` means zero state management — the filesystem is the database

**Why this phase is later:** The review UI is valuable but not load-bearing. The CLI works without it. Phases 1–4 deliver a complete, functional tool. The review UI is the quality-of-life layer that makes it pleasant to use for extended sessions.

**Definition of done:** `huragok review` opens a browser tab showing all runs. Clicking a run shows its artifacts. The 3D viewer lets you rotate and inspect models. The image gallery shows concept art at full resolution.

### Phase 6: Extended features

**Goal:** Quality-of-life improvements and ecosystem expansion.

**What gets built (roughly prioritized):**
- Additional 3D providers (Meshy, Tripo)
- Additional image providers (Stability AI)
- Anthropic as a prompt refinement provider
- Multi-view synthesis support (generate consistent multi-angle views from a single concept image)
- Variant generation at the 3D stage (generate N meshes, pick the best)
- Batch mode (`huragok batch assets.json` — run multiple create commands from a manifest)
- Recipe/preset system (predefined configs for common asset types: weapon, vehicle, prop, character)
- `huragok diff` — compare two runs visually (useful for A/B testing providers)

**Why these are deferred:** Each of these is independently valuable but none are required for the core workflow to function. They expand what's possible without changing the fundamental architecture.

---

## Key design decisions

### Why checkpoint review instead of automatic quality assessment

It's tempting to add an automated quality gate — run the generated mesh through a quality scorer and auto-reject bad results. We intentionally don't do this in v1.

**Reason:** "Quality" for a 3D game asset is deeply context-dependent. A low-poly stylized crate might look "bad" to an automated scorer but be exactly what the user wants. A photorealistic mesh might score high but be completely wrong for the project's art style. Human review (or informed agent review) is the only reliable quality signal right now. We may add optional automated scoring later, but it should never be the gatekeeper.

### Why TOML over JSON/YAML for configuration

JSON doesn't support comments. For a config file that users will read and edit, comments are essential — you need to explain what each option does and what the valid values are. YAML supports comments but has well-documented footprint problems (implicit type coercion, significant whitespace). TOML is explicit, supports comments, and has clear table syntax for nested configuration.

### Why gltf-transform over Blender for post-processing

Blender is vastly more capable but requires a separate install (~200MB+), has a complex Python API, and spawns a heavy subprocess. For the operations we need in the common case (mesh decimation, texture resizing, format conversion, mesh cleanup), gltf-transform handles them in-process with no external dependencies. Blender is available as an optional backend for edge cases (complex UV unwrapping, non-glTF export formats), but the default path should work without it.

### Why runs are directories, not a database

A run is a directory of files: images, meshes, text files, and one meta.json. There's no SQLite database, no append-only log, no binary format.

**Reason:** Files are inspectable. You can `ls` a run directory and see exactly what's in it. You can copy a run to another machine. You can delete it with `rm -rf`. You can version-control it if you want. You can point any tool at the files — open the GLB in Blender, open the images in Photoshop. A database would add a dependency, make debugging harder, and require migration logic as the schema evolves. The filesystem is the simplest persistence layer that meets our needs.

### Why the review UI is read-only

The review UI can view runs and artifacts but cannot modify them, trigger regeneration, or change configuration. All mutations go through the CLI.

**Reason:** One source of truth. If the UI and CLI can both modify state, you get synchronization bugs, race conditions, and two codepaths to maintain for every operation. The CLI is the single writer; the UI is a viewer. This is a constraint that simplifies everything. If we find that the UI needs write capabilities later, we add them carefully as CLI invocations triggered by the UI (the UI calls the CLI, not the other way around).

---

## Open questions

These are decisions that don't need to be made yet but will need to be resolved during implementation.

1. **npm package name:** Is `huragok` available on npm? If not, what's the fallback (`@huragok/cli`)?

2. **Tencent API authentication:** The Tencent Cloud API uses HMAC-SHA256 signed requests, not simple bearer tokens. Need to evaluate whether to use their official SDK or implement signing directly. The SDK may have large dependencies.

3. **Post-processing quality:** gltf-transform's mesh simplification is decent but not best-in-class. If the results are too lossy at game-ready poly counts, we may need to integrate meshoptimizer (a C++ library with WASM bindings) or fall back to Blender's decimate modifier.

4. **Multi-view synthesis provider:** For the multi-angle image mode, we need a model that generates consistent views from a single image. Zero123++, SV3D, and Hunyuan3D's own multi-view mode are candidates. Need to evaluate quality, cost, and API availability.

5. **Review UI bundling:** Should the SvelteKit review app be pre-built and bundled with the npm package, or built on-demand when the user runs `huragok review`? Pre-built is faster to start but increases package size. On-demand requires a build step but keeps the package small.

6. **Windows support:** The current project (halo-portfolio) is developed on Windows. The CLI should work cross-platform. File path handling (forward slash vs. backslash), shell commands (opening files in default viewer), and Blender binary location all need platform-aware handling.
