package keeper

import (
	"fmt"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/NibiruChain/nibiru/x/common/asset"
	types "github.com/NibiruChain/nibiru/x/perp/v2/types"
)

// Extends the Keeper with admin functions. Admin is syntactic sugar to separate
// admin calls off from the other Keeper methods.
//
// These Admin functions should:
// 1. Not be wired into the MsgServer.
// 2. Not be called in other methods in the x/perp module.
// 3. Only be callable from nibiru/wasmbinding via sudo contracts.
//
// The intention here is to make it more obvious to the developer that an unsafe
// function is being used when it's called from the PerpKeeper.Admin struct.
type admin struct{ *Keeper }

/*
WithdrawFromInsuranceFund sends funds from the Insurance Fund to the given "to"
address.

Args:
- ctx: Blockchain context holding the current state
- amount: Amount of micro-NUSD to withdraw.
- to: Recipient address
*/
func (k admin) WithdrawFromInsuranceFund(
	ctx sdk.Context, amount sdkmath.Int, to sdk.AccAddress,
) (err error) {
	collateral, err := k.Collateral.Get(ctx)
	if err != nil {
		return err
	}

	coinToSend := sdk.NewCoin(collateral, amount)
	if err = k.BankKeeper.SendCoinsFromModuleToAccount(
		ctx,
		/* from */ types.PerpEFModuleAccount,
		/* to */ to,
		/* amount */ sdk.NewCoins(coinToSend),
	); err != nil {
		return err
	}
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"withdraw_from_if",
		sdk.NewAttribute("to", to.String()),
		sdk.NewAttribute("funds", coinToSend.String()),
	))
	return nil
}

type ArgsCreateMarket struct {
	Pair            asset.Pair
	PriceMultiplier sdk.Dec
	SqrtDepth       sdk.Dec
	Market          *types.Market // pointer makes it optional
	// EnableMarket: Optionally enable the default market without explicitly passing
	// in each field as an argument. If 'Market' is present, this field is ignored.
	EnableMarket bool
}

// CreateMarket creates a pool for a specific pair.
func (k admin) CreateMarket(
	ctx sdk.Context,
	args ArgsCreateMarket,
) error {
	pair := args.Pair
	market, err := k.GetMarket(ctx, pair)
	if err == nil && market.Enabled {
		return fmt.Errorf("market %s already exists and it is enabled", pair)
	}

	// init market
	sqrtDepth := args.SqrtDepth
	quoteReserve := sqrtDepth
	baseReserve := sqrtDepth
	if args.Market == nil {
		market = types.DefaultMarket(pair)
		market.Enabled = args.EnableMarket
	} else {
		market = *args.Market
	}
	if err := market.Validate(); err != nil {
		return err
	}

	lastVersion := k.MarketLastVersion.GetOr(ctx, pair, types.MarketLastVersion{Version: 0})
	lastVersion.Version += 1
	market.Version = lastVersion.Version

	// init amm
	amm := types.AMM{
		Pair:            pair,
		Version:         lastVersion.Version,
		BaseReserve:     baseReserve,
		QuoteReserve:    quoteReserve,
		SqrtDepth:       sqrtDepth,
		PriceMultiplier: args.PriceMultiplier,
		TotalLong:       sdk.ZeroDec(),
		TotalShort:      sdk.ZeroDec(),
	}
	if err := amm.Validate(); err != nil {
		return err
	}

	k.SaveMarket(ctx, market)
	k.SaveAMM(ctx, amm)
	k.MarketLastVersion.Insert(ctx, pair, lastVersion)

	return nil
}

// CloseMarket closes the market. From now on, no new position can be opened on
// this market or closed. Only the open positions can be settled by calling
// SettlePosition.
func (k admin) CloseMarket(ctx sdk.Context, pair asset.Pair) (err error) {
	market, err := k.GetMarket(ctx, pair)
	if err != nil {
		return err
	}
	if !market.Enabled {
		return types.ErrMarketNotEnabled
	}

	amm, err := k.GetAMM(ctx, pair)
	if err != nil {
		return err
	}

	settlementPrice, _, err := amm.ComputeSettlementPrice()
	if err != nil {
		return
	}

	amm.SettlementPrice = settlementPrice
	market.Enabled = false

	k.SaveAMM(ctx, amm)
	k.SaveMarket(ctx, market)

	return nil
}

// ChangeCollateralDenom: Updates the collateral denom. A denom is valid if it is
// possible to make an sdk.Coin using it. [Admin] Only callable by sudoers.
func (k admin) ChangeCollateralDenom(
	ctx sdk.Context,
	denom string,
	sender sdk.AccAddress,
) error {
	if err := k.SudoKeeper.CheckPermissions(sender, ctx); err != nil {
		return err
	}
	return k.UnsafeChangeCollateralDenom(ctx, denom)
}

// UnsafeChangeCollateralDenom: Used in the genesis to set the collateral
// without requiring an explicit call from sudoers.
func (k admin) UnsafeChangeCollateralDenom(
	ctx sdk.Context,
	denom string,
) error {
	if err := sdk.ValidateDenom(denom); err != nil {
		return types.ErrInvalidCollateral.Wrap(err.Error())
	}
	k.Collateral.Set(ctx, denom)
	return nil
}

// ShiftPegMultiplier: Edit the peg multiplier of an amm pool after making sure
// there's enough money in the perp fund to pay for the repeg. These funds get
// send to the vault to pay for trader's new net margin.
func (k admin) ShiftPegMultiplier(
	ctx sdk.Context,
	pair asset.Pair,
	newPriceMultiplier sdk.Dec,
	sender sdk.AccAddress,
) error {
	if err := k.SudoKeeper.CheckPermissions(sender, ctx); err != nil {
		return err
	}

	amm, err := k.GetAMM(ctx, pair)
	if err != nil {
		return err
	}
	oldPriceMult := amm.PriceMultiplier

	if newPriceMultiplier.Equal(oldPriceMult) {
		// same price multiplier, no-op
		return nil
	}

	// Compute cost of re-pegging the pool
	cost, err := amm.CalcRepegCost(newPriceMultiplier)
	if err != nil {
		return err
	}

	costPaid, err := k.handleMarketUpdateCost(ctx, pair, cost)
	if err != nil {
		return err
	}

	// Do the re-peg
	amm.PriceMultiplier = newPriceMultiplier
	k.SaveAMM(ctx, amm)

	return ctx.EventManager().EmitTypedEvent(&types.EventShiftPegMultiplier{
		OldPegMultiplier: oldPriceMult,
		NewPegMultiplier: newPriceMultiplier,
		CostPaid:         costPaid,
	})
}

// ShiftSwapInvariant: Edit the swap invariant (liquidity depth) of an amm pool,
// ensuring that there's enough money in the perp  fund to pay for the operation.
// These funds get send to the vault to pay for trader's new net margin.
func (k admin) ShiftSwapInvariant(
	ctx sdk.Context,
	pair asset.Pair,
	newSwapInvariant sdkmath.Int,
	sender sdk.AccAddress,
) error {
	if err := k.SudoKeeper.CheckPermissions(sender, ctx); err != nil {
		return err
	}
	amm, err := k.GetAMM(ctx, pair)
	if err != nil {
		return err
	}

	cost, err := amm.CalcUpdateSwapInvariantCost(newSwapInvariant.ToLegacyDec())
	if err != nil {
		return err
	}

	costPaid, err := k.handleMarketUpdateCost(ctx, pair, cost)
	if err != nil {
		return err
	}

	err = amm.UpdateSwapInvariant(newSwapInvariant.ToLegacyDec())
	if err != nil {
		return err
	}

	k.SaveAMM(ctx, amm)

	return ctx.EventManager().EmitTypedEvent(&types.EventShiftSwapInvariant{
		OldSwapInvariant: amm.BaseReserve.Mul(amm.QuoteReserve).RoundInt(),
		NewSwapInvariant: newSwapInvariant,
		CostPaid:         costPaid,
	})
}
