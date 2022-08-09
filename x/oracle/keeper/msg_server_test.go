package keeper

import (
	"testing"

	"github.com/NibiruChain/nibiru/x/common"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/NibiruChain/nibiru/x/oracle/types"

	"github.com/cosmos/cosmos-sdk/x/staking"
)

func TestMsgServer_FeederDelegation(t *testing.T) {
	input, msgServer := setup(t)

	exchangeRates := types.ExchangeRateTuples{
		{
			Pair:         common.PairBTCStable.String(),
			ExchangeRate: randomExchangeRate,
		},
	}

	exchangeRateStr, err := exchangeRates.ToString()
	require.NoError(t, err)
	salt := "1"
	hash := types.GetAggregateVoteHash(salt, exchangeRateStr, ValAddrs[0])

	// Case 1: empty message
	delegateFeedConsentMsg := types.MsgDelegateFeedConsent{}
	_, err = msgServer.DelegateFeedConsent(sdk.WrapSDKContext(input.Ctx), &delegateFeedConsentMsg)
	require.Error(t, err)

	// Case 2: Normal Prevote - without delegation
	prevoteMsg := types.NewMsgAggregateExchangeRatePrevote(hash, Addrs[0], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx), prevoteMsg)
	require.NoError(t, err)

	// Case 2.1: Normal Prevote - with delegation fails
	prevoteMsg = types.NewMsgAggregateExchangeRatePrevote(hash, Addrs[1], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx), prevoteMsg)
	require.Error(t, err)

	// Case 2.2: Normal Vote - without delegation
	voteMsg := types.NewMsgAggregateExchangeRateVote(salt, exchangeRateStr, Addrs[0], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(1)), voteMsg)
	require.NoError(t, err)

	// Case 2.3: Normal Vote - with delegation fails
	voteMsg = types.NewMsgAggregateExchangeRateVote(salt, exchangeRateStr, Addrs[1], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(1)), voteMsg)
	require.Error(t, err)

	// Case 3: Normal MsgDelegateFeedConsent succeeds
	msg := types.NewMsgDelegateFeedConsent(ValAddrs[0], Addrs[1])
	_, err = msgServer.DelegateFeedConsent(sdk.WrapSDKContext(input.Ctx), msg)
	require.NoError(t, err)

	// Case 4.1: Normal Prevote - without delegation fails
	prevoteMsg = types.NewMsgAggregateExchangeRatePrevote(hash, Addrs[2], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx), prevoteMsg)
	require.Error(t, err)

	// Case 4.2: Normal Prevote - with delegation succeeds
	prevoteMsg = types.NewMsgAggregateExchangeRatePrevote(hash, Addrs[1], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx), prevoteMsg)
	require.NoError(t, err)

	// Case 4.3: Normal Vote - without delegation fails
	voteMsg = types.NewMsgAggregateExchangeRateVote(salt, exchangeRateStr, Addrs[2], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(1)), voteMsg)
	require.Error(t, err)

	// Case 4.4: Normal Vote - with delegation succeeds
	voteMsg = types.NewMsgAggregateExchangeRateVote(salt, exchangeRateStr, Addrs[1], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(1)), voteMsg)
	require.NoError(t, err)
}

func TestMsgServer_AggregatePrevoteVote(t *testing.T) {
	input, msgServer := setup(t)

	salt := "1"
	exchangeRates := types.ExchangeRateTuples{
		{
			Pair:         common.PairGovStable.String(),
			ExchangeRate: sdk.MustNewDecFromStr("1000.23"),
		},
		{
			Pair:         common.PairETHStable.String(),
			ExchangeRate: sdk.MustNewDecFromStr("0.29"),
		},

		{
			Pair:         common.PairBTCStable.String(),
			ExchangeRate: sdk.MustNewDecFromStr("0.27"),
		},
	}

	otherExchangeRate := types.ExchangeRateTuples{
		{
			Pair:         common.PairGovStable.String(),
			ExchangeRate: sdk.MustNewDecFromStr("1000.23"),
		},
		{
			Pair:         common.PairETHStable.String(),
			ExchangeRate: sdk.MustNewDecFromStr("0.29"),
		},

		{
			Pair:         common.PairETHStable.String(),
			ExchangeRate: sdk.MustNewDecFromStr("0.27"),
		},
	}

	unintendedExchangeRateStr := types.ExchangeRateTuples{
		{
			Pair:         common.PairGovStable.String(),
			ExchangeRate: sdk.MustNewDecFromStr("1000.23"),
		},
		{
			Pair:         common.PairETHStable.String(),
			ExchangeRate: sdk.MustNewDecFromStr("0.29"),
		},

		{
			Pair: common.AssetPair{
				Token0: "BTC",
				Token1: "CNY",
			}.String(),
			ExchangeRate: sdk.MustNewDecFromStr("0.27"),
		},
	}
	exchangeRatesStr, err := exchangeRates.ToString()
	require.NoError(t, err)

	otherExchangeRateStr, err := otherExchangeRate.ToString()
	require.NoError(t, err)

	unintendedExchageRateStr, err := unintendedExchangeRateStr.ToString()
	require.NoError(t, err)

	hash := types.GetAggregateVoteHash(salt, exchangeRatesStr, ValAddrs[0])

	aggregateExchangeRatePrevoteMsg := types.NewMsgAggregateExchangeRatePrevote(hash, Addrs[0], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx), aggregateExchangeRatePrevoteMsg)
	require.NoError(t, err)

	// Unauthorized feeder
	aggregateExchangeRatePrevoteMsg = types.NewMsgAggregateExchangeRatePrevote(hash, Addrs[1], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx), aggregateExchangeRatePrevoteMsg)
	require.Error(t, err)

	// Invalid addr
	aggregateExchangeRatePrevoteMsg = types.NewMsgAggregateExchangeRatePrevote(hash, sdk.AccAddress{}, ValAddrs[0])
	_, err = msgServer.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx), aggregateExchangeRatePrevoteMsg)
	require.Error(t, err)

	// Invalid validator addr
	aggregateExchangeRatePrevoteMsg = types.NewMsgAggregateExchangeRatePrevote(hash, Addrs[0], sdk.ValAddress{})
	_, err = msgServer.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx), aggregateExchangeRatePrevoteMsg)
	require.Error(t, err)

	// Invalid reveal period
	aggregateExchangeRateVoteMsg := types.NewMsgAggregateExchangeRateVote(salt, exchangeRatesStr, Addrs[0], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx), aggregateExchangeRateVoteMsg)
	require.Error(t, err)

	// Invalid reveal period
	input.Ctx = input.Ctx.WithBlockHeight(2)
	aggregateExchangeRateVoteMsg = types.NewMsgAggregateExchangeRateVote(salt, exchangeRatesStr, Addrs[0], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx), aggregateExchangeRateVoteMsg)
	require.Error(t, err)

	// Other exchange rate with valid real period
	input.Ctx = input.Ctx.WithBlockHeight(1)
	aggregateExchangeRateVoteMsg = types.NewMsgAggregateExchangeRateVote(salt, otherExchangeRateStr, Addrs[0], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx), aggregateExchangeRateVoteMsg)
	require.Error(t, err)

	// Unauthorized feeder
	aggregateExchangeRateVoteMsg = types.NewMsgAggregateExchangeRateVote(salt, exchangeRatesStr, Addrs[1], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx), aggregateExchangeRateVoteMsg)
	require.Error(t, err)

	// Unintended denom vote
	aggregateExchangeRateVoteMsg = types.NewMsgAggregateExchangeRateVote(salt, unintendedExchageRateStr, Addrs[0], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx), aggregateExchangeRateVoteMsg)
	require.Error(t, err)

	// Valid exchange rate reveal submission
	input.Ctx = input.Ctx.WithBlockHeight(1)
	aggregateExchangeRateVoteMsg = types.NewMsgAggregateExchangeRateVote(salt, exchangeRatesStr, Addrs[0], ValAddrs[0])
	_, err = msgServer.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx), aggregateExchangeRateVoteMsg)
	require.NoError(t, err)
}

var (
	stakingAmt         = sdk.TokensFromConsensusPower(10, sdk.DefaultPowerReduction)
	randomExchangeRate = sdk.NewDec(1700)
)

func setup(t *testing.T) (TestInput, types.MsgServer) {
	input := CreateTestInput(t)
	params := input.OracleKeeper.GetParams(input.Ctx)
	params.VotePeriod = 1
	params.SlashWindow = 100
	params.RewardDistributionWindow = 100
	input.OracleKeeper.SetParams(input.Ctx, params)
	msgServer := NewMsgServerImpl(input.OracleKeeper)

	sh := staking.NewHandler(input.StakingKeeper)

	// Validator created
	_, err := sh(input.Ctx, NewTestMsgCreateValidator(ValAddrs[0], ValPubKeys[0], stakingAmt))
	require.NoError(t, err)
	_, err = sh(input.Ctx, NewTestMsgCreateValidator(ValAddrs[1], ValPubKeys[1], stakingAmt))
	require.NoError(t, err)
	_, err = sh(input.Ctx, NewTestMsgCreateValidator(ValAddrs[2], ValPubKeys[2], stakingAmt))
	require.NoError(t, err)
	staking.EndBlocker(input.Ctx, input.StakingKeeper)

	return input, msgServer
}
