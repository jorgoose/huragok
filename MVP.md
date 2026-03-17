# MVP — AI Tinkerers Chicago Demo (March 17, 2026)

## Demo scenario

Run `huragok create` to generate a Halo-style pistol model on the fly, then load it into the halo-portfolio arena game as a secondary weapon. The audience watches the pipeline execute in real-time.

```bash
$ huragok create "M6D pistol, Halo CE style, matte gray" --output static/pistol.glb
```

## What the MVP does

1. Takes a text prompt from the user
2. Generates a concept image via OpenAI image API (gpt-image-1)
3. Sends that image to Hunyuan3D (Tencent API) for 3D model generation
4. Polls until the model is ready
5. Downloads the .glb and writes it to the `--output` path
6. Prints styled progress to the terminal so the audience can follow along

That's the entire scope. Prompt in, .glb out, with visible progress.

## What the MVP does NOT do

- Prompt refinement (raw prompt goes straight to image gen)
- Interactive checkpoints (runs straight through, no pauses)
- Post-processing / mesh decimation (use Hunyuan's raw output)
- Run management / persistence (no run directories, no meta.json)
- Config files (hardcode OpenAI + Hunyuan, API keys via env vars)
- Resume / retry (if it fails, run it again)
- Cost tracking
- Review UI
- JSON output mode
- Multiple providers or provider switching
- Direct text-to-3D mode (always goes image → 3D for the demo)

## Terminal output during demo

```
  ● HURAGOK — 3D Asset Pipeline

  Prompt:    M6D pistol, Halo CE style, matte gray

  ▸ Generating concept image... done (3.2s)
    Saved → .huragok/concept.png

  ▸ Generating 3D model via Hunyuan3D... done (47s)
    Faces: 32,400 | Vertices: 16,800

  ✓ Output → static/pistol.glb (4.2 MB)
```

## Project structure

```
huragok/
├── cmd/huragok/main.go              # Entry point + cobra root/create commands
├── internal/
│   ├── create/create.go             # Create command logic (the pipeline)
│   ├── provider/
│   │   ├── openai.go                # Image generation via go-openai SDK
│   │   └── hunyuan.go               # 3D generation via tencentcloud-sdk-go
│   └── display/display.go           # Styled terminal output via lipgloss
├── go.mod
└── go.sum
```

## Environment variables

```bash
export HURAGOK_OPENAI_KEY="sk-..."
export HURAGOK_HUNYUAN_SECRET_ID="..."
export HURAGOK_HUNYUAN_SECRET_KEY="..."
```

## Game-side changes (halo-portfolio repo)

The demo requires weapon switching in the arena so the generated pistol can be equipped as a secondary weapon. This is game code in halo-portfolio, not huragok code:

- Load a second GLB (the generated pistol) as a secondary `GunViewModel`
- Add a keypress (e.g., `2` or `Q`) to swap between primary AR and secondary pistol
- Different stats for the pistol (slower fire rate, more damage, smaller magazine)

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/charmbracelet/lipgloss` | Styled terminal output |
| `github.com/sashabaranov/go-openai` | OpenAI image generation |
| `github.com/tencentcloud/tencentcloud-sdk-go` | Hunyuan3D via Tencent API |

## Definition of done

1. `huragok create "M6D pistol, Halo CE style" --output static/pistol.glb` runs end-to-end and produces a valid .glb file
2. The terminal shows clear, styled progress at each step
3. The generated .glb loads in the halo-portfolio arena as a secondary weapon
4. The whole pipeline completes in under 2 minutes
