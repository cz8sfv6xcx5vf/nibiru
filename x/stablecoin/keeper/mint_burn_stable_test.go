package keeper_test

import (
	"testing"
	"time"

	"github.com/NibiruChain/nibiru/x/common"
	pricefeedTypes "github.com/NibiruChain/nibiru/x/pricefeed/types"
	"github.com/NibiruChain/nibiru/x/stablecoin/types"

	"github.com/NibiruChain/nibiru/x/testutil/sample"
	"github.com/NibiruChain/nibiru/x/testutil/testapp"

	"github.com/cosmos/cosmos-sdk/simapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ------------------------------------------------------------------
// MintStable
// ------------------------------------------------------------------

func TestMsgMint_ValidateBasic(t *testing.T) {
	testCases := []struct {
		name string
		msg  types.MsgMintStable
		err  error
	}{
		{
			name: "invalid address",
			msg: types.MsgMintStable{
				Creator: "invalid_address",
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "valid address",
			msg: types.MsgMintStable{
				Creator: sample.AccAddress().String(),
			},
		},
	}
	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMsgMintStableResponse_HappyPath(t *testing.T) {
	accFundsGovAmount := sdk.NewCoin(common.DenomGov, sdk.NewInt(10_000))
	accFundsCollAmount := sdk.NewCoin(common.DenomColl, sdk.NewInt(900_000))
	neededGovFees := sdk.NewCoin(common.DenomGov, sdk.NewInt(20))      // 0.002 fee
	neededCollFees := sdk.NewCoin(common.DenomColl, sdk.NewInt(1_800)) // 0.002 fee

	accFundsAmt := sdk.NewCoins(
		accFundsGovAmount.Add(neededGovFees),
		accFundsCollAmount.Add(neededCollFees),
	)

	tests := []struct {
		name                   string
		accFunds               sdk.Coins
		msgMint                types.MsgMintStable
		msgResponse            types.MsgMintStableResponse
		govPrice               sdk.Dec
		collPrice              sdk.Dec
		supplyNIBI             sdk.Coin
		supplyNUSD             sdk.Coin
		err                    error
		isCollateralRatioValid bool
	}{
		{
			name:     "Not able to mint because of no posted prices",
			accFunds: accFundsAmt,
			msgMint: types.MsgMintStable{
				Creator: sample.AccAddress().String(),
				Stable:  sdk.NewCoin(common.DenomStable, sdk.NewInt(1_000_000)),
			},
			govPrice:               sdk.MustNewDecFromStr("10"),
			collPrice:              sdk.MustNewDecFromStr("1"),
			err:                    types.NoValidCollateralRatio,
			isCollateralRatioValid: false,
		},
		{
			name:     "Successful mint",
			accFunds: accFundsAmt,
			msgMint: types.MsgMintStable{
				Creator: sample.AccAddress().String(),
				Stable:  sdk.NewCoin(common.DenomStable, sdk.NewInt(1_000_000)),
			},
			msgResponse: types.MsgMintStableResponse{
				Stable:    sdk.NewCoin(common.DenomStable, sdk.NewInt(1_000_000)),
				UsedCoins: sdk.NewCoins(accFundsCollAmount, accFundsGovAmount),
				FeesPayed: sdk.NewCoins(neededCollFees, neededGovFees),
			},
			govPrice:   sdk.MustNewDecFromStr("10"),
			collPrice:  sdk.MustNewDecFromStr("1"),
			supplyNIBI: sdk.NewCoin(common.DenomGov, sdk.NewInt(10)),
			// 10_000 - 20 (neededAmt - fees) - 10 (0.5 of fees from EFund are burned)
			supplyNUSD:             sdk.NewCoin(common.DenomStable, sdk.NewInt(1_000_000)),
			err:                    nil,
			isCollateralRatioValid: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			nibiruApp, ctx := testapp.NewNibiruAppAndContext(true)
			acc, _ := sdk.AccAddressFromBech32(tc.msgMint.Creator)
			oracle := sample.AccAddress()

			// We get module account, to create it.
			nibiruApp.AccountKeeper.GetModuleAccount(ctx, types.StableEFModuleAccount)

			// Set up pairs for the pricefeed keeper.
			priceKeeper := &nibiruApp.PricefeedKeeper
			pairs := common.AssetPairs{
				{Token0: common.DenomGov, Token1: common.DenomStable},
				{Token0: common.DenomColl, Token1: common.DenomStable},
			}
			pfParams := pricefeedTypes.Params{Pairs: pairs}
			priceKeeper.SetParams(ctx, pfParams)
			priceKeeper.WhitelistOracles(ctx, []sdk.AccAddress{oracle})

			collRatio := sdk.MustNewDecFromStr("0.9")
			feeRatio := sdk.MustNewDecFromStr("0.002")
			feeRatioEF := sdk.MustNewDecFromStr("0.5")
			bonusRateRecoll := sdk.MustNewDecFromStr("0.002")
			adjustmentStep := sdk.MustNewDecFromStr("0.0025")
			priceLowerBound := sdk.MustNewDecFromStr("0.9999")
			priceUpperBound := sdk.MustNewDecFromStr("1.0001")

			nibiruApp.StablecoinKeeper.SetParams(
				ctx, types.NewParams(
					collRatio,
					feeRatio,
					feeRatioEF,
					bonusRateRecoll,
					"15 min",
					adjustmentStep,
					priceLowerBound,
					priceUpperBound,
					tc.isCollateralRatioValid,
				),
			)

			// Post prices to each pair with the oracle.
			priceExpiry := ctx.BlockTime().Add(time.Hour)
			_, err := priceKeeper.PostRawPrice(
				ctx, oracle, common.PairGovStable.String(), tc.govPrice, priceExpiry,
			)
			require.NoError(t, err)
			_, err = priceKeeper.PostRawPrice(
				ctx, oracle, common.PairCollStable.String(), tc.collPrice, priceExpiry,
			)
			require.NoError(t, err)

			// Update the 'CurrentPrice' posted by the oracles.
			for _, pair := range pfParams.Pairs {
				err = priceKeeper.GatherRawPrices(ctx, pair.Token0, pair.Token1)
				require.NoError(t, err, "Error posting price for pair: %d", pair.String())
			}

			// Fund account
			err = simapp.FundAccount(nibiruApp.BankKeeper, ctx, acc, tc.accFunds)
			require.NoError(t, err)

			// Mint NUSD -> Response contains Stable (sdk.Coin)
			goCtx := sdk.WrapSDKContext(ctx)
			mintStableResponse, err := nibiruApp.StablecoinKeeper.MintStable(
				goCtx, &tc.msgMint)

			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}

			require.NoError(t, err)
			assert.EqualValues(t, tc.msgResponse, *mintStableResponse)
			assert.Equal(t, nibiruApp.StablecoinKeeper.GetSupplyNIBI(ctx), tc.supplyNIBI)
			assert.Equal(t, nibiruApp.StablecoinKeeper.GetSupplyNUSD(ctx), tc.supplyNUSD)

			// Check balances in EF
			efModuleBalance := nibiruApp.BankKeeper.GetAllBalances(
				ctx, nibiruApp.AccountKeeper.GetModuleAddress(types.StableEFModuleAccount),
			)
			collFeesInEf := neededCollFees.Amount.ToDec().Mul(sdk.MustNewDecFromStr("0.5")).TruncateInt()
			assert.Equal(t, sdk.NewCoins(sdk.NewCoin(common.DenomColl, collFeesInEf)), efModuleBalance)

			// Check balances in Treasury
			treasuryModuleBalance := nibiruApp.BankKeeper.
				GetAllBalances(ctx, nibiruApp.AccountKeeper.GetModuleAddress(common.TreasuryPoolModuleAccount))
			collFeesInTreasury := neededCollFees.Amount.ToDec().Mul(sdk.MustNewDecFromStr("0.5")).TruncateInt()
			govFeesInTreasury := neededGovFees.Amount.ToDec().Mul(sdk.MustNewDecFromStr("0.5")).TruncateInt()
			assert.Equal(
				t,
				sdk.NewCoins(
					sdk.NewCoin(common.DenomColl, collFeesInTreasury),
					sdk.NewCoin(common.DenomGov, govFeesInTreasury),
				),
				treasuryModuleBalance,
			)
		})
	}
}

func TestMsgMintStableResponse_NotEnoughFunds(t *testing.T) {
	testCases := []struct {
		name        string
		accFunds    sdk.Coins
		msgMint     types.MsgMintStable
		msgResponse types.MsgMintStableResponse
		govPrice    sdk.Dec
		collPrice   sdk.Dec
		err         error
	}{
		{
			name: "User has no GOV",
			accFunds: sdk.NewCoins(
				sdk.NewCoin(common.DenomColl, sdk.NewInt(9001)),
				sdk.NewCoin(common.DenomGov, sdk.NewInt(0)),
			),
			msgMint: types.MsgMintStable{
				Creator: sample.AccAddress().String(),
				Stable:  sdk.NewCoin(common.DenomStable, sdk.NewInt(100)),
			},
			msgResponse: types.MsgMintStableResponse{
				Stable: sdk.NewCoin(common.DenomStable, sdk.NewInt(0)),
			},
			govPrice:  sdk.MustNewDecFromStr("10"),
			collPrice: sdk.MustNewDecFromStr("1"),
			err:       types.NotEnoughBalance.Wrap(common.DenomGov),
		}, {
			name: "User has no COLL",
			accFunds: sdk.NewCoins(
				sdk.NewCoin(common.DenomColl, sdk.NewInt(0)),
				sdk.NewCoin(common.DenomGov, sdk.NewInt(9001)),
			),
			msgMint: types.MsgMintStable{
				Creator: sample.AccAddress().String(),
				Stable:  sdk.NewCoin(common.DenomStable, sdk.NewInt(100)),
			},
			msgResponse: types.MsgMintStableResponse{
				Stable: sdk.NewCoin(common.DenomStable, sdk.NewInt(0)),
			},
			govPrice:  sdk.MustNewDecFromStr("10"),
			collPrice: sdk.MustNewDecFromStr("1"),
			err:       types.NotEnoughBalance.Wrap(common.DenomColl),
		},
		{
			name: "Not enough GOV",
			accFunds: sdk.NewCoins(
				sdk.NewCoin(common.DenomColl, sdk.NewInt(9001)),
				sdk.NewCoin(common.DenomGov, sdk.NewInt(1)),
			),
			msgMint: types.MsgMintStable{
				Creator: sample.AccAddress().String(),
				Stable:  sdk.NewCoin(common.DenomStable, sdk.NewInt(1000)),
			},
			msgResponse: types.MsgMintStableResponse{
				Stable: sdk.NewCoin(common.DenomStable, sdk.NewInt(0)),
			},
			govPrice:  sdk.MustNewDecFromStr("10"),
			collPrice: sdk.MustNewDecFromStr("1"),
			err: types.NotEnoughBalance.Wrap(
				sdk.NewCoin(common.DenomGov, sdk.NewInt(1)).String()),
		}, {
			name: "Not enough COLL",
			accFunds: sdk.NewCoins(
				sdk.NewCoin(common.DenomColl, sdk.NewInt(1)),
				sdk.NewCoin(common.DenomGov, sdk.NewInt(9001)),
			),
			msgMint: types.MsgMintStable{
				Creator: sample.AccAddress().String(),
				Stable:  sdk.NewCoin(common.DenomStable, sdk.NewInt(100)),
			},
			msgResponse: types.MsgMintStableResponse{
				Stable: sdk.NewCoin(common.DenomStable, sdk.NewInt(0)),
			},
			govPrice:  sdk.MustNewDecFromStr("10"),
			collPrice: sdk.MustNewDecFromStr("1"),
			err: types.NotEnoughBalance.Wrap(
				sdk.NewCoin(common.DenomColl, sdk.NewInt(1)).String()),
		},
	}

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			nibiruApp, ctx := testapp.NewNibiruAppAndContext(true)
			acc, _ := sdk.AccAddressFromBech32(tc.msgMint.Creator)
			oracle := sample.AccAddress()

			// We get module account, to create it.
			nibiruApp.AccountKeeper.GetModuleAccount(ctx, types.StableEFModuleAccount)

			// Set up pairs for the pricefeed keeper.
			priceKeeper := &nibiruApp.PricefeedKeeper
			pairs := common.AssetPairs{
				common.PairCollStable,
				common.PairGovStable,
			}
			pfParams := pricefeedTypes.Params{Pairs: pairs}
			priceKeeper.SetParams(ctx, pfParams)
			priceKeeper.WhitelistOracles(ctx, []sdk.AccAddress{oracle})

			collRatio := sdk.MustNewDecFromStr("0.9")
			feeRatio := sdk.ZeroDec()
			feeRatioEF := sdk.MustNewDecFromStr("0.5")
			bonusRateRecoll := sdk.MustNewDecFromStr("0.002")
			adjustmentStep := sdk.MustNewDecFromStr("0.0025")
			priceLowerBound := sdk.MustNewDecFromStr("0.9999")
			priceUpperBound := sdk.MustNewDecFromStr("1.0001")

			nibiruApp.StablecoinKeeper.SetParams(
				ctx, types.NewParams(
					collRatio,
					feeRatio,
					feeRatioEF,
					bonusRateRecoll,
					"15 min",
					adjustmentStep,
					priceLowerBound,
					priceUpperBound,
					true,
				),
			)

			t.Log("Post prices to each pair with the oracle.")
			priceExpiry := ctx.BlockTime().Add(time.Hour)
			_, err := priceKeeper.PostRawPrice(
				ctx, oracle, common.PairGovStable.String(), tc.govPrice, priceExpiry,
			)
			require.NoError(t, err)
			_, err = priceKeeper.PostRawPrice(
				ctx, oracle, common.PairCollStable.String(), tc.collPrice, priceExpiry,
			)
			require.NoError(t, err)

			// Update the 'CurrentPrice' posted by the oracles.
			for _, pair := range pfParams.Pairs {
				err = priceKeeper.GatherRawPrices(ctx, pair.Token0, pair.Token1)
				require.NoError(t, err, "Error posting price for pair: %d", pair.String())
			}

			// Fund account
			err = simapp.FundAccount(nibiruApp.BankKeeper, ctx, acc, tc.accFunds)
			require.NoError(t, err)

			// Mint NUSD -> Response contains Stable (sdk.Coin)
			goCtx := sdk.WrapSDKContext(ctx)
			mintStableResponse, err := nibiruApp.StablecoinKeeper.MintStable(
				goCtx, &tc.msgMint)

			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}

			require.NoError(t, err)
			assert.EqualValues(t, tc.msgResponse, *mintStableResponse)

			balances := nibiruApp.BankKeeper.GetAllBalances(ctx, nibiruApp.AccountKeeper.GetModuleAddress(types.StableEFModuleAccount))
			assert.Equal(t, mintStableResponse.FeesPayed, balances)
		})
	}
}

// ------------------------------------------------------------------
// BurnStable / Redeem
// ------------------------------------------------------------------

func TestMsgBurn_ValidateBasic(t *testing.T) {
	testCases := []struct {
		name string
		msg  types.MsgBurnStable
		err  error
	}{
		{
			name: "invalid address",
			msg: types.MsgBurnStable{
				Creator: "invalid_address",
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "valid address",
			msg: types.MsgBurnStable{
				Creator: sample.AccAddress().String(),
			},
		},
	}
	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMsgBurnResponse_NotEnoughFunds(t *testing.T) {
	tests := []struct {
		name         string
		accFunds     sdk.Coins
		moduleFunds  sdk.Coins
		msgBurn      types.MsgBurnStable
		msgResponse  *types.MsgBurnStableResponse
		govPrice     sdk.Dec
		collPrice    sdk.Dec
		expectedPass bool
		err          string
	}{
		{
			name:     "Not enough stable",
			accFunds: sdk.NewCoins(sdk.NewInt64Coin(common.DenomStable, 10)),
			msgBurn: types.MsgBurnStable{
				Creator: sample.AccAddress().String(),
				Stable:  sdk.NewInt64Coin(common.DenomStable, 9001),
			},
			msgResponse: &types.MsgBurnStableResponse{
				Collateral: sdk.NewCoin(common.DenomGov, sdk.ZeroInt()),
				Gov:        sdk.NewCoin(common.DenomColl, sdk.ZeroInt()),
			},
			govPrice:     sdk.MustNewDecFromStr("10"),
			collPrice:    sdk.MustNewDecFromStr("1"),
			expectedPass: false,
			err:          "insufficient funds",
		},
		{
			name:      "Stable is zero",
			govPrice:  sdk.MustNewDecFromStr("10"),
			collPrice: sdk.MustNewDecFromStr("1"),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(common.DenomStable, 1000000000),
			),
			moduleFunds: sdk.NewCoins(
				sdk.NewInt64Coin(common.DenomColl, 100000000),
			),
			msgBurn: types.MsgBurnStable{
				Creator: sample.AccAddress().String(),
				Stable:  sdk.NewCoin(common.DenomStable, sdk.ZeroInt()),
			},
			msgResponse: &types.MsgBurnStableResponse{
				Gov:        sdk.NewCoin(common.DenomGov, sdk.ZeroInt()),
				Collateral: sdk.NewCoin(common.DenomColl, sdk.ZeroInt()),
				FeesPayed:  sdk.NewCoins(),
			},
			expectedPass: true,
			err:          types.NoCoinFound.Wrap(common.DenomStable).Error(),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			nibiruApp, ctx := testapp.NewNibiruAppAndContext(true)
			acc, _ := sdk.AccAddressFromBech32(tc.msgBurn.Creator)
			oracle := sample.AccAddress()

			// Set stablecoin params
			collRatio := sdk.MustNewDecFromStr("0.9")
			feeRatio := sdk.MustNewDecFromStr("0.002")
			feeRatioEF := sdk.MustNewDecFromStr("0.5")
			bonusRateRecoll := sdk.MustNewDecFromStr("0.002")
			adjustmentStep := sdk.MustNewDecFromStr("0.0025")
			priceLowerBound := sdk.MustNewDecFromStr("0.9999")
			priceUpperBound := sdk.MustNewDecFromStr("1.0001")

			nibiruApp.StablecoinKeeper.SetParams(
				ctx, types.NewParams(
					collRatio,
					feeRatio,
					feeRatioEF,
					bonusRateRecoll,
					"15 min",
					adjustmentStep,
					priceLowerBound,
					priceUpperBound,
					true,
				),
			)

			// Set up pairs for the pricefeed keeper.
			priceKeeper := nibiruApp.PricefeedKeeper
			pairs := common.AssetPairs{
				{Token1: common.DenomStable, Token0: common.DenomGov},
				{Token1: common.DenomStable, Token0: common.DenomColl},
			}
			pfParams := pricefeedTypes.Params{Pairs: pairs}
			priceKeeper.SetParams(ctx, pfParams)
			priceKeeper.WhitelistOracles(ctx, []sdk.AccAddress{oracle})

			defaultParams := types.DefaultParams()
			defaultParams.IsCollateralRatioValid = true
			nibiruApp.StablecoinKeeper.SetParams(ctx, defaultParams)

			t.Log("Post prices to each pair with the oracle.")
			priceExpiry := ctx.BlockTime().Add(time.Hour)
			_, err := priceKeeper.PostRawPrice(
				ctx, oracle, common.PairGovStable.String(), tc.govPrice, priceExpiry,
			)
			require.NoError(t, err)
			_, err = priceKeeper.PostRawPrice(
				ctx, oracle, common.PairCollStable.String(), tc.collPrice, priceExpiry,
			)
			require.NoError(t, err)

			// Update the 'CurrentPrice' posted by the oracles.
			for _, pair := range pfParams.Pairs {
				err = priceKeeper.GatherRawPrices(ctx, pair.Token0, pair.Token1)
				require.NoError(t, err, "Error posting price for pair: %d", pair.String())
			}

			// Add collaterals to the module
			err = nibiruApp.BankKeeper.MintCoins(ctx, types.ModuleName, tc.moduleFunds)
			if err != nil {
				panic(err)
			}

			err = simapp.FundAccount(nibiruApp.BankKeeper, ctx, acc, tc.accFunds)
			require.NoError(t, err)

			// Burn NUSD -> Response contains GOV and COLL
			goCtx := sdk.WrapSDKContext(ctx)
			burnStableResponse, err := nibiruApp.StablecoinKeeper.BurnStable(
				goCtx, &tc.msgBurn)

			if !tc.expectedPass {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.err)

				return
			}
			require.NoError(t, err)
			assert.EqualValues(t, tc.msgResponse, burnStableResponse)
		})
	}
}

func TestMsgBurnResponse_HappyPath(t *testing.T) {
	tests := []struct {
		name                   string
		accFunds               sdk.Coins
		moduleFunds            sdk.Coins
		msgBurn                types.MsgBurnStable
		msgResponse            types.MsgBurnStableResponse
		govPrice               sdk.Dec
		collPrice              sdk.Dec
		supplyNIBI             sdk.Coin
		supplyNUSD             sdk.Coin
		ecosystemFund          sdk.Coins
		treasuryFund           sdk.Coins
		expectedPass           bool
		err                    error
		isCollateralRatioValid bool
	}{
		{
			name:      "invalid collateral ratio",
			govPrice:  sdk.MustNewDecFromStr("10"),
			collPrice: sdk.MustNewDecFromStr("1"),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(common.DenomStable, 1_000_000_000),
			),
			moduleFunds: sdk.NewCoins(
				sdk.NewInt64Coin(common.DenomColl, 100_000_000),
			),
			msgBurn: types.MsgBurnStable{
				Creator: sample.AccAddress().String(),
				Stable:  sdk.NewInt64Coin(common.DenomStable, 10_000_000),
			},
			ecosystemFund:          sdk.NewCoins(sdk.NewInt64Coin(common.DenomColl, 9000)),
			treasuryFund:           sdk.NewCoins(sdk.NewInt64Coin(common.DenomColl, 9000), sdk.NewInt64Coin(common.DenomGov, 100)),
			expectedPass:           false,
			isCollateralRatioValid: false,
			err:                    types.NoValidCollateralRatio,
		},
		{
			name:      "Happy path",
			govPrice:  sdk.MustNewDecFromStr("10"),
			collPrice: sdk.MustNewDecFromStr("1"),
			accFunds: sdk.NewCoins(
				sdk.NewInt64Coin(common.DenomStable, 1_000_000_000),
			),
			moduleFunds: sdk.NewCoins(
				sdk.NewInt64Coin(common.DenomColl, 100_000_000),
			),
			msgBurn: types.MsgBurnStable{
				Creator: sample.AccAddress().String(),
				Stable:  sdk.NewInt64Coin(common.DenomStable, 10_000_000),
			},
			msgResponse: types.MsgBurnStableResponse{
				Gov:        sdk.NewInt64Coin(common.DenomGov, 100_000-200),       // amount - fees 0,02%
				Collateral: sdk.NewInt64Coin(common.DenomColl, 9_000_000-18_000), // amount - fees 0,02%
				FeesPayed: sdk.NewCoins(
					sdk.NewInt64Coin(common.DenomGov, 200),
					sdk.NewInt64Coin(common.DenomColl, 18_000),
				),
			},
			supplyNIBI:             sdk.NewCoin(common.DenomGov, sdk.NewInt(100_000-100)), // nibiru minus 0.5 of fees burned (the part that goes to EF)
			supplyNUSD:             sdk.NewCoin(common.DenomStable, sdk.NewInt(1_000_000_000-10_000_000)),
			ecosystemFund:          sdk.NewCoins(sdk.NewInt64Coin(common.DenomColl, 9000)),
			treasuryFund:           sdk.NewCoins(sdk.NewInt64Coin(common.DenomColl, 9000), sdk.NewInt64Coin(common.DenomGov, 100)),
			expectedPass:           true,
			isCollateralRatioValid: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			nibiruApp, ctx := testapp.NewNibiruAppAndContext(true)
			acc, _ := sdk.AccAddressFromBech32(tc.msgBurn.Creator)
			oracle := sample.AccAddress()

			// Set stablecoin params
			collRatio := sdk.MustNewDecFromStr("0.9")
			feeRatio := sdk.MustNewDecFromStr("0.002")
			feeRatioEF := sdk.MustNewDecFromStr("0.5")
			bonusRateRecoll := sdk.MustNewDecFromStr("0.002")
			adjustmentStep := sdk.MustNewDecFromStr("0.0025")
			priceLowerBound := sdk.MustNewDecFromStr("0.9999")
			priceUpperBound := sdk.MustNewDecFromStr("1.0001")

			nibiruApp.StablecoinKeeper.SetParams(
				ctx, types.NewParams(
					collRatio,
					feeRatio,
					feeRatioEF,
					bonusRateRecoll,
					"15 min",
					adjustmentStep,
					priceLowerBound,
					priceUpperBound,
					tc.isCollateralRatioValid,
				),
			)

			// Set up pairs for the pricefeed keeper.
			priceKeeper := nibiruApp.PricefeedKeeper
			pairs := common.AssetPairs{
				{Token0: common.DenomGov, Token1: common.DenomStable},
				{Token0: common.DenomColl, Token1: common.DenomStable},
			}
			pfParams := pricefeedTypes.Params{Pairs: pairs}
			priceKeeper.SetParams(ctx, pfParams)
			priceKeeper.WhitelistOracles(ctx, []sdk.AccAddress{oracle})

			t.Log("Post prices to each pair with the oracle.")
			priceExpiry := ctx.BlockTime().Add(time.Hour)
			_, err := priceKeeper.PostRawPrice(
				ctx, oracle, common.PairGovStable.String(), tc.govPrice, priceExpiry,
			)
			require.NoError(t, err)
			_, err = priceKeeper.PostRawPrice(
				ctx, oracle, common.PairCollStable.String(), tc.collPrice, priceExpiry,
			)
			require.NoError(t, err)

			// Update the 'CurrentPrice' posted by the oracles.
			for _, pair := range pfParams.Pairs {
				err = priceKeeper.GatherRawPrices(ctx, pair.Token0, pair.Token1)
				require.NoError(t, err, "Error posting price for pair: %d", pair.String())
			}

			// Add collaterals to the module
			err = nibiruApp.BankKeeper.MintCoins(ctx, types.ModuleName, tc.moduleFunds)
			if err != nil {
				panic(err)
			}

			err = simapp.FundAccount(nibiruApp.BankKeeper, ctx, acc, tc.accFunds)
			require.NoError(t, err)

			// Burn NUSD -> Response contains GOV and COLL
			goCtx := sdk.WrapSDKContext(ctx)
			burnStableResponse, err := nibiruApp.StablecoinKeeper.BurnStable(
				goCtx, &tc.msgBurn)

			if !tc.expectedPass {
				require.ErrorIs(t, err, tc.err)
				return
			}

			require.NoError(t, err)
			assert.EqualValues(t, tc.msgResponse, *burnStableResponse)

			require.Equal(t, tc.supplyNIBI, nibiruApp.StablecoinKeeper.GetSupplyNIBI(ctx))
			require.Equal(t, tc.supplyNUSD, nibiruApp.StablecoinKeeper.GetSupplyNUSD(ctx))

			// Funds sypplies
			require.Equal(t,
				tc.ecosystemFund,
				nibiruApp.BankKeeper.GetAllBalances(
					ctx,
					nibiruApp.AccountKeeper.GetModuleAddress(types.StableEFModuleAccount)))
			require.Equal(t,
				tc.treasuryFund,
				nibiruApp.BankKeeper.GetAllBalances(
					ctx,
					nibiruApp.AccountKeeper.GetModuleAddress(common.TreasuryPoolModuleAccount)))
		})
	}
}
