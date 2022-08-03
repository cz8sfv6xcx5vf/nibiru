package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/NibiruChain/nibiru/x/common"
)

const (
	ModuleName           = "perp"
	VaultModuleAccount   = "vault"
	PerpEFModuleAccount  = "perp_ef"
	FeePoolModuleAccount = "fee_pool"
)

// x/perp module sentinel errors
var (
	ErrMarginHighEnough                  = sdkerrors.Register(ModuleName, 1, "margin is higher than required maintenance margin ratio")
	ErrPositionNotFound                  = sdkerrors.Register(ModuleName, 2, "no position found")
	ErrPairNotFound                      = sdkerrors.Register(ModuleName, 3, "pair doesn't have live vpool")
	ErrPairMetadataNotFound              = sdkerrors.Register(ModuleName, 4, "pair doesn't have metadata")
	ErrPositionZero                      = sdkerrors.Register(ModuleName, 5, "position is zero")
	ErrFailedRemoveMarginCanCauseBadDebt = sdkerrors.Register(ModuleName, 7, "failed to remove margin; position would have bad debt if removed")
	ErrQuoteAmountIsZero                 = sdkerrors.Register(ModuleName, 8, "quote amount cannot be zero")
	ErrLeverageIsZero                    = sdkerrors.Register(ModuleName, 9, "leverage cannot be zero")
	ErrMarginRatioTooLow                 = sdkerrors.Register(ModuleName, 10, "margin ratio did not meet maintenance margin ratio")
)

func ZeroPosition(ctx sdk.Context, tokenPair common.AssetPair, traderAddr sdk.AccAddress) *Position {
	return &Position{
		TraderAddress:                       traderAddr.String(),
		Pair:                                tokenPair,
		Size_:                               sdk.ZeroDec(),
		Margin:                              sdk.ZeroDec(),
		OpenNotional:                        sdk.ZeroDec(),
		LastUpdateCumulativePremiumFraction: sdk.ZeroDec(),
		BlockNumber:                         ctx.BlockHeight(),
	}
}

func (l *LiquidateResp) Validate() error {
	nilFieldError := fmt.Errorf(
		`invalid liquidationOutput: %v,
				must not have nil fields`, l.String())

	// nil sdk.Int check
	for _, field := range []sdk.Int{
		l.FeeToLiquidator, l.FeeToPerpEcosystemFund} {
		if field.IsNil() {
			return nilFieldError
		}
	}

	// nil sdk.Int check
	for _, field := range []sdk.Int{l.BadDebt} {
		if field.IsNil() {
			return nilFieldError
		}
	}

	if _, err := sdk.AccAddressFromBech32(l.Liquidator); err != nil {
		return err
	}

	return nil
}
