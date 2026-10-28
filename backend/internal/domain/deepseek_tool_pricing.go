package domain

// DeepSeekToolPricing configures one successful Search or Fetch call.
// An absent tool entry keeps the legacy shared web-search price and group rate.
type DeepSeekToolPricing struct {
	Mode           string   `json:"mode"`
	PricePerCall   *float64 `json:"price_per_call"`
	RateMultiplier *float64 `json:"rate_multiplier,omitempty"`
}
