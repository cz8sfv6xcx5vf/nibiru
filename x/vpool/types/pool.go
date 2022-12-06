package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// HasEnoughQuoteReserve returns true if there is enough quote reserve based on
// quoteReserve * tradeLimitRatio
func (vpool *Vpool) HasEnoughQuoteReserve(quoteAmount sdk.Dec) bool {
	return vpool.QuoteAssetReserve.Mul(vpool.Config.TradeLimitRatio).GTE(quoteAmount.Abs())
}

// HasEnoughBaseReserve returns true if there is enough base reserve based on
// baseReserve * tradeLimitRatio
func (vpool *Vpool) HasEnoughBaseReserve(baseAmount sdk.Dec) bool {
	return vpool.BaseAssetReserve.Mul(vpool.Config.TradeLimitRatio).GTE(baseAmount.Abs())
}

func (vpool *Vpool) HasEnoughReservesForTrade(
	quoteAmtAbs sdk.Dec, baseAmtAbs sdk.Dec,
) (err error) {
	if !vpool.HasEnoughQuoteReserve(quoteAmtAbs) {
		return ErrOverTradingLimit.Wrapf(
			"quote amount %s is over trading limit", quoteAmtAbs)
	}
	if !vpool.HasEnoughBaseReserve(baseAmtAbs) {
		return ErrOverTradingLimit.Wrapf(
			"base amount %s is over trading limit", baseAmtAbs)
	}

	return nil
}

/*
GetBaseAmountByQuoteAmount returns the amount of base asset you will get out
by giving a specified amount of quote asset

args:
  - quoteDelta: the amount of quote asset to add to/remove from the pool.
    Adding to the quote reserves is synonymous with positive 'quoteDelta'.

ret:
  - baseOutAbs: the amount of base assets required to make this hypothetical swap
    always an absolute value
  - err: error
*/
func (vpool *Vpool) GetBaseAmountByQuoteAmount(
	quoteDelta sdk.Dec,
) (baseOutAbs sdk.Dec, err error) {
	if quoteDelta.IsZero() {
		return sdk.ZeroDec(), nil
	}

	invariant := vpool.QuoteAssetReserve.Mul(vpool.BaseAssetReserve) // x * y = k

	quoteReservesAfter := vpool.QuoteAssetReserve.Add(quoteDelta)
	if quoteReservesAfter.LTE(sdk.ZeroDec()) {
		return sdk.Dec{}, ErrQuoteReserveAtZero
	}

	baseReservesAfter := invariant.Quo(quoteReservesAfter)
	baseOutAbs = baseReservesAfter.Sub(vpool.BaseAssetReserve).Abs()

	return baseOutAbs, nil
}

/*
GetQuoteAmountByBaseAmount returns the amount of quote asset you will get out
by giving a specified amount of base asset

args:
  - dir: add to pool or remove from pool
  - baseAmount: the amount of base asset to add to/remove from the pool

ret:
  - quoteOutAbs: the amount of quote assets required to make this hypothetical swap
    always an absolute value
  - err: error
*/
func (vpool *Vpool) GetQuoteAmountByBaseAmount(
	baseDelta sdk.Dec,
) (quoteOutAbs sdk.Dec, err error) {
	if baseDelta.IsZero() {
		return sdk.ZeroDec(), nil
	}

	invariant := vpool.QuoteAssetReserve.Mul(vpool.BaseAssetReserve) // x * y = k

	baseReservesAfter := vpool.BaseAssetReserve.Add(baseDelta)
	if baseReservesAfter.LTE(sdk.ZeroDec()) {
		return sdk.Dec{}, ErrBaseReserveAtZero.Wrapf(
			"base assets below zero after trying to swap %s base assets",
			baseDelta.String(),
		)
	}

	quoteReservesAfter := invariant.Quo(baseReservesAfter)
	quoteOutAbs = quoteReservesAfter.Sub(vpool.QuoteAssetReserve).Abs()

	return quoteOutAbs, nil
}

// AddToQuoteAssetReserve adds 'amount' to the quote asset reserves
// The 'amount' is not assumed to be positive.
func (vpool *Vpool) AddToQuoteAssetReserve(amount sdk.Dec) {
	vpool.QuoteAssetReserve = vpool.QuoteAssetReserve.Add(amount)
}

// AddToBaseAssetReserve adds 'amount' to the base asset reserves
// The 'amount' is not assumed to be positive.
func (vpool *Vpool) AddToBaseAssetReserve(amount sdk.Dec) {
	vpool.BaseAssetReserve = vpool.BaseAssetReserve.Add(amount)
}

// ValidateReserves checks that reserves are positive.
func (vpool *Vpool) ValidateReserves() error {
	if !vpool.QuoteAssetReserve.IsPositive() || !vpool.BaseAssetReserve.IsPositive() {
		return ErrNonPositiveReserves.Wrap("pool: " + vpool.String())
	} else {
		return nil
	}
}

func (cfg *VpoolConfig) Validate() error {
	// trade limit ratio always between 0 and 1
	if cfg.TradeLimitRatio.LT(sdk.ZeroDec()) || cfg.TradeLimitRatio.GT(sdk.OneDec()) {
		return fmt.Errorf("trade limit ratio must be 0 <= ratio <= 1")
	}

	// fluctuation limit ratio between 0 and 1
	if cfg.FluctuationLimitRatio.LT(sdk.ZeroDec()) || cfg.FluctuationLimitRatio.GT(sdk.OneDec()) {
		return fmt.Errorf("fluctuation limit ratio must be 0 <= ratio <= 1")
	}

	// max oracle spread ratio between 0 and 1
	if cfg.MaxOracleSpreadRatio.LT(sdk.ZeroDec()) || cfg.MaxOracleSpreadRatio.GT(sdk.OneDec()) {
		return fmt.Errorf("max oracle spread ratio must be 0 <= ratio <= 1")
	}

	if cfg.MaintenanceMarginRatio.LT(sdk.ZeroDec()) || cfg.MaintenanceMarginRatio.GT(sdk.OneDec()) {
		return fmt.Errorf("maintenance margin ratio ratio must be 0 <= ratio <= 1")
	}

	if cfg.MaxLeverage.LTE(sdk.ZeroDec()) {
		return fmt.Errorf("max leverage must be > 0")
	}

	if sdk.OneDec().Quo(cfg.MaxLeverage).LT(cfg.MaintenanceMarginRatio) {
		return fmt.Errorf("margin ratio opened with max leverage position will be lower than Maintenance margin ratio")
	}

	return nil
}

func (vpool *Vpool) Validate() error {
	if err := vpool.Pair.Validate(); err != nil {
		return fmt.Errorf("invalid asset pair: %w", err)
	}

	// base asset reserve always > 0
	// quote asset reserve always > 0
	if err := vpool.ValidateReserves(); err != nil {
		return err
	}

	if err := vpool.Config.Validate(); err != nil {
		return err
	}

	return nil
}

// GetMarkPrice returns the price of the asset.
func (vpool Vpool) GetMarkPrice() sdk.Dec {
	if vpool.BaseAssetReserve.IsNil() || vpool.BaseAssetReserve.IsZero() ||
		vpool.QuoteAssetReserve.IsNil() || vpool.QuoteAssetReserve.IsZero() {
		return sdk.ZeroDec()
	}

	return vpool.QuoteAssetReserve.Quo(vpool.BaseAssetReserve)
}

/*
IsOverFluctuationLimitInRelationWithSnapshot compares the updated pool's spot price with the current spot price.

If the fluctuation limit ratio is zero, then the fluctuation limit check is skipped.

args:
  - pool: the updated vpool
  - snapshot: the snapshot to compare against

ret:
  - bool: true if the fluctuation limit is violated. false otherwise
*/
func (vpool Vpool) IsOverFluctuationLimitInRelationWithSnapshot(snapshot ReserveSnapshot) bool {
	if vpool.Config.FluctuationLimitRatio.IsZero() {
		return false
	}

	markPrice := vpool.GetMarkPrice()
	snapshotUpperLimit := snapshot.GetUpperMarkPriceFluctuationLimit(
		vpool.Config.FluctuationLimitRatio)
	snapshotLowerLimit := snapshot.GetLowerMarkPriceFluctuationLimit(
		vpool.Config.FluctuationLimitRatio)

	if markPrice.GT(snapshotUpperLimit) || markPrice.LT(snapshotLowerLimit) {
		return true
	}

	return false
}

/*
IsOverSpreadLimit compares the current mark price of the vpool
to the underlying's index price.
It panics if you provide it with a pair that doesn't exist in the state.

args:
  - indexPrice: the index price we want to compare.

ret:
  - bool: whether or not the price has deviated from the oracle price beyond a spread ratio
*/
func (vpool Vpool) IsOverSpreadLimit(indexPrice sdk.Dec) bool {
	return vpool.GetMarkPrice().Sub(indexPrice).
		Quo(indexPrice).Abs().GTE(vpool.Config.MaxOracleSpreadRatio)
}

func (vpool Vpool) ToSnapshot(ctx sdk.Context) ReserveSnapshot {
	snapshot := NewReserveSnapshot(
		vpool.Pair,
		vpool.BaseAssetReserve,
		vpool.QuoteAssetReserve,
		ctx.BlockTime(),
	)
	if err := snapshot.Validate(); err != nil {
		panic(err)
	}
	return snapshot
}

func (dir Direction) ToMultiplier() int64 {
	var dirMult int64
	switch dir {
	case Direction_ADD_TO_POOL, Direction_DIRECTION_UNSPECIFIED:
		dirMult = 1
	case Direction_REMOVE_FROM_POOL:
		dirMult = -1
	}
	return dirMult
}
