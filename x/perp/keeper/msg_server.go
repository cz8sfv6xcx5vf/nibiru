package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/perp/types"
)

type msgServer struct {
	k Keeper
}

var _ types.MsgServer = msgServer{}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{k: keeper}
}

func (m msgServer) RemoveMargin(ctx context.Context, msg *types.MsgRemoveMargin,
) (*types.MsgRemoveMarginResponse, error) {
	// These fields should have already been validated by MsgRemoveMargin.ValidateBasic() prior to being sent to the msgServer.
	traderAddr := sdk.MustAccAddressFromBech32(msg.Sender)
	pair := common.MustNewAssetPair(msg.TokenPair)

	marginOut, fundingPayment, position, err := m.k.RemoveMargin(sdk.UnwrapSDKContext(ctx), pair, traderAddr, msg.Margin)
	if err != nil {
		return nil, err
	}

	return &types.MsgRemoveMarginResponse{
		MarginOut:      marginOut,
		FundingPayment: fundingPayment,
		Position:       &position,
	}, nil
}

func (m msgServer) AddMargin(ctx context.Context, msg *types.MsgAddMargin,
) (*types.MsgAddMarginResponse, error) {
	// These fields should have already been validated by MsgAddMargin.ValidateBasic() prior to being sent to the msgServer.
	traderAddr := sdk.MustAccAddressFromBech32(msg.Sender)
	pair := common.MustNewAssetPair(msg.TokenPair)
	return m.k.AddMargin(sdk.UnwrapSDKContext(ctx), pair, traderAddr, msg.Margin)
}

func (m msgServer) OpenPosition(goCtx context.Context, req *types.MsgOpenPosition,
) (response *types.MsgOpenPositionResponse, err error) {
	pair := common.MustNewAssetPair(req.TokenPair)
	traderAddr := sdk.MustAccAddressFromBech32(req.Sender)

	positionResp, err := m.k.OpenPosition(
		sdk.UnwrapSDKContext(goCtx),
		pair,
		req.Side,
		traderAddr,
		req.QuoteAssetAmount,
		req.Leverage,
		req.BaseAssetAmountLimit.ToDec(),
	)
	if err != nil {
		return nil, err
	}

	return &types.MsgOpenPositionResponse{
		Position:               positionResp.Position,
		ExchangedNotionalValue: positionResp.ExchangedNotionalValue,
		ExchangedPositionSize:  positionResp.ExchangedPositionSize,
		FundingPayment:         positionResp.FundingPayment,
		RealizedPnl:            positionResp.RealizedPnl,
		UnrealizedPnlAfter:     positionResp.UnrealizedPnlAfter,
		MarginToVault:          positionResp.MarginToVault,
		PositionNotional:       positionResp.PositionNotional,
	}, nil
}

func (m msgServer) ClosePosition(goCtx context.Context, position *types.MsgClosePosition) (*types.MsgClosePositionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	traderAddr := sdk.MustAccAddressFromBech32(position.Sender)
	tokenPair := common.MustNewAssetPair(position.TokenPair)

	resp, err := m.k.ClosePosition(ctx, tokenPair, traderAddr)
	if err != nil {
		return nil, err
	}

	return &types.MsgClosePositionResponse{
		ExchangedNotionalValue: resp.ExchangedNotionalValue,
		ExchangedPositionSize:  resp.ExchangedPositionSize,
		FundingPayment:         resp.FundingPayment,
		RealizedPnl:            resp.RealizedPnl,
		MarginToTrader:         resp.MarginToVault.Neg(),
	}, nil
}

func (m msgServer) Liquidate(goCtx context.Context, msg *types.MsgLiquidate,
) (*types.MsgLiquidateResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	liquidatorAddr := sdk.MustAccAddressFromBech32(msg.Sender)

	traderAddr := sdk.MustAccAddressFromBech32(msg.Trader)

	pair := common.MustNewAssetPair(msg.TokenPair)

	feeToLiquidator, feeToFund, err := m.k.Liquidate(ctx, liquidatorAddr, pair, traderAddr)
	if err != nil {
		return nil, err
	}

	return &types.MsgLiquidateResponse{
		FeeToLiquidator:        feeToLiquidator,
		FeeToPerpEcosystemFund: feeToFund,
	}, nil
}

func (m msgServer) MultiLiquidate(goCtx context.Context, req *types.MsgMultiLiquidate) (*types.MsgMultiLiquidateResponse, error) {
	positions := make([]MultiLiquidationRequest, len(req.Liquidations))
	for i, pos := range req.Liquidations {
		positions[i] = MultiLiquidationRequest{
			pair:   common.MustNewAssetPair(pos.TokenPair),
			trader: sdk.MustAccAddressFromBech32(pos.Trader),
		}
	}

	resp := m.k.MultiLiquidate(sdk.UnwrapSDKContext(goCtx), sdk.MustAccAddressFromBech32(req.Sender), positions)

	liqResp := make([]*types.MsgMultiLiquidateResponse_MultiLiquidateResponse, len(resp))
	for i, r := range resp {
		liqResp[i] = r.IntoMultiLiquidateResponse()
	}

	return &types.MsgMultiLiquidateResponse{LiquidationResponses: liqResp}, nil
}

func (m msgServer) DonateToEcosystemFund(ctx context.Context, msg *types.MsgDonateToEcosystemFund) (*types.MsgDonateToEcosystemFundResponse, error) {
	if err := m.k.BankKeeper.SendCoinsFromAccountToModule(
		sdk.UnwrapSDKContext(ctx),
		sdk.MustAccAddressFromBech32(msg.Sender),
		types.PerpEFModuleAccount,
		sdk.NewCoins(msg.Donation),
	); err != nil {
		return nil, err
	}

	return &types.MsgDonateToEcosystemFundResponse{}, nil
}
