# huragok — 3D Asset Generation

Generate 3D models (.glb) from text descriptions using the huragok CLI.

## When to use

Use this skill when the user asks to:
- Create, generate, or make a 3D model or game asset
- Replace or regenerate an existing 3D model
- Turn a description into a .glb file

## Prerequisites

The following environment variables must be set:
- `HURAGOK_OPENAI_KEY` — OpenAI API key (for concept image generation)
- `HURAGOK_HUNYUAN_SECRET_ID` — Tencent Cloud SecretId
- `HURAGOK_HUNYUAN_SECRET_KEY` — Tencent Cloud SecretKey

If any are missing, tell the user which ones need to be set.

## Usage

```bash
huragok create "<prompt>" --output <path>
```

The pipeline runs automatically: text prompt → concept image (DALL-E 3) → 3D model (Hunyuan3D) → .glb file. Takes ~1-3 minutes total.

## Writing prompts

The prompt is sent to OpenAI DALL-E 3 for concept image generation. The tool auto-appends "single object, centered, isolated on plain white background, product photography style, no text" — you do NOT need to add this yourself.

**Content filter:** OpenAI blocks certain terms. You MUST avoid these words in prompts:
- BLOCKED: pistol, gun, rifle, weapon, shoot, bullet, ammunition
- USE INSTEAD: sidearm, handgun prop, blaster prop, energy device, handheld prop, game asset

Good prompt examples:
- `"sci-fi sidearm, compact futuristic handgun prop, sleek angular design, matte gray with blue energy accents, game asset"`
- `"futuristic sci-fi cargo crate, metal panels with glowing blue indicators, weathered surface, game prop"`
- `"alien energy blade, glowing plasma edge, ornate hilt, fantasy game prop"`
- `"military supply container, olive drab, stenciled markings, industrial game prop"`

## Intermediate artifacts

The tool creates a `.huragok/` directory in the current working directory containing:
- `concept.png` — the DALL-E 3 concept image used for 3D generation
- `model_raw.glb` — the raw model before copying to --output

## Error handling

- **Content filter error** → rephrase the prompt using the safe terms above
- **Billing/credit error** → tell the user to check their OpenAI or Tencent Cloud billing
- **Timeout** → run the command again, Hunyuan3D occasionally takes longer under load
- **Invalid GLB** → this should not happen with the Rapid endpoint; if it does, retry

## Example

User: "Make me a sci-fi crate model"

```bash
huragok create "futuristic sci-fi cargo crate, metal panels with glowing blue indicators, weathered surface, game prop" --output static/cargo_crate.glb
```

Then verify the output:
```bash
# Check it's a valid GLB (should start with "glTF")
xxd static/cargo_crate.glb | head -1
```
