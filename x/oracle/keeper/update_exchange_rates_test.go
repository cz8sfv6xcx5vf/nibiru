package keeper_test

import (
	"fmt"
	"math"
	"sort"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"

	"github.com/NibiruChain/collections"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/common/asset"
	"github.com/NibiruChain/nibiru/x/common/denoms"
	"github.com/NibiruChain/nibiru/x/oracle"
	"github.com/NibiruChain/nibiru/x/oracle/keeper"
	"github.com/NibiruChain/nibiru/x/oracle/types"
)

func TestOracleThreshold(t *testing.T) {
	exchangeRates := types.ExchangeRateTuples{
		{
			Pair:         asset.Registry.Pair(denoms.BTC, denoms.NUSD),
			ExchangeRate: randomExchangeRate,
		},
	}
	input, h := setup(t)
	exchangeRateStr, err := exchangeRates.ToString()
	require.NoError(t, err)

	// Case 1.
	// Less than the threshold signs, exchange rate consensus fails
	salt := "1"
	hash := types.GetAggregateVoteHash(salt, exchangeRateStr, keeper.ValAddrs[0])
	prevoteMsg := types.NewMsgAggregateExchangeRatePrevote(hash, keeper.Addrs[0], keeper.ValAddrs[0])
	voteMsg := types.NewMsgAggregateExchangeRateVote(salt, exchangeRateStr, keeper.Addrs[0], keeper.ValAddrs[0])

	_, err1 := h.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(0)), prevoteMsg)
	_, err2 := h.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(1)), voteMsg)
	require.NoError(t, err1)
	require.NoError(t, err2)

	oracle.EndBlocker(input.Ctx.WithBlockHeight(1), input.OracleKeeper)

	_, err = input.OracleKeeper.ExchangeRates.Get(input.Ctx.WithBlockHeight(1), exchangeRates[0].Pair)
	require.Error(t, err)

	// Case 2.
	// More than the threshold signs, exchange rate consensus succeeds
	salt = "1"
	hash = types.GetAggregateVoteHash(salt, exchangeRateStr, keeper.ValAddrs[0])
	prevoteMsg = types.NewMsgAggregateExchangeRatePrevote(hash, keeper.Addrs[0], keeper.ValAddrs[0])
	voteMsg = types.NewMsgAggregateExchangeRateVote(salt, exchangeRateStr, keeper.Addrs[0], keeper.ValAddrs[0])

	_, err1 = h.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(0)), prevoteMsg)
	_, err2 = h.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(1)), voteMsg)
	require.NoError(t, err1)
	require.NoError(t, err2)

	salt = "2"
	hash = types.GetAggregateVoteHash(salt, exchangeRateStr, keeper.ValAddrs[1])
	prevoteMsg = types.NewMsgAggregateExchangeRatePrevote(hash, keeper.Addrs[1], keeper.ValAddrs[1])
	voteMsg = types.NewMsgAggregateExchangeRateVote(salt, exchangeRateStr, keeper.Addrs[1], keeper.ValAddrs[1])

	_, err1 = h.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(0)), prevoteMsg)
	_, err2 = h.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(1)), voteMsg)
	require.NoError(t, err1)
	require.NoError(t, err2)

	salt = "3"
	hash = types.GetAggregateVoteHash(salt, exchangeRateStr, keeper.ValAddrs[2])
	prevoteMsg = types.NewMsgAggregateExchangeRatePrevote(hash, keeper.Addrs[2], keeper.ValAddrs[2])
	voteMsg = types.NewMsgAggregateExchangeRateVote(salt, exchangeRateStr, keeper.Addrs[2], keeper.ValAddrs[2])

	_, err1 = h.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(0)), prevoteMsg)
	_, err2 = h.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(1)), voteMsg)
	require.NoError(t, err1)
	require.NoError(t, err2)

	salt = "4"
	hash = types.GetAggregateVoteHash(salt, exchangeRateStr, keeper.ValAddrs[3])
	prevoteMsg = types.NewMsgAggregateExchangeRatePrevote(hash, keeper.Addrs[3], keeper.ValAddrs[3])
	voteMsg = types.NewMsgAggregateExchangeRateVote(salt, exchangeRateStr, keeper.Addrs[3], keeper.ValAddrs[3])

	_, err1 = h.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(0)), prevoteMsg)
	_, err2 = h.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(1)), voteMsg)
	require.NoError(t, err1)
	require.NoError(t, err2)

	oracle.EndBlocker(input.Ctx.WithBlockHeight(1), input.OracleKeeper)

	rate, err := input.OracleKeeper.ExchangeRates.Get(input.Ctx.WithBlockHeight(1), exchangeRates[0].Pair)
	require.NoError(t, err)
	require.Equal(t, randomExchangeRate, rate)

	// Case 3.
	// Increase voting power of absent validator, exchange rate consensus fails
	val, _ := input.StakingKeeper.GetValidator(input.Ctx, keeper.ValAddrs[4])
	_, _ = input.StakingKeeper.Delegate(input.Ctx.WithBlockHeight(0), keeper.Addrs[4], stakingAmt.MulRaw(8), stakingtypes.Unbonded, val, false)

	salt = "1"
	hash = types.GetAggregateVoteHash(salt, exchangeRateStr, keeper.ValAddrs[0])
	prevoteMsg = types.NewMsgAggregateExchangeRatePrevote(hash, keeper.Addrs[0], keeper.ValAddrs[0])
	voteMsg = types.NewMsgAggregateExchangeRateVote(salt, exchangeRateStr, keeper.Addrs[0], keeper.ValAddrs[0])

	_, err1 = h.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(0)), prevoteMsg)
	_, err2 = h.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(1)), voteMsg)
	require.NoError(t, err1)
	require.NoError(t, err2)

	salt = "2"
	hash = types.GetAggregateVoteHash(salt, exchangeRateStr, keeper.ValAddrs[1])
	prevoteMsg = types.NewMsgAggregateExchangeRatePrevote(hash, keeper.Addrs[1], keeper.ValAddrs[1])
	voteMsg = types.NewMsgAggregateExchangeRateVote(salt, exchangeRateStr, keeper.Addrs[1], keeper.ValAddrs[1])

	_, err1 = h.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(0)), prevoteMsg)
	_, err2 = h.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(1)), voteMsg)
	require.NoError(t, err1)
	require.NoError(t, err2)

	salt = "3"
	hash = types.GetAggregateVoteHash(salt, exchangeRateStr, keeper.ValAddrs[2])
	prevoteMsg = types.NewMsgAggregateExchangeRatePrevote(hash, keeper.Addrs[2], keeper.ValAddrs[2])
	voteMsg = types.NewMsgAggregateExchangeRateVote(salt, exchangeRateStr, keeper.Addrs[2], keeper.ValAddrs[2])

	_, err1 = h.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(0)), prevoteMsg)
	_, err2 = h.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(1)), voteMsg)
	require.NoError(t, err1)
	require.NoError(t, err2)

	salt = "4"
	hash = types.GetAggregateVoteHash(salt, exchangeRateStr, keeper.ValAddrs[3])
	prevoteMsg = types.NewMsgAggregateExchangeRatePrevote(hash, keeper.Addrs[3], keeper.ValAddrs[3])
	voteMsg = types.NewMsgAggregateExchangeRateVote(salt, exchangeRateStr, keeper.Addrs[3], keeper.ValAddrs[3])

	_, err1 = h.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(0)), prevoteMsg)
	_, err2 = h.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(1)), voteMsg)
	require.NoError(t, err1)
	require.NoError(t, err2)

	oracle.EndBlocker(input.Ctx.WithBlockHeight(1), input.OracleKeeper)

	_, err = input.OracleKeeper.ExchangeRates.Get(input.Ctx.WithBlockHeight(1), exchangeRates[0].Pair)
	require.Error(t, err)
}

func TestOracleDrop(t *testing.T) {
	input, h := setup(t)

	input.OracleKeeper.ExchangeRates.Insert(input.Ctx, asset.Registry.Pair(denoms.NIBI, denoms.NUSD), randomExchangeRate)

	// Account 1, pair gov stable
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 0)

	// Immediately swap halt after an illiquid oracle vote
	oracle.EndBlocker(input.Ctx, input.OracleKeeper)

	_, err := input.OracleKeeper.ExchangeRates.Get(input.Ctx, asset.Registry.Pair(denoms.NIBI, denoms.NUSD))
	require.Error(t, err)
}

func TestOracleTally(t *testing.T) {
	input, _ := setup(t)

	ballot := types.ExchangeRateBallots{}
	rates, valAddrs, stakingKeeper := types.GenerateRandomTestCase()
	input.OracleKeeper.StakingKeeper = stakingKeeper
	h := keeper.NewMsgServerImpl(input.OracleKeeper)
	for i, rate := range rates {
		decExchangeRate := sdk.NewDecWithPrec(int64(rate*math.Pow10(keeper.OracleDecPrecision)), int64(keeper.OracleDecPrecision))
		exchangeRateStr, err := types.ExchangeRateTuples{
			{ExchangeRate: decExchangeRate, Pair: asset.Registry.Pair(denoms.BTC, denoms.NUSD)}}.ToString()
		require.NoError(t, err)

		salt := fmt.Sprintf("%d", i)
		hash := types.GetAggregateVoteHash(salt, exchangeRateStr, valAddrs[i])
		prevoteMsg := types.NewMsgAggregateExchangeRatePrevote(hash, sdk.AccAddress(valAddrs[i]), valAddrs[i])
		voteMsg := types.NewMsgAggregateExchangeRateVote(salt, exchangeRateStr, sdk.AccAddress(valAddrs[i]), valAddrs[i])

		_, err1 := h.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(0)), prevoteMsg)
		_, err2 := h.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(1)), voteMsg)
		require.NoError(t, err1)
		require.NoError(t, err2)

		power := stakingAmt.QuoRaw(int64(6)).Int64()
		if decExchangeRate.IsZero() {
			power = int64(0)
		}

		vote := types.NewExchangeRateBallot(
			decExchangeRate, asset.Registry.Pair(denoms.BTC, denoms.NUSD), valAddrs[i], power)
		ballot = append(ballot, vote)

		// change power of every three validator
		if i%3 == 0 {
			stakingKeeper.Validators()[i].SetConsensusPower(int64(i + 1))
		}
	}

	validatorClaimMap := make(types.ValidatorPerformances)
	for _, valAddr := range valAddrs {
		validatorClaimMap[valAddr.String()] = types.ValidatorPerformance{
			Power:        stakingKeeper.Validator(input.Ctx, valAddr).GetConsensusPower(sdk.DefaultPowerReduction),
			RewardWeight: int64(0),
			WinCount:     int64(0),
			ValAddress:   valAddr,
		}
	}
	sort.Sort(ballot)
	weightedMedian := ballot.WeightedMedianWithAssertion()
	standardDeviation := ballot.StandardDeviation(weightedMedian)
	maxSpread := weightedMedian.Mul(input.OracleKeeper.RewardBand(input.Ctx).QuoInt64(2))

	if standardDeviation.GT(maxSpread) {
		maxSpread = standardDeviation
	}

	expectedValidatorClaimMap := make(types.ValidatorPerformances)
	for _, valAddr := range valAddrs {
		expectedValidatorClaimMap[valAddr.String()] = types.ValidatorPerformance{
			Power:        stakingKeeper.Validator(input.Ctx, valAddr).GetConsensusPower(sdk.DefaultPowerReduction),
			RewardWeight: int64(0),
			WinCount:     int64(0),
			ValAddress:   valAddr,
		}
	}

	for _, vote := range ballot {
		if (vote.ExchangeRate.GTE(weightedMedian.Sub(maxSpread)) &&
			vote.ExchangeRate.LTE(weightedMedian.Add(maxSpread))) ||
			!vote.ExchangeRate.IsPositive() {
			key := vote.Voter.String()
			claim := expectedValidatorClaimMap[key]
			claim.RewardWeight += vote.Power
			claim.WinCount++
			expectedValidatorClaimMap[key] = claim
		}
	}

	tallyMedian := keeper.Tally(ballot, input.OracleKeeper.RewardBand(input.Ctx), validatorClaimMap)

	require.Equal(t, validatorClaimMap, expectedValidatorClaimMap)
	require.Equal(t, tallyMedian.MulInt64(100).TruncateInt(), weightedMedian.MulInt64(100).TruncateInt())
}

func TestOracleTallyTiming(t *testing.T) {
	input, h := setup(t)

	// all the keeper.Addrs vote for the block ... not last period block yet, so tally fails
	for i := range keeper.Addrs[:4] {
		makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.BTC, denoms.NUSD), ExchangeRate: randomExchangeRate}}, i)
	}

	params := input.OracleKeeper.GetParams(input.Ctx)
	params.VotePeriod = 10 // set vote period to 10 for now, for convenience
	input.OracleKeeper.SetParams(input.Ctx, params)
	require.Equal(t, 0, int(input.Ctx.BlockHeight()))

	oracle.EndBlocker(input.Ctx, input.OracleKeeper)
	_, err := input.OracleKeeper.ExchangeRates.Get(input.Ctx, asset.Registry.Pair(denoms.BTC, denoms.NUSD))
	require.Error(t, err)

	input.Ctx = input.Ctx.WithBlockHeight(int64(params.VotePeriod - 1))

	oracle.EndBlocker(input.Ctx, input.OracleKeeper)
	_, err = input.OracleKeeper.ExchangeRates.Get(input.Ctx, asset.Registry.Pair(denoms.BTC, denoms.NUSD))
	require.NoError(t, err)
}

func TestOracleRewardDistribution(t *testing.T) {
	// the following test scenario simulates that two validators, out of three, are voting for one common pair.
	// they have the same voting power, and the reward allocation lasts for 1 voting period.
	input, h := setup(t)

	// Account 1, btc:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.BTC, denoms.NUSD), ExchangeRate: randomExchangeRate},
	}, 0)

	// Account 2, btc:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.BTC, denoms.NUSD), ExchangeRate: randomExchangeRate},
	}, 1)

	// Account 3, btc:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.BTC, denoms.NUSD), ExchangeRate: randomExchangeRate},
	}, 2)

	// Account 3, btc:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.BTC, denoms.NUSD), ExchangeRate: randomExchangeRate},
	}, 3)

	rewardAllocation := sdk.NewCoins(sdk.NewCoin("reward", sdk.NewInt(1*common.Precision)))
	keeper.AllocateRewards(t, input, asset.Registry.Pair(denoms.BTC, denoms.NUSD), rewardAllocation, 1)

	oracle.EndBlocker(input.Ctx.WithBlockHeight(1), input.OracleKeeper)

	expectedRewardOneVal := sdk.NewDecCoinsFromCoins(rewardAllocation...).QuoDec(sdk.NewDec(4))

	distributionRewards := input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[0])
	require.Equalf(t, expectedRewardOneVal, distributionRewards.Rewards, "%s<=>%s", expectedRewardOneVal.String(), distributionRewards.String())

	distributionRewards = input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[1])
	require.Equal(t, expectedRewardOneVal, distributionRewards.Rewards, "%s %s", expectedRewardOneVal.String(), distributionRewards.Rewards.AmountOf(denoms.NIBI).TruncateInt().String())

	distributionRewards = input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[2])
	require.Equal(t, expectedRewardOneVal, distributionRewards.Rewards, "%s %s", expectedRewardOneVal.String(), distributionRewards.Rewards.AmountOf(denoms.NIBI).TruncateInt().String())

	distributionRewards = input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[3])
	require.Equal(t, expectedRewardOneVal, distributionRewards.Rewards, "%s %s", expectedRewardOneVal.String(), distributionRewards.Rewards.AmountOf(denoms.NIBI).TruncateInt().String())
}

func TestOracleRewardBand(t *testing.T) {
	input, h := setup(t)
	params := input.OracleKeeper.GetParams(input.Ctx)
	params.Whitelist = []asset.Pair{asset.Registry.Pair(denoms.NIBI, denoms.NUSD)}
	input.OracleKeeper.SetParams(input.Ctx, params)

	// clear pairs to reset vote targets
	for _, p := range input.OracleKeeper.WhitelistedPairs.Iterate(input.Ctx, collections.Range[asset.Pair]{}).Keys() {
		input.OracleKeeper.WhitelistedPairs.Delete(input.Ctx, p)
	}
	input.OracleKeeper.WhitelistedPairs.Insert(input.Ctx, asset.Registry.Pair(denoms.NIBI, denoms.NUSD))

	rewardSpread := randomExchangeRate.Mul(input.OracleKeeper.RewardBand(input.Ctx).QuoInt64(2))

	// no one will miss the vote
	// Account 1, nibi:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate.Sub(rewardSpread)},
	}, 0)

	// Account 2, nibi:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate},
	}, 1)

	// Account 3, nibi:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate.Add(rewardSpread)},
	}, 2)

	// Account 4, nibi:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate.Add(rewardSpread)},
	}, 3)

	oracle.EndBlocker(input.Ctx, input.OracleKeeper)

	require.Equal(t, uint64(0), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[0], 0))
	require.Equal(t, uint64(0), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[1], 0))
	require.Equal(t, uint64(0), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[2], 0))
	require.Equal(t, uint64(0), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[3], 0))

	// Account 1 will miss the vote due to raward band condition
	// Account 1, nibi:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate.Sub(rewardSpread.Add(sdk.OneDec()))},
	}, 0)

	// Account 2, nibi:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate},
	}, 1)

	// Account 3, nibi:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate},
	}, 2)

	// Account 4, nibi:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate},
	}, 3)

	oracle.EndBlocker(input.Ctx, input.OracleKeeper)

	require.Equal(t, uint64(1), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[0], 0))
	require.Equal(t, uint64(0), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[1], 0))
	require.Equal(t, uint64(0), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[2], 0))
	require.Equal(t, uint64(0), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[3], 0))
}

/* TODO(Mercilex): not appliable right now: https://github.com/NibiruChain/nibiru/issues/805
func TestOracleMultiRewardDistribution(t *testing.T) {
	input, h := setup(t)

	// SDR and KRW have the same voting power, but KRW has been chosen as referencepair by alphabetical order.
	// Account 1, SDR, KRW
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: common.Pairbtc:nusd.String(), ExchangeRate: randomExchangeRate}, {Pair: common.Pairnibi:nusd.String(), ExchangeRate: randomExchangeRate}}, 0)

	// Account 2, SDR
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: common.Pairbtc:nusd.String(), ExchangeRate: randomExchangeRate}}, 1)

	// Account 3, KRW
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: common.Pairbtc:nusd.String(), ExchangeRate: randomExchangeRate}}, 2)

	rewardAmt := sdk.NewInt(100000000)
	err := input.BankKeeper.MintCoins(input.Ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(denoms.Gov, rewardAmt)))
	require.NoError(t, err)

	oracle.EndBlocker(input.Ctx.WithBlockHeight(1), input.OracleKeeper)

	rewardDistributedWindow := input.OracleKeeper.RewardDistributionWindow(input.Ctx)

	expectedRewardAmt := sdk.NewDecFromInt(rewardAmt.QuoRaw(3).MulRaw(2)).QuoInt64(int64(rewardDistributedWindow)).TruncateInt()
	expectedRewardAmt2 := sdk.ZeroInt() // even vote power is same KRW with SDR, KRW chosen referenceTerra because alphabetical order
	expectedRewardAmt3 := sdk.NewDecFromInt(rewardAmt.QuoRaw(3)).QuoInt64(int64(rewardDistributedWindow)).TruncateInt()

	rewards := input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[0])
	require.Equal(t, expectedRewardAmt, rewards.Rewards.AmountOf(denoms.Gov).TruncateInt())
	rewards = input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[1])
	require.Equal(t, expectedRewardAmt2, rewards.Rewards.AmountOf(denoms.Gov).TruncateInt())
	rewards = input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[2])
	require.Equal(t, expectedRewardAmt3, rewards.Rewards.AmountOf(denoms.Gov).TruncateInt())
}
*/

func TestOracleExchangeRate(t *testing.T) {
	// The following scenario tests four validators providing prices for eth:nusd, nibi:nusd, and btc:nusd.
	// eth:nusd and nibi:nusd pass, but btc:nusd fails due to not enough validators voting.
	input, h := setup(t)

	nibiNusdExchangeRate := sdk.NewDec(1000000000)
	ethNusdExchangeRate := sdk.NewDec(1000000)
	btcNusdExchangeRate := sdk.NewDec(100000000)

	// Account 1, eth:nusd, nibi:nusd, btc:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.ETH, denoms.NUSD), ExchangeRate: ethNusdExchangeRate},
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: nibiNusdExchangeRate},
		{Pair: asset.Registry.Pair(denoms.BTC, denoms.NUSD), ExchangeRate: btcNusdExchangeRate},
	}, 0)

	// Account 2, eth:nusd, nibi:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.ETH, denoms.NUSD), ExchangeRate: ethNusdExchangeRate},
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: nibiNusdExchangeRate},
	}, 1)

	// Account 3, eth:nusd, nibi:nusd, btc:nusd(abstain)
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.ETH, denoms.NUSD), ExchangeRate: ethNusdExchangeRate},
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: nibiNusdExchangeRate},
		{Pair: asset.Registry.Pair(denoms.BTC, denoms.NUSD), ExchangeRate: sdk.ZeroDec()},
	}, 2)

	// Account 4, eth:nusd, nibi:nusd, btc:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.ETH, denoms.NUSD), ExchangeRate: ethNusdExchangeRate},
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: nibiNusdExchangeRate},
		{Pair: asset.Registry.Pair(denoms.BTC, denoms.NUSD), ExchangeRate: sdk.ZeroDec()},
	}, 3)

	ethNusdRewards := sdk.NewInt64Coin("ETHREWARD", 1*common.Precision)
	nibiNusdRewards := sdk.NewInt64Coin("NIBIREWARD", 1*common.Precision)

	keeper.AllocateRewards(t, input, asset.Registry.Pair(denoms.ETH, denoms.NUSD), sdk.NewCoins(ethNusdRewards), 1)
	keeper.AllocateRewards(t, input, asset.Registry.Pair(denoms.NIBI, denoms.NUSD), sdk.NewCoins(nibiNusdRewards), 1)

	oracle.EndBlocker(input.Ctx.WithBlockHeight(1), input.OracleKeeper)

	// total reward pool for the current vote period is 1* common.Precision for eth:nusd and 1* common.Precision for nibi:nusd
	// val 1,2,3,4 all won on 2 pairs
	// so total votes are 2 * 2 + 2 + 2 = 8
	expectedRewardAmt := sdk.NewDecCoinsFromCoins(ethNusdRewards, nibiNusdRewards).
		QuoDec(sdk.NewDec(8)). // total votes
		MulDec(sdk.NewDec(2))  // votes won by val1 and val2
	rewards := input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[0])
	require.Equalf(t, expectedRewardAmt, rewards.Rewards, "%s <-> %s", expectedRewardAmt, rewards.Rewards)
	rewards = input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[1])
	require.Equalf(t, expectedRewardAmt, rewards.Rewards, "%s <-> %s", expectedRewardAmt, rewards.Rewards)
	rewards = input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[2])
	require.Equalf(t, expectedRewardAmt, rewards.Rewards, "%s <-> %s", expectedRewardAmt, rewards.Rewards)
	rewards = input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[3])
	require.Equalf(t, expectedRewardAmt, rewards.Rewards, "%s <-> %s", expectedRewardAmt, rewards.Rewards)
}

func TestOracleEnsureSorted(t *testing.T) {
	input, h := setup(t)

	for i := 0; i < 100; i++ {
		nibiNusdRate1 := sdk.NewDec(int64(rand.Uint64() % 100000000))
		ethNusdRate1 := sdk.NewDec(int64(rand.Uint64() % 100000000))

		nibiNusdRate2 := sdk.NewDec(int64(rand.Uint64() % 100000000))
		ethNusdRate2 := sdk.NewDec(int64(rand.Uint64() % 100000000))

		nibiNusdRate3 := sdk.NewDec(int64(rand.Uint64() % 100000000))
		ethNusdRate3 := sdk.NewDec(int64(rand.Uint64() % 100000000))

		// Account 1, eth:nusd, nibi:nusd
		makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.ETH, denoms.NUSD), ExchangeRate: ethNusdRate1}, {Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: nibiNusdRate1}}, 0)

		// Account 2, eth:nusd, nibi:nusd
		makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.ETH, denoms.NUSD), ExchangeRate: ethNusdRate2}, {Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: nibiNusdRate2}}, 1)

		// Account 3, eth:nusd, nibi:nusd
		makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.ETH, denoms.NUSD), ExchangeRate: nibiNusdRate3}, {Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: ethNusdRate3}}, 2)

		require.NotPanics(t, func() {
			oracle.EndBlocker(input.Ctx.WithBlockHeight(1), input.OracleKeeper)
		})
	}
}

func TestOracleExchangeRateVal5(t *testing.T) {
	input, h := setupVal5(t)

	nibiNusdRate1 := sdk.NewDec(505000)
	nibiNusdRate2 := sdk.NewDec(500000)
	ethNusdRate1 := sdk.NewDec(505)
	ethNusdRate2 := sdk.NewDec(500)

	// nibi:nusd has been chosen as reference pair by highest voting power
	// Account 1, nibi:nusd, eth:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: nibiNusdRate1},
		{Pair: asset.Registry.Pair(denoms.ETH, denoms.NUSD), ExchangeRate: ethNusdRate1},
	}, 0)

	// Account 2, nibi:nusd, eth:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: nibiNusdRate1},
		{Pair: asset.Registry.Pair(denoms.ETH, denoms.NUSD), ExchangeRate: ethNusdRate1},
	}, 1)

	// Account 3, nibi:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: nibiNusdRate1},
	}, 2)

	// Account 4, nibi:nusd, eth:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.ETH, denoms.NUSD), ExchangeRate: ethNusdRate1},
	}, 3)

	// Account 5, nibi:nusd, eth:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: nibiNusdRate2},
		{Pair: asset.Registry.Pair(denoms.ETH, denoms.NUSD), ExchangeRate: ethNusdRate2},
	}, 4)

	ethNusdRewards := sdk.NewInt64Coin("ETHRWARD", 1*common.Precision)
	nibiNusdRewards := sdk.NewInt64Coin("NIBIREWARD", 1*common.Precision)

	keeper.AllocateRewards(t, input, asset.Registry.Pair(denoms.ETH, denoms.NUSD), sdk.NewCoins(ethNusdRewards), 1)
	keeper.AllocateRewards(t, input, asset.Registry.Pair(denoms.NIBI, denoms.NUSD), sdk.NewCoins(nibiNusdRewards), 1)

	oracle.EndBlocker(input.Ctx.WithBlockHeight(1), input.OracleKeeper)

	gotnibiNusdRate, err := input.OracleKeeper.ExchangeRates.Get(input.Ctx, asset.Registry.Pair(denoms.NIBI, denoms.NUSD))
	require.NoError(t, err)
	gotethNusdRate, err := input.OracleKeeper.ExchangeRates.Get(input.Ctx, asset.Registry.Pair(denoms.ETH, denoms.NUSD))
	require.NoError(t, err)

	require.Equal(t, nibiNusdRate1, gotnibiNusdRate)
	require.Equal(t, ethNusdRate1, gotethNusdRate)

	// votes are 8 in total
	// 2 wins by val1,2,5
	// 1 win by val3,4
	expectedRewardAmt := sdk.NewDecCoinsFromCoins(ethNusdRewards, nibiNusdRewards).
		QuoDec(sdk.NewDec(8)). // total votes
		MulDec(sdk.NewDec(2))  // wins
	expectedRewardAmt2 := sdk.NewDecCoinsFromCoins(ethNusdRewards, nibiNusdRewards).
		QuoDec(sdk.NewDec(8)). // total votes
		MulDec(sdk.NewDec(1))  // wins
	rewards := input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[0])
	require.Equalf(t, expectedRewardAmt, rewards.Rewards, "%s <-> %s", expectedRewardAmt, rewards.Rewards)
	rewards1 := input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[1])
	require.Equalf(t, expectedRewardAmt, rewards1.Rewards, "%s <-> %s", expectedRewardAmt2, rewards1)
	rewards2 := input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[2])
	require.Equalf(t, expectedRewardAmt2, rewards2.Rewards, "%s <-> %s", expectedRewardAmt2, rewards2)
	rewards3 := input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[3])
	require.Equalf(t, expectedRewardAmt2, rewards3.Rewards, "%s <-> %s", expectedRewardAmt, rewards3)
	rewards4 := input.DistrKeeper.GetValidatorOutstandingRewards(input.Ctx.WithBlockHeight(2), keeper.ValAddrs[4])
	require.Equalf(t, expectedRewardAmt, rewards4.Rewards, "%s <->", expectedRewardAmt, rewards4)
}

func TestWhitelistedPairs(t *testing.T) {
	input, h := setup(t)
	params := input.OracleKeeper.GetParams(input.Ctx)

	for _, p := range input.OracleKeeper.WhitelistedPairs.Iterate(input.Ctx, collections.Range[asset.Pair]{}).Keys() {
		input.OracleKeeper.WhitelistedPairs.Delete(input.Ctx, p)
	}
	input.OracleKeeper.WhitelistedPairs.Insert(input.Ctx, asset.Registry.Pair(denoms.NIBI, denoms.NUSD))

	// nibi:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 0)
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 1)
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 2)

	// set btc:nusd for next vote period
	params.Whitelist = []asset.Pair{asset.Registry.Pair(denoms.NIBI, denoms.NUSD), asset.Registry.Pair(denoms.BTC, denoms.NUSD)}
	input.OracleKeeper.SetParams(input.Ctx, params)
	oracle.EndBlocker(input.Ctx, input.OracleKeeper)

	// no missing current
	require.Equal(t, uint64(0), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[0], 0))
	require.Equal(t, uint64(0), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[1], 0))
	require.Equal(t, uint64(0), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[2], 0))

	// whitelisted pairs are {nibi:nusd, btc:nusd}
	require.Equal(t, []asset.Pair{asset.Registry.Pair(denoms.BTC, denoms.NUSD), asset.Registry.Pair(denoms.NIBI, denoms.NUSD)}, input.OracleKeeper.GetWhitelistedPairs(input.Ctx))

	// nibi:nusd, missing btc:nusd
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 0)
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 1)
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 2)

	// delete btc:nusd for next vote period
	params.Whitelist = []asset.Pair{asset.Registry.Pair(denoms.NIBI, denoms.NUSD)}
	input.OracleKeeper.SetParams(input.Ctx, params)
	oracle.EndBlocker(input.Ctx, input.OracleKeeper)

	require.Equal(t, uint64(1), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[0], 0))
	require.Equal(t, uint64(1), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[1], 0))
	require.Equal(t, uint64(1), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[2], 0))

	// btc:nusd must be deleted
	require.Equal(t, []asset.Pair{asset.Registry.Pair(denoms.NIBI, denoms.NUSD)}, input.OracleKeeper.GetWhitelistedPairs(input.Ctx))
	require.False(t, input.OracleKeeper.WhitelistedPairs.Has(input.Ctx, asset.Registry.Pair(denoms.BTC, denoms.NUSD)))

	// nibi:nusd, no missing
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 0)
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 1)
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 2)

	oracle.EndBlocker(input.Ctx, input.OracleKeeper)

	require.Equal(t, uint64(1), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[0], 0))
	require.Equal(t, uint64(1), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[1], 0))
	require.Equal(t, uint64(1), input.OracleKeeper.MissCounters.GetOr(input.Ctx, keeper.ValAddrs[2], 0))
}

func TestAbstainWithSmallStakingPower(t *testing.T) {
	input, h := setupWithSmallVotingPower(t)

	// clear tobin tax to reset vote targets
	for _, p := range input.OracleKeeper.WhitelistedPairs.Iterate(input.Ctx, collections.Range[asset.Pair]{}).Keys() {
		input.OracleKeeper.WhitelistedPairs.Delete(input.Ctx, p)
	}
	input.OracleKeeper.WhitelistedPairs.Insert(input.Ctx, asset.Registry.Pair(denoms.NIBI, denoms.NUSD))
	makeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: sdk.ZeroDec()}}, 0)

	oracle.EndBlocker(input.Ctx, input.OracleKeeper)
	_, err := input.OracleKeeper.ExchangeRates.Get(input.Ctx, asset.Registry.Pair(denoms.NIBI, denoms.NUSD))
	require.Error(t, err)
}

func makeAggregatePrevoteAndVote(t *testing.T, input keeper.TestInput, h types.MsgServer, height int64, rates types.ExchangeRateTuples, valIdx int) {
	salt := "1"
	ratesStr, err := rates.ToString()
	require.NoError(t, err)
	hash := types.GetAggregateVoteHash(salt, ratesStr, keeper.ValAddrs[valIdx])

	prevoteMsg := types.NewMsgAggregateExchangeRatePrevote(hash, keeper.Addrs[valIdx], keeper.ValAddrs[valIdx])
	_, err = h.AggregateExchangeRatePrevote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(height)), prevoteMsg)
	require.NoError(t, err)

	voteMsg := types.NewMsgAggregateExchangeRateVote(salt, ratesStr, keeper.Addrs[valIdx], keeper.ValAddrs[valIdx])
	_, err = h.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx.WithBlockHeight(height+1)), voteMsg)
	require.NoError(t, err)
}

func setupWithSmallVotingPower(t *testing.T) (keeper.TestInput, types.MsgServer) {
	input := keeper.CreateTestInput(t)
	params := input.OracleKeeper.GetParams(input.Ctx)
	params.VotePeriod = 1
	params.SlashWindow = 100
	input.OracleKeeper.SetParams(input.Ctx, params)
	h := keeper.NewMsgServerImpl(input.OracleKeeper)

	sh := staking.NewHandler(input.StakingKeeper)
	_, err := sh(input.Ctx, keeper.NewTestMsgCreateValidator(keeper.ValAddrs[0], keeper.ValPubKeys[0], sdk.TokensFromConsensusPower(1, sdk.DefaultPowerReduction)))
	require.NoError(t, err)

	staking.EndBlocker(input.Ctx, input.StakingKeeper)

	return input, h
}

func setupVal5(t *testing.T) (keeper.TestInput, types.MsgServer) {
	input := keeper.CreateTestInput(t)
	params := input.OracleKeeper.GetParams(input.Ctx)
	params.VotePeriod = 1
	params.SlashWindow = 100
	input.OracleKeeper.SetParams(input.Ctx, params)
	h := keeper.NewMsgServerImpl(input.OracleKeeper)

	sh := staking.NewHandler(input.StakingKeeper)

	// Validator created
	_, err := sh(input.Ctx, keeper.NewTestMsgCreateValidator(keeper.ValAddrs[0], keeper.ValPubKeys[0], stakingAmt))
	require.NoError(t, err)
	_, err = sh(input.Ctx, keeper.NewTestMsgCreateValidator(keeper.ValAddrs[1], keeper.ValPubKeys[1], stakingAmt))
	require.NoError(t, err)
	_, err = sh(input.Ctx, keeper.NewTestMsgCreateValidator(keeper.ValAddrs[2], keeper.ValPubKeys[2], stakingAmt))
	require.NoError(t, err)
	_, err = sh(input.Ctx, keeper.NewTestMsgCreateValidator(keeper.ValAddrs[3], keeper.ValPubKeys[3], stakingAmt))
	require.NoError(t, err)
	_, err = sh(input.Ctx, keeper.NewTestMsgCreateValidator(keeper.ValAddrs[4], keeper.ValPubKeys[4], stakingAmt))
	require.NoError(t, err)
	staking.EndBlocker(input.Ctx, input.StakingKeeper)

	return input, h
}
