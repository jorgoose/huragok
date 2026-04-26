package provider

// Per-call cost estimates in USD. Used for the --max-cost safety belt and
// for cost reporting in meta.json / --json output.
//
// These are estimates, not invoices. Real billing depends on response size,
// region, and current pricing. If a number here drifts more than a cent or
// two from reality, update it.
const (
	// DALL-E 3 standard quality, 1024x1024 — the size huragok always requests.
	// https://openai.com/api/pricing/
	DallE3StandardCostUSD = 0.040

	// Hunyuan3D Rapid via Tencent Cloud. TODO: verify against current pricing.
	// https://www.tencentcloud.com/products/hunyuan3d
	// This is a placeholder estimate — adjust if your actual invoices differ.
	HunyuanRapidCostUSD = 0.10
)
