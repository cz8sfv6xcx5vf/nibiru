package types

import sdk "github.com/cosmos/cosmos-sdk/types"

// CalcSpotPrice calculates the spot price based on weight.
// spotPrice = (BalanceIn / WeightIn) / (BalanceOut / WeightOut)
func (p Pool) CalcSpotPrice(tokenIn, tokenOut string) (sdk.Dec, error) {
	_, poolAssetIn, err := p.getPoolAssetAndIndex(tokenIn)
	if err != nil {
		return sdk.Dec{}, err
	}

	_, poolAssetOut, err := p.getPoolAssetAndIndex(tokenOut)
	if err != nil {
		return sdk.Dec{}, err
	}

	weightedBalanceIn := poolAssetIn.Token.Amount.ToDec().Quo(poolAssetIn.Weight.ToDec())
	weightedBalanceOut := poolAssetOut.Token.Amount.ToDec().Quo(poolAssetOut.Weight.ToDec())

	return weightedBalanceIn.Quo(weightedBalanceOut), nil
}
