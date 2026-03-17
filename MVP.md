# MVP — AI Tinkerers Chicago Demo (March 17, 2026)

## Demo scenario

Run `huragok create` to generate a 3D game asset on the fly. The audience watches the full pipeline execute in real-time: text prompt → concept image → 3D model.

```bash
$ huragok create "sci-fi sidearm, compact futuristic handgun prop, sleek angular design, matte gray with blue energy accents, game asset" --output demo_sidearm.glb
```

## Important: OpenAI content filter

OpenAI's DALL-E 3 blocks prompts containing words like "pistol", "gun", "rifle", "weapon", "shoot". Use filter-friendly alternatives:

| Blocked | Use instead |
|---|---|
| pistol, gun | sidearm, handgun prop, handheld device |
| rifle | blaster prop, energy tool |
| weapon | game asset, prop, device |
| shoot | — |

Good demo prompts:
- `"sci-fi sidearm, compact futuristic handgun prop, sleek angular design, matte gray with blue energy accents, game asset"`
- `"futuristic plasma device, Halo-inspired handheld prop, metallic with glowing elements, game asset"`
- `"futuristic sci-fi cargo crate, metal panels with glowing blue indicators, weathered surface, game prop"`

## What the MVP does

1. Takes a text prompt from the user
2. Generates a concept image via OpenAI DALL-E 3 (auto-appends "isolated on white background" for clean 3D generation)
3. Sends the image URL to Hunyuan3D Rapid (Tencent Cloud intl API) for 3D model generation
4. Polls until the model is ready (~1-2 minutes)
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

  Prompt:  sci-fi sidearm, compact futuristic handgun prop...

  ▸ Generating concept image... done (13.3s)
    Saved → .huragok\concept.png

  ▸ Generating 3D model via Hunyuan3D... done (1m5s)
    Raw model: 10.3 MB

  ✓ Output → demo_sidearm.glb (10.3 MB)
```

## Project structure

```
huragok/
├── cmd/huragok/main.go              # Entry point + cobra root/create commands
├── internal/
│   ├── create/create.go             # Create command logic (the pipeline)
│   ├── provider/
│   │   ├── openai.go                # Image generation via go-openai SDK
│   │   └── hunyuan.go               # 3D generation via Tencent Cloud intl SDK
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

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/charmbracelet/lipgloss` | Styled terminal output |
| `github.com/sashabaranov/go-openai` | OpenAI image generation |
| `github.com/tencentcloud/tencentcloud-sdk-go-intl-en` | Hunyuan3D via Tencent Cloud intl API |

## Showing the result

After the .glb is generated, open it in https://gltf-viewer.donmccurdy.com/ (drag and drop) to show the audience the 3D model with textures. Rotate it, zoom in.

## If something goes wrong

- **Content filter blocks prompt** → rephrase without blocked terms (see table above)
- **Hunyuan3D times out** → run it again, or show a pre-generated .glb
- **Billing error** → check env vars are set, credits exist on OpenAI and Tencent Cloud

## Definition of done

1. `huragok create "<prompt>" --output demo_sidearm.glb` runs end-to-end and produces a valid .glb file
2. The terminal shows clear, styled progress at each step
3. The generated .glb opens correctly in a 3D viewer
4. The whole pipeline completes in under 3 minutes
