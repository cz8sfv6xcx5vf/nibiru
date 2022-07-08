package keeper

import (
	"fmt"
	"math"
	"testing"
	"time"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/perp/types"
	testutilevents "github.com/NibiruChain/nibiru/x/testutil/events"
	"github.com/NibiruChain/nibiru/x/testutil/sample"
	vpooltypes "github.com/NibiruChain/nibiru/x/vpool/types"
)

func TestRequireMoreMarginRatio(t *testing.T) {
	type test struct {
		marginRatio, baseMarginRatio sdk.Dec
		largerThanEqualTo            bool
		wantErr                      bool
	}

	cases := map[string]test{
		"ok - largeThanOrEqualTo true": {
			marginRatio:       sdk.NewDec(2),
			baseMarginRatio:   sdk.NewDec(1),
			largerThanEqualTo: true,
			wantErr:           false,
		},
		"ok - largerThanOrEqualTo false": {
			marginRatio:       sdk.NewDec(1),
			baseMarginRatio:   sdk.NewDec(2),
			largerThanEqualTo: false,
			wantErr:           false,
		},
		"fails - largerThanEqualTo true": {
			marginRatio:       sdk.NewDec(1),
			baseMarginRatio:   sdk.NewDec(2),
			largerThanEqualTo: true,
			wantErr:           true,
		},
		"fails - largerThanEqualTo false": {
			marginRatio:       sdk.NewDec(2),
			baseMarginRatio:   sdk.NewDec(1),
			largerThanEqualTo: false,
			wantErr:           true,
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			err := requireMoreMarginRatio(tc.marginRatio, tc.baseMarginRatio, tc.largerThanEqualTo)
			switch {
			case tc.wantErr:
				if err == nil {
					t.Fatalf("expected error")
				}
			case !tc.wantErr:
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			}
		})
	}
}

func TestGetMarginRatio_Errors(t *testing.T) {
	tests := []struct {
		name string
		test func()
	}{
		{
			name: "empty size position",
			test: func() {
				k, _, ctx := getKeeper(t)

				pos := types.Position{
					Size_: sdk.ZeroDec(),
				}

				_, err := k.GetMarginRatio(
					ctx, pos, types.MarginCalculationPriceOption_MAX_PNL)
				assert.EqualError(t, err, types.ErrPositionZero.Error())
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.test()
		})
	}
}

func TestGetMarginRatio(t *testing.T) {
	tests := []struct {
		name                string
		position            types.Position
		newPrice            sdk.Dec
		expectedMarginRatio sdk.Dec
	}{
		{
			name: "margin without price changes",
			position: types.Position{
				TraderAddress:                       sample.AccAddress().String(),
				Pair:                                common.PairBTCStable,
				Size_:                               sdk.NewDec(10),
				OpenNotional:                        sdk.NewDec(10),
				Margin:                              sdk.NewDec(1),
				LastUpdateCumulativePremiumFraction: sdk.OneDec(),
			},
			newPrice:            sdk.MustNewDecFromStr("10"),
			expectedMarginRatio: sdk.MustNewDecFromStr("0.1"),
		},
		{
			name: "margin with price changes",
			position: types.Position{
				TraderAddress:                       sample.AccAddress().String(),
				Pair:                                common.PairBTCStable,
				Size_:                               sdk.NewDec(10),
				OpenNotional:                        sdk.NewDec(10),
				Margin:                              sdk.NewDec(1),
				LastUpdateCumulativePremiumFraction: sdk.OneDec(),
			},
			newPrice:            sdk.MustNewDecFromStr("12"),
			expectedMarginRatio: sdk.MustNewDecFromStr("0.25"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			perpKeeper, mocks, ctx := getKeeper(t)

			t.Log("Mock vpool spot price")
			mocks.mockVpoolKeeper.EXPECT().
				GetBaseAssetPrice(
					ctx,
					common.PairBTCStable,
					vpooltypes.Direction_ADD_TO_POOL,
					tc.position.Size_.Abs(),
				).
				Return(tc.newPrice, nil)
			t.Log("Mock vpool twap")
			mocks.mockVpoolKeeper.EXPECT().
				GetBaseAssetTWAP(
					ctx,
					common.PairBTCStable,
					vpooltypes.Direction_ADD_TO_POOL,
					tc.position.Size_.Abs(),
					15*time.Minute,
				).
				Return(tc.newPrice, nil)

			perpKeeper.PairMetadataState(ctx).Set(&types.PairMetadata{
				Pair:                       common.PairBTCStable,
				CumulativePremiumFractions: []sdk.Dec{sdk.OneDec()},
			})

			marginRatio, err := perpKeeper.GetMarginRatio(
				ctx, tc.position, types.MarginCalculationPriceOption_MAX_PNL)

			require.NoError(t, err)
			require.Equal(t, tc.expectedMarginRatio, marginRatio)
		})
	}
}

func TestRemoveMargin(t *testing.T) {
	tests := []struct {
		name string
		test func()
	}{
		{
			name: "fail - invalid sender",
			test: func() {
				k, _, ctx := getKeeper(t)
				goCtx := sdk.WrapSDKContext(ctx)

				msg := &types.MsgRemoveMargin{Sender: ""}
				_, err := k.RemoveMargin(goCtx, msg)
				require.Error(t, err)
			},
		},
		{
			name: "fail - invalid token pair",
			test: func() {
				k, _, ctx := getKeeper(t)
				goCtx := sdk.WrapSDKContext(ctx)

				trader := sample.AccAddress()
				the3pool := "dai:usdc:usdt"
				msg := &types.MsgRemoveMargin{
					Sender:    trader.String(),
					TokenPair: the3pool,
					Margin:    sdk.NewCoin(common.DenomStable, sdk.NewInt(5))}
				_, err := k.RemoveMargin(goCtx, msg)
				require.Error(t, err)
				require.ErrorContains(t, err, common.ErrInvalidTokenPair.Error())
			},
		},
		{
			name: "fail - request is too large",
			test: func() {
				perpKeeper, mocks, ctx := getKeeper(t)
				goCtx := sdk.WrapSDKContext(ctx)

				t.Log("Build msg that specifies an impossible margin removal (too high)")
				trader := sample.AccAddress()
				pair := common.AssetPair{
					Token0: "osmo",
					Token1: "nusd",
				}
				msg := &types.MsgRemoveMargin{
					Sender:    trader.String(),
					TokenPair: pair.String(),
					Margin:    sdk.NewCoin(pair.GetQuoteTokenDenom(), sdk.NewInt(600)),
				}

				mocks.mockVpoolKeeper.EXPECT().ExistsPool(ctx, pair).Return(true)

				t.Log("Set vpool defined by pair on PerpKeeper")
				perpKeeper.PairMetadataState(ctx).Set(&types.PairMetadata{
					Pair: pair,
					CumulativePremiumFractions: []sdk.Dec{
						sdk.ZeroDec(),
						sdk.MustNewDecFromStr("0.1")},
				})

				t.Log("Set an underwater position, positive bad debt due to excessive margin request")
				perpKeeper.PositionsState(ctx).Set(pair, trader, &types.Position{
					TraderAddress:                       trader.String(),
					Pair:                                pair,
					Size_:                               sdk.NewDec(1_000),
					OpenNotional:                        sdk.NewDec(1000),
					Margin:                              sdk.NewDec(500),
					LastUpdateCumulativePremiumFraction: sdk.MustNewDecFromStr("0.1"),
					BlockNumber:                         ctx.BlockHeight(),
				})

				_, err := perpKeeper.RemoveMargin(goCtx, msg)

				require.Error(t, err)
				require.ErrorContains(t, err, types.ErrFailedRemoveMarginCanCauseBadDebt.Error())
			},
		},
		{
			name: "fail - vault doesn't have enough funds",
			test: func() {
				perpKeeper, mocks, ctx := getKeeper(t)
				goCtx := sdk.WrapSDKContext(ctx)

				traderAddr := sample.AccAddress()
				msg := &types.MsgRemoveMargin{
					Sender:    traderAddr.String(),
					TokenPair: "osmo:nusd",
					Margin:    sdk.NewCoin("nusd", sdk.NewInt(100)),
				}

				pair := common.MustNewAssetPair(msg.TokenPair)

				t.Log("mock vpool keeper")
				mocks.mockVpoolKeeper.EXPECT().ExistsPool(ctx, pair).AnyTimes().Return(true)
				mocks.mockVpoolKeeper.EXPECT().GetSpotPrice(ctx, pair).Return(sdk.OneDec(), nil)
				mocks.mockVpoolKeeper.EXPECT().GetBaseAssetPrice(
					ctx,
					pair,
					vpooltypes.Direction_ADD_TO_POOL,
					sdk.NewDec(1_000),
				).Return(sdk.NewDec(1000), nil).Times(2)
				mocks.mockVpoolKeeper.EXPECT().GetBaseAssetTWAP(
					ctx,
					pair,
					vpooltypes.Direction_ADD_TO_POOL,
					sdk.NewDec(1_000),
					15*time.Minute,
				).Return(sdk.NewDec(1000), nil)

				t.Log("mock account keeper")
				mocks.mockAccountKeeper.
					EXPECT().GetModuleAddress(types.VaultModuleAccount).
					Return(authtypes.NewModuleAddress(types.VaultModuleAccount))

				t.Log("mock bank keeper")
				expectedError := fmt.Errorf("not enough funds in vault module account")
				mocks.mockBankKeeper.EXPECT().SendCoinsFromModuleToModule(
					ctx, types.PerpEFModuleAccount, types.VaultModuleAccount, sdk.NewCoins(msg.Margin),
				).Return(expectedError)
				mocks.mockBankKeeper.EXPECT().GetBalance(ctx, authtypes.NewModuleAddress(types.VaultModuleAccount), pair.GetQuoteTokenDenom()).Return(sdk.NewCoin(pair.GetQuoteTokenDenom(), sdk.ZeroInt()))

				t.Log("set pair metadata")
				perpKeeper.PairMetadataState(ctx).Set(&types.PairMetadata{
					Pair: pair,
					CumulativePremiumFractions: []sdk.Dec{
						sdk.ZeroDec(),
					},
				})

				t.Log("Set position a healthy position that has 0 unrealized funding")
				perpKeeper.PositionsState(ctx).Set(pair, traderAddr, &types.Position{
					TraderAddress:                       traderAddr.String(),
					Pair:                                pair,
					Size_:                               sdk.NewDec(1_000),
					OpenNotional:                        sdk.NewDec(1_000),
					Margin:                              sdk.NewDec(500),
					LastUpdateCumulativePremiumFraction: sdk.ZeroDec(),
					BlockNumber:                         ctx.BlockHeight(),
				})

				t.Log("Attempt to RemoveMargin when the vault lacks funds")
				_, err := perpKeeper.RemoveMargin(goCtx, msg)

				require.Error(t, err)
				require.ErrorContains(t, err, expectedError.Error())
			},
		},
		{
			name: "happy path - zero funding",
			test: func() {
				perpKeeper, mocks, ctx := getKeeper(t)
				goCtx := sdk.WrapSDKContext(ctx)

				traderAddr := sample.AccAddress()
				msg := &types.MsgRemoveMargin{
					Sender:    traderAddr.String(),
					TokenPair: "osmo:nusd",
					Margin:    sdk.NewCoin("nusd", sdk.NewInt(100)),
				}

				pair := common.MustNewAssetPair(msg.TokenPair)

				t.Log("mock vpool keeper")
				mocks.mockVpoolKeeper.EXPECT().ExistsPool(ctx, pair).Return(true).Times(2)
				mocks.mockVpoolKeeper.EXPECT().GetSpotPrice(ctx, pair).Return(sdk.OneDec(), nil)
				mocks.mockVpoolKeeper.EXPECT().GetBaseAssetPrice(
					ctx, pair, vpooltypes.Direction_ADD_TO_POOL, sdk.NewDec(1_000)).
					Return(sdk.NewDec(1000), nil).Times(2)
				mocks.mockVpoolKeeper.EXPECT().GetBaseAssetTWAP(
					ctx, pair, vpooltypes.Direction_ADD_TO_POOL, sdk.NewDec(1_000),
					15*time.Minute,
				).Return(sdk.NewDec(1000), nil)

				t.Log("mock account keeper")
				mocks.mockAccountKeeper.
					EXPECT().GetModuleAddress(types.VaultModuleAccount).
					Return(authtypes.NewModuleAddress(types.VaultModuleAccount))

				t.Log("mock bank keeper")
				mocks.mockBankKeeper.
					EXPECT().GetBalance(ctx, authtypes.NewModuleAddress(types.VaultModuleAccount), pair.GetQuoteTokenDenom()).
					Return(sdk.NewCoin(pair.GetQuoteTokenDenom(), sdk.NewInt(math.MaxInt64)))
				mocks.mockBankKeeper.EXPECT().SendCoinsFromModuleToAccount(
					ctx, types.VaultModuleAccount, traderAddr, sdk.NewCoins(msg.Margin),
				).Return(nil)

				t.Log("set pair metadata")
				perpKeeper.PairMetadataState(ctx).Set(&types.PairMetadata{
					Pair: pair,
					CumulativePremiumFractions: []sdk.Dec{
						sdk.ZeroDec(),
					},
				})

				t.Log("Set position a healthy position that has 0 unrealized funding")
				perpKeeper.PositionsState(ctx).Set(pair, traderAddr, &types.Position{
					TraderAddress:                       traderAddr.String(),
					Pair:                                pair,
					Size_:                               sdk.NewDec(1_000),
					OpenNotional:                        sdk.NewDec(1_000),
					Margin:                              sdk.NewDec(500),
					LastUpdateCumulativePremiumFraction: sdk.ZeroDec(),
					BlockNumber:                         ctx.BlockHeight(),
				})

				t.Log("'RemoveMargin' from the position")
				res, err := perpKeeper.RemoveMargin(goCtx, msg)

				require.NoError(t, err)
				assert.EqualValues(t, msg.Margin, res.MarginOut)
				assert.EqualValues(t, sdk.ZeroDec(), res.FundingPayment)

				t.Log("Verify correct events emitted for 'RemoveMargin'")
				testutilevents.RequireHasTypedEvent(t, ctx,
					&types.PositionChangedEvent{
						Pair:                  msg.TokenPair,
						TraderAddress:         traderAddr.String(),
						Margin:                sdk.NewInt64Coin(pair.GetQuoteTokenDenom(), 400),
						PositionNotional:      sdk.NewDec(1000),
						ExchangedPositionSize: sdk.ZeroDec(),                                         // always zero when removing margin
						TransactionFee:        sdk.NewCoin(pair.GetQuoteTokenDenom(), sdk.ZeroInt()), // always zero when removing margin
						PositionSize:          sdk.NewDec(1000),
						RealizedPnl:           sdk.ZeroDec(), // always zero when removing margin
						UnrealizedPnlAfter:    sdk.ZeroDec(),
						BadDebt:               sdk.NewCoin(pair.GetQuoteTokenDenom(), sdk.ZeroInt()), // always zero when adding margin
						FundingPayment:        sdk.ZeroDec(),
						SpotPrice:             sdk.OneDec(),
						BlockHeight:           ctx.BlockHeight(),
						BlockTimeMs:           ctx.BlockTime().UnixMilli(),
						LiquidationPenalty:    sdk.ZeroDec(),
					},
				)

				pos, err := perpKeeper.PositionsState(ctx).Get(pair, traderAddr)
				require.NoError(t, err)
				assert.EqualValues(t, sdk.NewDec(400).String(), pos.Margin.String())
				assert.EqualValues(t, sdk.NewDec(1000).String(), pos.Size_.String())
				assert.EqualValues(t, traderAddr.String(), pos.TraderAddress)
			},
		},
		{
			name: "happy path - massive funding payment",
			test: func() {
				perpKeeper, mocks, ctx := getKeeper(t)
				goCtx := sdk.WrapSDKContext(ctx)

				traderAddr := sample.AccAddress()
				msg := &types.MsgRemoveMargin{
					Sender:    traderAddr.String(),
					TokenPair: "osmo:nusd",
					Margin:    sdk.NewCoin("nusd", sdk.NewInt(100)),
				}

				pair := common.MustNewAssetPair(msg.TokenPair)

				t.Log("mock vpool keeper")
				mocks.mockVpoolKeeper.EXPECT().ExistsPool(ctx, pair).Return(true)

				t.Log("set pair metadata")
				perpKeeper.PairMetadataState(ctx).Set(&types.PairMetadata{
					Pair: pair,
					CumulativePremiumFractions: []sdk.Dec{
						sdk.OneDec(),
					},
				})

				t.Log("Set position a healthy position that has 0 unrealized funding")
				perpKeeper.PositionsState(ctx).Set(pair, traderAddr, &types.Position{
					TraderAddress:                       traderAddr.String(),
					Pair:                                pair,
					Size_:                               sdk.NewDec(500),
					OpenNotional:                        sdk.NewDec(500),
					Margin:                              sdk.NewDec(500),
					LastUpdateCumulativePremiumFraction: sdk.ZeroDec(),
					BlockNumber:                         ctx.BlockHeight(),
				})

				t.Log("'RemoveMargin' from the position")
				res, err := perpKeeper.RemoveMargin(goCtx, msg)

				require.ErrorIs(t, err, types.ErrFailedRemoveMarginCanCauseBadDebt)
				require.Nil(t, res)

				pos, err := perpKeeper.PositionsState(ctx).Get(pair, traderAddr)
				require.NoError(t, err)
				assert.EqualValues(t, sdk.NewDec(500).String(), pos.Margin.String())
				assert.EqualValues(t, sdk.NewDec(500).String(), pos.Size_.String())
				assert.EqualValues(t, traderAddr.String(), pos.TraderAddress)
			},
		},
	}

	for _, testCase := range tests {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			tc.test()
		})
	}
}

func TestAddMargin(t *testing.T) {
	tests := []struct {
		name string
		test func()
	}{
		{
			name: "fail - invalid sender",
			test: func() {
				k, _, ctx := getKeeper(t)
				goCtx := sdk.WrapSDKContext(ctx)

				msg := &types.MsgAddMargin{Sender: ""}
				_, err := k.AddMargin(goCtx, msg)
				require.Error(t, err)
			},
		},
		{
			name: "fail - invalid token pair",
			test: func() {
				k, _, ctx := getKeeper(t)
				goCtx := sdk.WrapSDKContext(ctx)

				trader := sample.AccAddress()
				the3pool := "dai:usdc:usdt"
				msg := &types.MsgAddMargin{
					Sender:    trader.String(),
					TokenPair: the3pool,
					Margin:    sdk.NewInt64Coin(common.DenomStable, 5),
				}
				_, err := k.AddMargin(goCtx, msg)
				require.ErrorContains(t, err, common.ErrInvalidTokenPair.Error())
			},
		},
		{
			name: "fail - user doesn't have enough funds",
			test: func() {
				perpKeeper, mocks, ctx := getKeeper(t)
				goCtx := sdk.WrapSDKContext(ctx)

				traderAddr := sample.AccAddress()
				assetPair := common.AssetPair{
					Token0: "uosmo",
					Token1: "unusd",
				}

				t.Log("set pair metadata")
				perpKeeper.PairMetadataState(ctx).Set(&types.PairMetadata{
					Pair: assetPair,
					CumulativePremiumFractions: []sdk.Dec{
						sdk.ZeroDec(),
					},
				})
				mocks.mockVpoolKeeper.EXPECT().ExistsPool(ctx, assetPair).Return(true)

				t.Log("build msg")
				msg := &types.MsgAddMargin{
					Sender:    traderAddr.String(),
					TokenPair: assetPair.String(),
					Margin:    sdk.NewInt64Coin(assetPair.GetQuoteTokenDenom(), 600),
				}

				t.Log("set a position")
				perpKeeper.PositionsState(ctx).Set(assetPair, traderAddr, &types.Position{
					TraderAddress:                       traderAddr.String(),
					Pair:                                assetPair,
					Size_:                               sdk.NewDec(1_000),
					OpenNotional:                        sdk.NewDec(1_000),
					Margin:                              sdk.NewDec(500),
					LastUpdateCumulativePremiumFraction: sdk.ZeroDec(),
					BlockNumber:                         ctx.BlockHeight(),
				})

				t.Log("mock bankkeeper not enough funds")
				expectedError := fmt.Errorf("not enough funds in vault module account")
				mocks.mockBankKeeper.EXPECT().SendCoinsFromAccountToModule(
					ctx, traderAddr, types.VaultModuleAccount, sdk.NewCoins(msg.Margin),
				).Return(expectedError)

				_, err := perpKeeper.AddMargin(goCtx, msg)

				require.Error(t, err)
				require.ErrorContains(t, err, expectedError.Error())
			},
		},
		{
			name: "happy path - zero funding",
			test: func() {
				perpKeeper, mocks, ctx := getKeeper(t)
				goCtx := sdk.WrapSDKContext(ctx)

				pair := common.MustNewAssetPair("uosmo:unusd")

				traderAddr := sample.AccAddress()

				msg := &types.MsgAddMargin{
					Sender:    traderAddr.String(),
					TokenPair: pair.String(),
					Margin:    sdk.NewInt64Coin("unusd", 100),
				}

				mocks.mockVpoolKeeper.EXPECT().ExistsPool(ctx, pair).
					AnyTimes().Return(true)
				mocks.mockVpoolKeeper.EXPECT().GetBaseAssetPrice(ctx, pair, vpooltypes.Direction_ADD_TO_POOL, sdk.NewDec(1000)).Return(sdk.NewDec(1000), nil)
				mocks.mockVpoolKeeper.EXPECT().GetSpotPrice(ctx, pair).Return(sdk.OneDec(), nil)

				t.Log("set pair metadata")
				perpKeeper.PairMetadataState(ctx).Set(&types.PairMetadata{
					Pair: pair,
					CumulativePremiumFractions: []sdk.Dec{
						sdk.ZeroDec(),
					},
				})

				t.Log("set position")
				perpKeeper.PositionsState(ctx).Set(pair, traderAddr, &types.Position{
					TraderAddress:                       traderAddr.String(),
					Pair:                                pair,
					Size_:                               sdk.NewDec(1_000),
					OpenNotional:                        sdk.NewDec(1_000),
					Margin:                              sdk.NewDec(500),
					LastUpdateCumulativePremiumFraction: sdk.ZeroDec(),
					BlockNumber:                         1,
				})

				t.Log("mock bankKeeper")
				mocks.mockBankKeeper.EXPECT().SendCoinsFromAccountToModule(
					ctx, traderAddr, types.VaultModuleAccount, sdk.NewCoins(msg.Margin),
				).Return(nil)

				t.Log("execute AddMargin")
				resp, err := perpKeeper.AddMargin(goCtx, msg)
				require.NoError(t, err)

				t.Log("assert correct response")
				assert.EqualValues(t, sdk.ZeroDec(), resp.FundingPayment)
				assert.EqualValues(t, sdk.NewDec(600), resp.Position.Margin)
				assert.EqualValues(t, sdk.NewDec(1_000), resp.Position.OpenNotional)
				assert.EqualValues(t, sdk.NewDec(1_000), resp.Position.Size_)
				assert.EqualValues(t, traderAddr.String(), resp.Position.TraderAddress)
				assert.EqualValues(t, pair, resp.Position.Pair)
				assert.EqualValues(t, sdk.ZeroDec(), resp.Position.LastUpdateCumulativePremiumFraction)
				assert.EqualValues(t, ctx.BlockHeight(), resp.Position.BlockNumber)

				t.Log("Verify correct events emitted")
				testutilevents.RequireHasTypedEvent(t, ctx,
					&types.PositionChangedEvent{
						Pair:                  msg.TokenPair,
						TraderAddress:         traderAddr.String(),
						Margin:                sdk.NewInt64Coin(pair.GetQuoteTokenDenom(), 600),
						PositionNotional:      sdk.NewDec(1000),
						ExchangedPositionSize: sdk.ZeroDec(),                                         // always zero when adding margin
						TransactionFee:        sdk.NewCoin(pair.GetQuoteTokenDenom(), sdk.ZeroInt()), // always zero when adding margin
						PositionSize:          sdk.NewDec(1000),
						RealizedPnl:           sdk.ZeroDec(), // always zero when adding margin
						UnrealizedPnlAfter:    sdk.ZeroDec(),
						BadDebt:               sdk.NewCoin(pair.GetQuoteTokenDenom(), sdk.ZeroInt()), // always zero when adding margin
						FundingPayment:        sdk.ZeroDec(),
						SpotPrice:             sdk.OneDec(),
						BlockHeight:           ctx.BlockHeight(),
						BlockTimeMs:           ctx.BlockTime().UnixMilli(),
						LiquidationPenalty:    sdk.ZeroDec(),
					},
				)
			},
		},
		{
			name: "happy path - with funding payment",
			test: func() {
				perpKeeper, mocks, ctx := getKeeper(t)

				pair := common.MustNewAssetPair("uosmo:unusd")

				traderAddr := sample.AccAddress()

				msg := &types.MsgAddMargin{
					Sender:    traderAddr.String(),
					TokenPair: pair.String(),
					Margin:    sdk.NewInt64Coin("unusd", 100),
				}

				mocks.mockVpoolKeeper.EXPECT().ExistsPool(ctx, pair).AnyTimes().Return(true)
				mocks.mockVpoolKeeper.EXPECT().GetBaseAssetPrice(ctx, pair, vpooltypes.Direction_ADD_TO_POOL, sdk.NewDec(1000)).Return(sdk.NewDec(1000), nil)
				mocks.mockVpoolKeeper.EXPECT().GetSpotPrice(ctx, pair).Return(sdk.OneDec(), nil)

				t.Log("set pair metadata")
				perpKeeper.PairMetadataState(ctx).Set(&types.PairMetadata{
					Pair: pair,
					CumulativePremiumFractions: []sdk.Dec{
						sdk.MustNewDecFromStr("0.001"),
					},
				})

				t.Log("set position")
				perpKeeper.PositionsState(ctx).Set(pair, traderAddr, &types.Position{
					TraderAddress:                       traderAddr.String(),
					Pair:                                pair,
					Size_:                               sdk.NewDec(1_000),
					OpenNotional:                        sdk.NewDec(1_000),
					Margin:                              sdk.NewDec(500),
					LastUpdateCumulativePremiumFraction: sdk.ZeroDec(),
					BlockNumber:                         1,
				})

				mocks.mockBankKeeper.EXPECT().SendCoinsFromAccountToModule(
					ctx, traderAddr, types.VaultModuleAccount, sdk.NewCoins(msg.Margin),
				).Return(nil)

				t.Log("execute AddMargin")
				resp, err := perpKeeper.AddMargin(sdk.WrapSDKContext(ctx), msg)
				require.NoError(t, err)

				t.Log("assert correct response")
				assert.EqualValues(t, sdk.NewDec(1), resp.FundingPayment)
				assert.EqualValues(t, sdk.NewDec(599), resp.Position.Margin)
				assert.EqualValues(t, sdk.NewDec(1_000), resp.Position.OpenNotional)
				assert.EqualValues(t, sdk.NewDec(1_000), resp.Position.Size_)
				assert.EqualValues(t, traderAddr.String(), resp.Position.TraderAddress)
				assert.EqualValues(t, pair, resp.Position.Pair)
				assert.EqualValues(t, sdk.MustNewDecFromStr("0.001"), resp.Position.LastUpdateCumulativePremiumFraction)
				assert.EqualValues(t, ctx.BlockHeight(), resp.Position.BlockNumber)

				t.Log("assert correct final position in state")
				pos, err := perpKeeper.PositionsState(ctx).Get(pair, traderAddr)
				require.NoError(t, err)
				assert.EqualValues(t, sdk.NewDec(599).String(), pos.Margin.String())
				assert.EqualValues(t, sdk.NewDec(1000).String(), pos.Size_.String())
				assert.EqualValues(t, traderAddr.String(), pos.TraderAddress)

				t.Log("Verify correct events emitted")
				testutilevents.RequireHasTypedEvent(t, ctx,
					&types.PositionChangedEvent{
						Pair:                  msg.TokenPair,
						TraderAddress:         traderAddr.String(),
						Margin:                sdk.NewInt64Coin(pair.GetQuoteTokenDenom(), 599),
						PositionNotional:      sdk.NewDec(1000),
						ExchangedPositionSize: sdk.ZeroDec(),                                         // always zero when adding margin
						TransactionFee:        sdk.NewCoin(pair.GetQuoteTokenDenom(), sdk.ZeroInt()), // always zero when adding margin
						PositionSize:          sdk.NewDec(1000),
						RealizedPnl:           sdk.ZeroDec(), // always zero when adding margin
						UnrealizedPnlAfter:    sdk.ZeroDec(),
						BadDebt:               sdk.NewCoin(pair.GetQuoteTokenDenom(), sdk.ZeroInt()), // always zero when adding margin
						FundingPayment:        sdk.OneDec(),
						SpotPrice:             sdk.OneDec(),
						BlockHeight:           ctx.BlockHeight(),
						BlockTimeMs:           ctx.BlockTime().UnixMilli(),
						LiquidationPenalty:    sdk.ZeroDec(),
					},
				)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.test()
		})
	}
}

func TestGetPositionNotionalAndUnrealizedPnl(t *testing.T) {
	tests := []struct {
		name                       string
		initialPosition            types.Position
		setMocks                   func(ctx sdk.Context, mocks mockedDependencies)
		pnlCalcOption              types.PnLCalcOption
		expectedPositionalNotional sdk.Dec
		expectedUnrealizedPnL      sdk.Dec
	}{
		{
			name: "long position; positive pnl; spot price calc",
			initialPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetPrice(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_ADD_TO_POOL,
						sdk.NewDec(10),
					).
					Return(sdk.NewDec(20), nil)
			},
			pnlCalcOption:              types.PnLCalcOption_SPOT_PRICE,
			expectedPositionalNotional: sdk.NewDec(20),
			expectedUnrealizedPnL:      sdk.NewDec(10),
		},
		{
			name: "long position; negative pnl; spot price calc",
			initialPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetPrice(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_ADD_TO_POOL,
						sdk.NewDec(10),
					).
					Return(sdk.NewDec(5), nil)
			},
			pnlCalcOption:              types.PnLCalcOption_SPOT_PRICE,
			expectedPositionalNotional: sdk.NewDec(5),
			expectedUnrealizedPnL:      sdk.NewDec(-5),
		},
		{
			name: "long position; positive pnl; twap calc",
			initialPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetTWAP(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_ADD_TO_POOL,
						sdk.NewDec(10),
						15*time.Minute,
					).
					Return(sdk.NewDec(20), nil)
			},
			pnlCalcOption:              types.PnLCalcOption_TWAP,
			expectedPositionalNotional: sdk.NewDec(20),
			expectedUnrealizedPnL:      sdk.NewDec(10),
		},
		{
			name: "long position; negative pnl; twap calc",
			initialPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetTWAP(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_ADD_TO_POOL,
						sdk.NewDec(10),
						15*time.Minute,
					).
					Return(sdk.NewDec(5), nil)
			},
			pnlCalcOption:              types.PnLCalcOption_TWAP,
			expectedPositionalNotional: sdk.NewDec(5),
			expectedUnrealizedPnL:      sdk.NewDec(-5),
		},
		{
			name: "long position; positive pnl; oracle calc",
			initialPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				mocks.mockVpoolKeeper.EXPECT().
					GetUnderlyingPrice(
						ctx,
						common.PairBTCStable,
					).
					Return(sdk.NewDec(2), nil)
			},
			pnlCalcOption:              types.PnLCalcOption_ORACLE,
			expectedPositionalNotional: sdk.NewDec(20),
			expectedUnrealizedPnL:      sdk.NewDec(10),
		},
		{
			name: "long position; negative pnl; oracle calc",
			initialPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				mocks.mockVpoolKeeper.EXPECT().
					GetUnderlyingPrice(
						ctx,
						common.PairBTCStable,
					).
					Return(sdk.MustNewDecFromStr("0.5"), nil)
			},
			pnlCalcOption:              types.PnLCalcOption_ORACLE,
			expectedPositionalNotional: sdk.NewDec(5),
			expectedUnrealizedPnL:      sdk.NewDec(-5),
		},
		{
			name: "short position; positive pnl; spot price calc",
			initialPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(-10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetPrice(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_REMOVE_FROM_POOL,
						sdk.NewDec(10),
					).
					Return(sdk.NewDec(5), nil)
			},
			pnlCalcOption:              types.PnLCalcOption_SPOT_PRICE,
			expectedPositionalNotional: sdk.NewDec(5),
			expectedUnrealizedPnL:      sdk.NewDec(5),
		},
		{
			name: "short position; negative pnl; spot price calc",
			initialPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(-10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetPrice(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_REMOVE_FROM_POOL,
						sdk.NewDec(10),
					).
					Return(sdk.NewDec(20), nil)
			},
			pnlCalcOption:              types.PnLCalcOption_SPOT_PRICE,
			expectedPositionalNotional: sdk.NewDec(20),
			expectedUnrealizedPnL:      sdk.NewDec(-10),
		},
		{
			name: "short position; positive pnl; twap calc",
			initialPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(-10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetTWAP(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_REMOVE_FROM_POOL,
						sdk.NewDec(10),
						15*time.Minute,
					).
					Return(sdk.NewDec(5), nil)
			},
			pnlCalcOption:              types.PnLCalcOption_TWAP,
			expectedPositionalNotional: sdk.NewDec(5),
			expectedUnrealizedPnL:      sdk.NewDec(5),
		},
		{
			name: "short position; negative pnl; twap calc",
			initialPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(-10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetTWAP(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_REMOVE_FROM_POOL,
						sdk.NewDec(10),
						15*time.Minute,
					).
					Return(sdk.NewDec(20), nil)
			},
			pnlCalcOption:              types.PnLCalcOption_TWAP,
			expectedPositionalNotional: sdk.NewDec(20),
			expectedUnrealizedPnL:      sdk.NewDec(-10),
		},
		{
			name: "short position; positive pnl; oracle calc",
			initialPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(-10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				mocks.mockVpoolKeeper.EXPECT().
					GetUnderlyingPrice(
						ctx,
						common.PairBTCStable,
					).
					Return(sdk.MustNewDecFromStr("0.5"), nil)
			},
			pnlCalcOption:              types.PnLCalcOption_ORACLE,
			expectedPositionalNotional: sdk.NewDec(5),
			expectedUnrealizedPnL:      sdk.NewDec(5),
		},
		{
			name: "long position; negative pnl; oracle calc",
			initialPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(-10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				mocks.mockVpoolKeeper.EXPECT().
					GetUnderlyingPrice(
						ctx,
						common.PairBTCStable,
					).
					Return(sdk.NewDec(2), nil)
			},
			pnlCalcOption:              types.PnLCalcOption_ORACLE,
			expectedPositionalNotional: sdk.NewDec(20),
			expectedUnrealizedPnL:      sdk.NewDec(-10),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			perpKeeper, mocks, ctx := getKeeper(t)

			tc.setMocks(ctx, mocks)

			positionalNotional, unrealizedPnl, err := perpKeeper.
				getPositionNotionalAndUnrealizedPnL(
					ctx,
					tc.initialPosition,
					tc.pnlCalcOption,
				)
			require.NoError(t, err)

			assert.EqualValues(t, tc.expectedPositionalNotional, positionalNotional)
			assert.EqualValues(t, tc.expectedUnrealizedPnL, unrealizedPnl)
		})
	}
}

func TestGetPreferencePositionNotionalAndUnrealizedPnl(t *testing.T) {
	// all tests are assumed long positions with positive pnl for ease of calculation
	// short positions and negative pnl are implicitly correct because of
	// TestGetPositionNotionalAndUnrealizedPnl
	testcases := []struct {
		name                       string
		initPosition               types.Position
		setMocks                   func(ctx sdk.Context, mocks mockedDependencies)
		pnlPreferenceOption        types.PnLPreferenceOption
		expectedPositionalNotional sdk.Dec
		expectedUnrealizedPnl      sdk.Dec
	}{
		{
			name: "max pnl, pick spot price",
			initPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				t.Log("Mock vpool spot price")
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetPrice(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_ADD_TO_POOL,
						sdk.NewDec(10),
					).
					Return(sdk.NewDec(20), nil)
				t.Log("Mock vpool twap")
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetTWAP(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_ADD_TO_POOL,
						sdk.NewDec(10),
						15*time.Minute,
					).
					Return(sdk.NewDec(15), nil)
			},
			pnlPreferenceOption:        types.PnLPreferenceOption_MAX,
			expectedPositionalNotional: sdk.NewDec(20),
			expectedUnrealizedPnl:      sdk.NewDec(10),
		},
		{
			name: "max pnl, pick twap",
			initPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				t.Log("Mock vpool spot price")
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetPrice(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_ADD_TO_POOL,
						sdk.NewDec(10),
					).
					Return(sdk.NewDec(20), nil)
				t.Log("Mock vpool twap")
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetTWAP(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_ADD_TO_POOL,
						sdk.NewDec(10),
						15*time.Minute,
					).
					Return(sdk.NewDec(30), nil)
			},
			pnlPreferenceOption:        types.PnLPreferenceOption_MAX,
			expectedPositionalNotional: sdk.NewDec(30),
			expectedUnrealizedPnl:      sdk.NewDec(20),
		},
		{
			name: "min pnl, pick spot price",
			initPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				t.Log("Mock vpool spot price")
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetPrice(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_ADD_TO_POOL,
						sdk.NewDec(10),
					).
					Return(sdk.NewDec(20), nil)
				t.Log("Mock vpool twap")
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetTWAP(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_ADD_TO_POOL,
						sdk.NewDec(10),
						15*time.Minute,
					).
					Return(sdk.NewDec(30), nil)
			},
			pnlPreferenceOption:        types.PnLPreferenceOption_MIN,
			expectedPositionalNotional: sdk.NewDec(20),
			expectedUnrealizedPnl:      sdk.NewDec(10),
		},
		{
			name: "min pnl, pick twap",
			initPosition: types.Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.PairBTCStable,
				Size_:         sdk.NewDec(10),
				OpenNotional:  sdk.NewDec(10),
				Margin:        sdk.NewDec(1),
			},
			setMocks: func(ctx sdk.Context, mocks mockedDependencies) {
				t.Log("Mock vpool spot price")
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetPrice(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_ADD_TO_POOL,
						sdk.NewDec(10),
					).
					Return(sdk.NewDec(20), nil)
				t.Log("Mock vpool twap")
				mocks.mockVpoolKeeper.EXPECT().
					GetBaseAssetTWAP(
						ctx,
						common.PairBTCStable,
						vpooltypes.Direction_ADD_TO_POOL,
						sdk.NewDec(10),
						15*time.Minute,
					).
					Return(sdk.NewDec(15), nil)
			},
			pnlPreferenceOption:        types.PnLPreferenceOption_MIN,
			expectedPositionalNotional: sdk.NewDec(15),
			expectedUnrealizedPnl:      sdk.NewDec(5),
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			perpKeeper, mocks, ctx := getKeeper(t)

			tc.setMocks(ctx, mocks)

			positionalNotional, unrealizedPnl, err := perpKeeper.
				getPreferencePositionNotionalAndUnrealizedPnL(
					ctx,
					tc.initPosition,
					tc.pnlPreferenceOption,
				)

			require.NoError(t, err)
			assert.EqualValues(t, tc.expectedPositionalNotional, positionalNotional)
			assert.EqualValues(t, tc.expectedUnrealizedPnl, unrealizedPnl)
		})
	}
}
