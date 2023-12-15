package types

import (
	"math/big"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/common/asset"
)

func (amm AMM) Validate() error {
	if amm.BaseReserve.LTE(sdk.ZeroDec()) {
		return ErrAmmBaseSupplyNonpositive
	}

	if amm.QuoteReserve.LTE(sdk.ZeroDec()) {
		return ErrAmmQuoteSupplyNonpositive
	}

	if amm.PriceMultiplier.LTE(sdk.ZeroDec()) {
		return ErrAmmNonPositivePegMult
	}

	if amm.SqrtDepth.LTE(sdk.ZeroDec()) {
		return ErrAmmNonPositiveSwapInvariant
	}

	computedSqrtDepth, err := amm.ComputeSqrtDepth()
	if err != nil {
		return err
	}

	if !amm.SqrtDepth.Sub(computedSqrtDepth).Abs().LTE(sdk.OneDec()) {
		return ErrLiquidityDepth.Wrap(
			"computed sqrt and current sqrt are mismatched. pool: " + amm.String())
	}

	return nil
}

// ComputeSettlementPrice computes the uniform settlement price for the current AMM.
//
// Returns:
//   - price: uniform settlement price from several batched trades. In this case,
//     "price" is the result of closing all positions together and giving
//     all traders the same price.
//   - newAmm: The AMM that results from closing all positions together. Note that
//     this should have a bias, or skew, of 0.
//   - err: Errors if it's impossible to swap away the open interest bias.
func (amm AMM) ComputeSettlementPrice() (sdk.Dec, AMM, error) {
	// bias: open interest (base) skew in the AMM.
	bias := amm.Bias()
	if bias.IsZero() {
		return amm.InstMarkPrice(), amm, nil
	}

	var dir Direction
	if bias.IsPositive() {
		dir = Direction_SHORT
	} else {
		dir = Direction_LONG
	}

	quoteAssetDelta, err := amm.SwapBaseAsset(bias.Abs(), dir)
	if err != nil {
		return sdk.Dec{}, AMM{}, err
	}
	price := quoteAssetDelta.Abs().Quo(bias.Abs())

	return price, amm, err
}

// QuoteReserveToAsset converts quote reserves to assets
func (amm AMM) QuoteReserveToAsset(quoteReserve sdk.Dec) sdk.Dec {
	return QuoteReserveToAsset(quoteReserve, amm.PriceMultiplier)
}

// QuoteAssetToReserve converts quote assets to reserves
func (amm AMM) QuoteAssetToReserve(quoteAssets sdk.Dec) sdk.Dec {
	return QuoteAssetToReserve(quoteAssets, amm.PriceMultiplier)
}

// QuoteAssetToReserve converts "quote assets" to "quote reserves". In this
// convention, "assets" are liquid funds that change hands, whereas reserves
// are simply a number field on the DAMM. The reason for this distinction is to
// account for the AMM.PriceMultiplier.
func QuoteAssetToReserve(quoteAsset, priceMult sdk.Dec) sdk.Dec {
	return quoteAsset.Quo(priceMult)
}

// QuoteReserveToAsset converts "quote reserves" to "quote assets". In this
// convention, "assets" are liquid funds that change hands, whereas reserves
// are simply a number field on the DAMM. The reason for this distinction is to
// account for the AMM.PriceMultiplier.
func QuoteReserveToAsset(quoteReserve, priceMult sdk.Dec) sdk.Dec {
	return quoteReserve.Mul(priceMult)
}

// GetBaseReserveAmt Returns the amount of base reserve equivalent to the amount of quote reserve given
//
// args:
// - quoteReserveAmt: the amount of quote reserve before the trade, must be positive
// - dir: the direction of the trade
//
// returns:
// - baseReserveDelta: the amount of base reserve after the trade, unsigned
// - err: error
//
// NOTE: baseReserveDelta is always positive
// Throws an error if input quoteReserveAmt is negative, or if the final quote reserve is not positive
func (amm AMM) GetBaseReserveAmt(
	quoteReserveAmt sdk.Dec, // unsigned
	dir Direction,
) (baseReserveDelta sdk.Dec, err error) {
	if quoteReserveAmt.IsNegative() {
		return sdk.Dec{}, ErrInputQuoteAmtNegative
	}

	invariant := amm.QuoteReserve.Mul(amm.BaseReserve) // x * y = k

	var quoteReservesAfter sdk.Dec
	if dir == Direction_LONG {
		quoteReservesAfter = amm.QuoteReserve.Add(quoteReserveAmt)
	} else {
		quoteReservesAfter = amm.QuoteReserve.Sub(quoteReserveAmt)
	}

	if !quoteReservesAfter.IsPositive() {
		return sdk.Dec{}, ErrAmmNonpositiveReserves
	}

	baseReservesAfter := invariant.Quo(quoteReservesAfter)
	baseReserveDelta = baseReservesAfter.Sub(amm.BaseReserve).Abs()

	return baseReserveDelta, nil
}

// GetQuoteReserveAmt returns the amount of quote reserve equivalent to the amount of base asset given
//
// args:
// - baseReserveAmt: the amount of base reserves to trade, must be positive
// - dir: the direction of the trade
//
// returns:
// - quoteReserveDelta: the amount of quote reserve after the trade
// - err: error
//
// NOTE: quoteReserveDelta is always positive
func (amm AMM) GetQuoteReserveAmt(
	baseReserveAmt sdk.Dec,
	dir Direction,
) (quoteReserveDelta sdk.Dec, err error) {
	if baseReserveAmt.IsNegative() {
		return sdk.Dec{}, ErrInputBaseAmtNegative
	}
	if baseReserveAmt.IsZero() {
		return sdk.ZeroDec(), nil
	}

	invariant := amm.QuoteReserve.Mul(amm.BaseReserve) // x * y = k

	var baseReservesAfter sdk.Dec
	if dir == Direction_LONG {
		baseReservesAfter = amm.BaseReserve.Sub(baseReserveAmt)
	} else {
		baseReservesAfter = amm.BaseReserve.Add(baseReserveAmt)
	}

	if !baseReservesAfter.IsPositive() {
		return sdk.Dec{}, ErrAmmNonpositiveReserves.Wrapf(
			"base assets below zero (%s) after trying to swap %s base assets",
			baseReservesAfter.String(),
			baseReserveAmt.String(),
		)
	}

	quoteReservesAfter := invariant.Quo(baseReservesAfter)
	quoteReserveDelta = quoteReservesAfter.Sub(amm.QuoteReserve).Abs()

	return quoteReserveDelta, nil
}

// InstMarkPrice returns the instantaneous mark price of the trading pair.
// This is the price if the AMM has zero slippage, or equivalently, if there's
// infinite liquidity depth with the same ratio of reserves.
func (amm AMM) InstMarkPrice() sdk.Dec {
	if amm.BaseReserve.IsNil() || amm.BaseReserve.IsZero() ||
		amm.QuoteReserve.IsNil() || amm.QuoteReserve.IsZero() {
		return sdk.ZeroDec()
	}

	return amm.QuoteReserve.Quo(amm.BaseReserve).Mul(amm.PriceMultiplier)
}

// ComputeSqrtDepth returns the sqrt of the product of the reserves
func (amm AMM) ComputeSqrtDepth() (sqrtDepth sdk.Dec, err error) {
	liqDepthBigInt := new(big.Int).Mul(
		amm.QuoteReserve.BigInt(), amm.BaseReserve.BigInt(),
	)
	chopped := common.ChopPrecisionAndRound(liqDepthBigInt)
	if chopped.BitLen() > common.MaxDecBitLen {
		return sdk.Dec{}, ErrAmmLiquidityDepthOverflow
	}
	liqDepth := amm.QuoteReserve.Mul(amm.BaseReserve)
	return common.SqrtDec(liqDepth)
}

func (amm *AMM) WithPair(pair asset.Pair) *AMM {
	amm.Pair = pair
	return amm
}

func (amm *AMM) WithBaseReserve(baseReserve sdk.Dec) *AMM {
	amm.BaseReserve = baseReserve
	return amm
}

func (amm *AMM) WithQuoteReserve(quoteReserve sdk.Dec) *AMM {
	amm.QuoteReserve = quoteReserve
	return amm
}

func (amm *AMM) WithPriceMultiplier(priceMultiplier sdk.Dec) *AMM {
	amm.PriceMultiplier = priceMultiplier
	return amm
}

func (amm *AMM) WithTotalLong(totalLong sdk.Dec) *AMM {
	amm.TotalLong = totalLong
	return amm
}

func (amm *AMM) WithTotalShort(totalShort sdk.Dec) *AMM {
	amm.TotalShort = totalShort
	return amm
}

func (amm *AMM) WithSqrtDepth(sqrtDepth sdk.Dec) *AMM {
	amm.SqrtDepth = sqrtDepth
	return amm
}

// SwapQuoteAsset swaps base asset for quote asset
//
// args:
// - quoteAssetAmt: amount of base asset to swap, must be positive
// - dir: direction the user takes
//
// returns:
// - baseAssetDelta: amount of base asset received
// - err: error
//
// NOTE: baseAssetDelta is always positive
func (amm *AMM) SwapQuoteAsset(
	quoteAssetAmt sdk.Dec, // unsigned
	dir Direction,
) (baseAssetDelta sdk.Dec, err error) {
	quoteReserveAmt := QuoteAssetToReserve(quoteAssetAmt, amm.PriceMultiplier)
	baseReserveDelta, err := amm.GetBaseReserveAmt(quoteReserveAmt, dir)
	if err != nil {
		return sdk.Dec{}, err
	}

	if dir == Direction_LONG {
		amm.QuoteReserve = amm.QuoteReserve.Add(quoteReserveAmt)
		amm.BaseReserve = amm.BaseReserve.Sub(baseReserveDelta)
		amm.TotalLong = amm.TotalLong.Add(baseReserveDelta)
	} else if dir == Direction_SHORT {
		amm.QuoteReserve = amm.QuoteReserve.Sub(quoteReserveAmt)
		amm.BaseReserve = amm.BaseReserve.Add(baseReserveDelta)
		amm.TotalShort = amm.TotalShort.Add(baseReserveDelta)
	}

	return baseReserveDelta, nil
}

// SwapBaseAsset swaps base asset for quote asset
//
// args:
//   - baseAssetAmt: amount of base asset to swap, must be positive
//   - dir: direction of swap
//
// returns:
//   - quoteAssetDelta: amount of quote asset received. Always positive
//   - err: error if any
func (amm *AMM) SwapBaseAsset(baseAssetAmt sdk.Dec, dir Direction) (quoteAssetDelta sdk.Dec, err error) {
	quoteReserveDelta, err := amm.GetQuoteReserveAmt(baseAssetAmt, dir)
	if err != nil {
		return sdk.Dec{}, err
	}

	if dir == Direction_LONG {
		amm.QuoteReserve = amm.QuoteReserve.Add(quoteReserveDelta)
		amm.BaseReserve = amm.BaseReserve.Sub(baseAssetAmt)
		amm.TotalLong = amm.TotalLong.Add(baseAssetAmt)
	} else if dir == Direction_SHORT {
		amm.QuoteReserve = amm.QuoteReserve.Sub(quoteReserveDelta)
		amm.BaseReserve = amm.BaseReserve.Add(baseAssetAmt)
		amm.TotalShort = amm.TotalShort.Add(baseAssetAmt)
	}

	return amm.QuoteReserveToAsset(quoteReserveDelta), nil
}

// Bias: returns the bias, or open interest skew, of the market in the base
// units. Bias is the net amount of long perpetual contracts minus the net
// amount of shorts.
func (amm *AMM) Bias() (bias sdk.Dec) {
	return amm.TotalLong.Sub(amm.TotalShort)
}

/*
CalcRepegCost provides the cost of re-pegging the pool to a new candidate peg multiplier.
*/
func (amm AMM) CalcRepegCost(newPriceMultiplier sdk.Dec) (cost sdkmath.Int, err error) {
	if !newPriceMultiplier.IsPositive() {
		return sdkmath.Int{}, ErrAmmNonPositivePegMult
	}

	bias := amm.Bias()

	if bias.IsZero() {
		return sdk.ZeroInt(), nil
	}

	var dir Direction
	if bias.IsPositive() {
		dir = Direction_SHORT
	} else {
		dir = Direction_LONG
	}

	biasInQuoteReserve, err := amm.GetQuoteReserveAmt(bias.Abs(), dir)
	if err != nil {
		return sdkmath.Int{}, err
	}

	costDec := biasInQuoteReserve.Mul(newPriceMultiplier.Sub(amm.PriceMultiplier))

	if bias.IsNegative() {
		costDec = costDec.Neg()
	}

	return costDec.Ceil().TruncateInt(), nil
}

// returns the amount of quote assets the amm has to pay out if all longs and shorts close out their positions
// positive value means the amm has to pay out quote assets
// negative value means the amm has to receive quote assets
func (amm AMM) GetMarketValue() (sdk.Dec, error) {
	bias := amm.Bias()

	if bias.IsZero() {
		return sdk.ZeroDec(), nil
	}

	var dir Direction
	if bias.IsPositive() {
		dir = Direction_SHORT
	} else {
		dir = Direction_LONG
	}

	marketValueInReserves, err := amm.GetQuoteReserveAmt(bias.Abs(), dir)
	if err != nil {
		return sdk.Dec{}, err
	}

	if bias.IsNegative() {
		marketValueInReserves = marketValueInReserves.Neg()
	}

	return amm.QuoteReserveToAsset(marketValueInReserves), nil
}

/*
CalcUpdateSwapInvariantCost returns the cost of updating the invariant of the pool
*/
func (amm AMM) CalcUpdateSwapInvariantCost(newSwapInvariant sdk.Dec) (sdkmath.Int, error) {
	if newSwapInvariant.IsNil() {
		return sdkmath.Int{}, ErrNilSwapInvariant
	}

	if !newSwapInvariant.IsPositive() {
		return sdkmath.Int{}, ErrAmmNonPositiveSwapInvariant
	}

	marketValueBefore, err := amm.GetMarketValue()
	if err != nil {
		return sdkmath.Int{}, err
	}

	err = amm.UpdateSwapInvariant(newSwapInvariant)
	if err != nil {
		return sdkmath.Int{}, err
	}

	marketValueAfter, err := amm.GetMarketValue()
	if err != nil {
		return sdkmath.Int{}, err
	}

	cost := marketValueAfter.Sub(marketValueBefore)

	return cost.Ceil().TruncateInt(), nil
}

// UpdateSwapInvariant updates the swap invariant of the amm
func (amm *AMM) UpdateSwapInvariant(newSwapInvariant sdk.Dec) (err error) {
	// k = x * y
	// newK = (cx) * (cy) = c^2 xy = c^2 k
	// newPrice = (c y) / (c x) = y / x = price | unchanged price
	newSqrtDepth := common.MustSqrtDec(newSwapInvariant)
	multiplier := newSqrtDepth.Quo(amm.SqrtDepth)

	// Change the swap invariant while holding price constant.
	// Multiplying by the same factor to both of the reserves won't affect price.
	amm.SqrtDepth = newSqrtDepth
	amm.BaseReserve = amm.BaseReserve.Mul(multiplier)
	amm.QuoteReserve = amm.QuoteReserve.Mul(multiplier)

	return amm.Validate() // might be useless
}
