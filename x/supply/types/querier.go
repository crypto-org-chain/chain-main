package types

const (
	// QueryTotalSupply defines rest route for total supply
	QueryTotalSupply = "total_supply"

	// QueryLiquidSupply defines rest route for liquid supply
	QueryLiquidSupply = "liquid_supply"
)

// NewSupplyRequest returns a new supply request
func NewSupplyRequest() *SupplyRequest {
	return &SupplyRequest{}
}
