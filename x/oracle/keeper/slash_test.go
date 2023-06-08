package keeper

import (
	"testing"

	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/collections"

	"github.com/NibiruChain/nibiru/x/common/asset"
	"github.com/NibiruChain/nibiru/x/common/denoms"
	"github.com/NibiruChain/nibiru/x/oracle/types"
)

func TestSlashAndResetMissCounters(t *testing.T) {
	// initial setup
	input := CreateTestFixture(t)
	addr, val := ValAddrs[0], ValPubKeys[0]
	addr1, val1 := ValAddrs[1], ValPubKeys[1]
	amt := sdk.TokensFromConsensusPower(100, sdk.DefaultPowerReduction)
	sh := stakingkeeper.NewMsgServerImpl(input.StakingKeeper)
	ctx := input.Ctx

	// Validator created
	_, err := sh.CreateValidator(ctx, NewTestMsgCreateValidator(addr, val, amt))
	require.NoError(t, err)
	_, err = sh.CreateValidator(ctx, NewTestMsgCreateValidator(addr1, val1, amt))
	require.NoError(t, err)
	staking.EndBlocker(ctx, input.StakingKeeper)

	require.Equal(
		t, input.BankKeeper.GetAllBalances(ctx, sdk.AccAddress(addr)),
		sdk.NewCoins(sdk.NewCoin(input.StakingKeeper.GetParams(ctx).BondDenom, InitTokens.Sub(amt))),
	)
	require.Equal(t, amt, input.StakingKeeper.Validator(ctx, addr).GetBondedTokens())
	require.Equal(
		t, input.BankKeeper.GetAllBalances(ctx, sdk.AccAddress(addr1)),
		sdk.NewCoins(sdk.NewCoin(input.StakingKeeper.GetParams(ctx).BondDenom, InitTokens.Sub(amt))),
	)
	require.Equal(t, amt, input.StakingKeeper.Validator(ctx, addr1).GetBondedTokens())

	votePeriodsPerWindow := sdk.NewDec(int64(input.OracleKeeper.SlashWindow(input.Ctx))).QuoInt64(int64(input.OracleKeeper.VotePeriod(input.Ctx))).TruncateInt64()
	slashFraction := input.OracleKeeper.SlashFraction(input.Ctx)
	minValidVotes := input.OracleKeeper.MinValidPerWindow(input.Ctx).MulInt64(votePeriodsPerWindow).Ceil().TruncateInt64()
	// Case 1, no slash
	input.OracleKeeper.MissCounters.Insert(input.Ctx, ValAddrs[0], uint64(votePeriodsPerWindow-minValidVotes))
	input.OracleKeeper.SlashAndResetMissCounters(input.Ctx)
	staking.EndBlocker(input.Ctx, input.StakingKeeper)

	validator, _ := input.StakingKeeper.GetValidator(input.Ctx, ValAddrs[0])
	require.Equal(t, amt, validator.GetBondedTokens())

	// Case 2, slash
	input.OracleKeeper.MissCounters.Insert(input.Ctx, ValAddrs[0], uint64(votePeriodsPerWindow-minValidVotes+1))
	input.OracleKeeper.SlashAndResetMissCounters(input.Ctx)
	validator, _ = input.StakingKeeper.GetValidator(input.Ctx, ValAddrs[0])
	require.Equal(t, amt.Sub(slashFraction.MulInt(amt).TruncateInt()), validator.GetBondedTokens())
	require.True(t, validator.IsJailed())

	// Case 3, slash unbonded validator
	validator, _ = input.StakingKeeper.GetValidator(input.Ctx, ValAddrs[0])
	validator.Status = stakingtypes.Unbonded
	validator.Jailed = false
	validator.Tokens = amt
	input.StakingKeeper.SetValidator(input.Ctx, validator)

	input.OracleKeeper.MissCounters.Insert(input.Ctx, ValAddrs[0], uint64(votePeriodsPerWindow-minValidVotes+1))
	input.OracleKeeper.SlashAndResetMissCounters(input.Ctx)
	validator, _ = input.StakingKeeper.GetValidator(input.Ctx, ValAddrs[0])
	require.Equal(t, amt, validator.Tokens)
	require.False(t, validator.IsJailed())

	// Case 4, slash jailed validator
	validator, _ = input.StakingKeeper.GetValidator(input.Ctx, ValAddrs[0])
	validator.Status = stakingtypes.Bonded
	validator.Jailed = true
	validator.Tokens = amt
	input.StakingKeeper.SetValidator(input.Ctx, validator)

	input.OracleKeeper.MissCounters.Insert(input.Ctx, ValAddrs[0], uint64(votePeriodsPerWindow-minValidVotes+1))
	input.OracleKeeper.SlashAndResetMissCounters(input.Ctx)
	validator, _ = input.StakingKeeper.GetValidator(input.Ctx, ValAddrs[0])
	require.Equal(t, amt, validator.Tokens)
}

func TestInvalidVotesSlashing(t *testing.T) {
	input, h := Setup(t)
	params, err := input.OracleKeeper.Params.Get(input.Ctx)
	require.NoError(t, err)
	params.Whitelist = asset.Pairs{asset.Registry.Pair(denoms.NIBI, denoms.NUSD)}
	input.OracleKeeper.Params.Set(input.Ctx, params)
	input.OracleKeeper.WhitelistedPairs.Insert(input.Ctx, asset.Registry.Pair(denoms.NIBI, denoms.NUSD))

	votePeriodsPerWindow := sdk.NewDec(int64(input.OracleKeeper.SlashWindow(input.Ctx))).QuoInt64(int64(input.OracleKeeper.VotePeriod(input.Ctx))).TruncateInt64()
	slashFraction := input.OracleKeeper.SlashFraction(input.Ctx)
	minValidPerWindow := input.OracleKeeper.MinValidPerWindow(input.Ctx)

	for i := uint64(0); i < uint64(sdk.OneDec().Sub(minValidPerWindow).MulInt64(votePeriodsPerWindow).TruncateInt64()); i++ {
		input.Ctx = input.Ctx.WithBlockHeight(input.Ctx.BlockHeight() + 1)

		// Account 1, govstable
		MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
			{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate},
		}, 0)

		// Account 2, govstable, miss vote
		MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
			{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate.Add(sdk.NewDec(100000000000000))},
		}, 1)

		// Account 3, govstable
		MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
			{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate},
		}, 2)

		// Account 4, govstable
		MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
			{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate},
		}, 3)

		input.OracleKeeper.UpdateExchangeRates(input.Ctx)
		// input.OracleKeeper.SlashAndResetMissCounters(input.Ctx)
		// input.OracleKeeper.UpdateExchangeRates(input.Ctx)

		require.Equal(t, i+1, input.OracleKeeper.MissCounters.GetOr(input.Ctx, ValAddrs[1], 0))
	}

	validator := input.StakingKeeper.Validator(input.Ctx, ValAddrs[1])
	require.Equal(t, stakingAmt, validator.GetBondedTokens())

	// one more miss vote will inccur ValAddrs[1] slashing
	// Account 1, govstable
	MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate},
	}, 0)

	// Account 2, govstable, miss vote
	MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate.Add(sdk.NewDec(100000000000000))},
	}, 1)

	// Account 3, govstable
	MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate},
	}, 2)

	// Account 4, govstable
	MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{
		{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate},
	}, 3)

	input.Ctx = input.Ctx.WithBlockHeight(votePeriodsPerWindow - 1)
	input.OracleKeeper.UpdateExchangeRates(input.Ctx)
	input.OracleKeeper.SlashAndResetMissCounters(input.Ctx)
	// input.OracleKeeper.UpdateExchangeRates(input.Ctx)

	validator = input.StakingKeeper.Validator(input.Ctx, ValAddrs[1])
	require.Equal(t, sdk.OneDec().Sub(slashFraction).MulInt(stakingAmt).TruncateInt(), validator.GetBondedTokens())
}

func TestWhitelistSlashing(t *testing.T) {
	input, h := Setup(t)

	votePeriodsPerWindow := sdk.NewDec(int64(input.OracleKeeper.SlashWindow(input.Ctx))).QuoInt64(int64(input.OracleKeeper.VotePeriod(input.Ctx))).TruncateInt64()
	slashFraction := input.OracleKeeper.SlashFraction(input.Ctx)
	minValidPerWindow := input.OracleKeeper.MinValidPerWindow(input.Ctx)

	for i := uint64(0); i < uint64(sdk.OneDec().Sub(minValidPerWindow).MulInt64(votePeriodsPerWindow).TruncateInt64()); i++ {
		input.Ctx = input.Ctx.WithBlockHeight(input.Ctx.BlockHeight() + 1)

		// Account 2, govstable
		MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 1)
		// Account 3, govstable
		MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 2)

		input.OracleKeeper.UpdateExchangeRates(input.Ctx)
		// input.OracleKeeper.SlashAndResetMissCounters(input.Ctx)
		// input.OracleKeeper.UpdateExchangeRates(input.Ctx)
		require.Equal(t, i+1, input.OracleKeeper.MissCounters.GetOr(input.Ctx, ValAddrs[0], 0))
	}

	validator := input.StakingKeeper.Validator(input.Ctx, ValAddrs[0])
	require.Equal(t, stakingAmt, validator.GetBondedTokens())

	// one more miss vote will inccur Account 1 slashing

	// Account 2, govstable
	MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 1)
	// Account 3, govstable
	MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 2)

	input.Ctx = input.Ctx.WithBlockHeight(votePeriodsPerWindow - 1)
	input.OracleKeeper.UpdateExchangeRates(input.Ctx)
	input.OracleKeeper.SlashAndResetMissCounters(input.Ctx)
	// input.OracleKeeper.UpdateExchangeRates(input.Ctx)
	validator = input.StakingKeeper.Validator(input.Ctx, ValAddrs[0])
	require.Equal(t, sdk.OneDec().Sub(slashFraction).MulInt(stakingAmt).TruncateInt(), validator.GetBondedTokens())
}

func TestNotPassedBallotSlashing(t *testing.T) {
	input, h := Setup(t)
	params, err := input.OracleKeeper.Params.Get(input.Ctx)
	require.NoError(t, err)
	params.Whitelist = []asset.Pair{asset.Registry.Pair(denoms.NIBI, denoms.NUSD)}
	input.OracleKeeper.Params.Set(input.Ctx, params)

	// clear tobin tax to reset vote targets
	for _, p := range input.OracleKeeper.WhitelistedPairs.Iterate(input.Ctx, collections.Range[asset.Pair]{}).Keys() {
		input.OracleKeeper.WhitelistedPairs.Delete(input.Ctx, p)
	}
	input.OracleKeeper.WhitelistedPairs.Insert(input.Ctx, asset.Registry.Pair(denoms.NIBI, denoms.NUSD))

	input.Ctx = input.Ctx.WithBlockHeight(input.Ctx.BlockHeight() + 1)

	// Account 1, govstable
	MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 0)

	input.OracleKeeper.UpdateExchangeRates(input.Ctx)
	input.OracleKeeper.SlashAndResetMissCounters(input.Ctx)
	// input.OracleKeeper.UpdateExchangeRates(input.Ctx)
	require.Equal(t, uint64(0), input.OracleKeeper.MissCounters.GetOr(input.Ctx, ValAddrs[0], 0))
	require.Equal(t, uint64(0), input.OracleKeeper.MissCounters.GetOr(input.Ctx, ValAddrs[1], 0))
	require.Equal(t, uint64(0), input.OracleKeeper.MissCounters.GetOr(input.Ctx, ValAddrs[2], 0))
}

func TestAbstainSlashing(t *testing.T) {
	input, h := Setup(t)
	params, err := input.OracleKeeper.Params.Get(input.Ctx)
	require.NoError(t, err)
	params.Whitelist = []asset.Pair{asset.Registry.Pair(denoms.NIBI, denoms.NUSD)}
	input.OracleKeeper.Params.Set(input.Ctx, params)

	// clear tobin tax to reset vote targets
	for _, p := range input.OracleKeeper.WhitelistedPairs.Iterate(input.Ctx, collections.Range[asset.Pair]{}).Keys() {
		input.OracleKeeper.WhitelistedPairs.Delete(input.Ctx, p)
	}
	input.OracleKeeper.WhitelistedPairs.Insert(input.Ctx, asset.Registry.Pair(denoms.NIBI, denoms.NUSD))

	votePeriodsPerWindow := sdk.NewDec(int64(input.OracleKeeper.SlashWindow(input.Ctx))).QuoInt64(int64(input.OracleKeeper.VotePeriod(input.Ctx))).TruncateInt64()
	minValidPerWindow := input.OracleKeeper.MinValidPerWindow(input.Ctx)

	for i := uint64(0); i <= uint64(sdk.OneDec().Sub(minValidPerWindow).MulInt64(votePeriodsPerWindow).TruncateInt64()); i++ {
		input.Ctx = input.Ctx.WithBlockHeight(input.Ctx.BlockHeight() + 1)

		// Account 1, govstable
		MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 0)

		// Account 2, govstable, abstain vote
		MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: sdk.ZeroDec()}}, 1)

		// Account 3, govstable
		MakeAggregatePrevoteAndVote(t, input, h, 0, types.ExchangeRateTuples{{Pair: asset.Registry.Pair(denoms.NIBI, denoms.NUSD), ExchangeRate: randomExchangeRate}}, 2)

		input.OracleKeeper.UpdateExchangeRates(input.Ctx)
		input.OracleKeeper.SlashAndResetMissCounters(input.Ctx)
		// input.OracleKeeper.UpdateExchangeRates(input.Ctx)
		require.Equal(t, uint64(0), input.OracleKeeper.MissCounters.GetOr(input.Ctx, ValAddrs[1], 0))
	}

	validator := input.StakingKeeper.Validator(input.Ctx, ValAddrs[1])
	require.Equal(t, stakingAmt, validator.GetBondedTokens())
}
