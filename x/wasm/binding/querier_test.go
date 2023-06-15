package binding_test

import (
	"encoding/json"
	"testing"
	"time"

	sdkerrors "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

	"github.com/NibiruChain/nibiru/app"
	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/common/asset"
	"github.com/NibiruChain/nibiru/x/common/denoms"
	"github.com/NibiruChain/nibiru/x/common/testutil"
	"github.com/NibiruChain/nibiru/x/common/testutil/genesis"
	"github.com/NibiruChain/nibiru/x/common/testutil/testapp"
	"github.com/NibiruChain/nibiru/x/wasm/binding/cw_struct"
	"github.com/NibiruChain/nibiru/x/wasm/binding/wasmbin"
)

func TestSuiteQuerier_RunAll(t *testing.T) {
	suite.Run(t, new(TestSuiteQuerier))
}

func DoCustomBindingQuery(
	ctx sdk.Context,
	nibiru *app.NibiruApp,
	contract sdk.AccAddress,
	bindingRequest cw_struct.BindingQuery,
	responsePointer interface{},
) (contractRespBz []byte, err error) {
	// Parse query type compatible with wasm vm
	reqJsonBz, err := json.Marshal(bindingRequest)
	if err != nil {
		return contractRespBz, err
	}

	// Query the smart contract
	var originalError error
	if err := common.TryCatch(func() {
		// The WasmVM tends to panic pretty easily with "Wasmer runtimer error".
		// TryCatch here makes it more safe and easy to debug.
		bz, err := nibiru.WasmKeeper.QuerySmart(
			ctx, contract, reqJsonBz,
		)
		if err != nil {
			originalError = err
		} else {
			contractRespBz = bz
		}
	})(); err != nil {
		return contractRespBz, sdkerrors.Wrapf(
			err, "contractRespBz: %s", contractRespBz)
	}

	// originalError: the error raised if the WasmVM doesn't panic
	if originalError != nil {
		return contractRespBz, originalError
	}

	// Parse the response data into the response pointer
	err = json.Unmarshal(contractRespBz, responsePointer)
	if err != nil {
		return contractRespBz, sdkerrors.Wrapf(
			err, "responsePointer: %s", responsePointer)
	}

	return contractRespBz, nil
}

type TestSuiteQuerier struct {
	suite.Suite

	nibiru           *app.NibiruApp
	ctx              sdk.Context
	contractDeployer sdk.AccAddress

	contractPerp sdk.AccAddress
	fields       ExampleFields
}

type ExampleFields struct {
	Pair   string
	Trader sdk.AccAddress
	Dec    sdk.Dec
	Int    sdkmath.Int
	Market cw_struct.Market
}

func GetHappyFields() ExampleFields {
	fields := ExampleFields{
		Pair:   asset.Registry.Pair(denoms.ETH, denoms.NUSD).String(),
		Trader: sdk.AccAddress([]byte("trader")),
		Dec:    sdk.NewDec(50),
		Int:    sdk.NewInt(420),
	}

	fields.Market = cw_struct.Market{
		Pair:         fields.Pair,
		BaseReserve:  fields.Dec,
		QuoteReserve: fields.Dec,
		SqrtDepth:    fields.Dec,
		Depth:        fields.Int,
		TotalLong:    fields.Dec,
		TotalShort:   fields.Dec,
		PegMult:      fields.Dec,
		Config: &cw_struct.MarketConfig{
			MaintenanceMarginRatio: fields.Dec,
			MaxLeverage:            fields.Dec,
		},
		MarkPrice:   fields.Dec,
		IndexPrice:  fields.Dec.String(),
		TwapMark:    fields.Dec.String(),
		BlockNumber: sdk.NewInt(100),
	}
	return fields
}

func (s *TestSuiteQuerier) SetupSuite() {
	s.fields = GetHappyFields()
	sender := testutil.AccAddress()
	s.contractDeployer = sender

	genesisState := SetupPerpGenesis()
	nibiru := testapp.NewNibiruTestApp(genesisState)
	ctx := nibiru.NewContext(false, tmproto.Header{
		Height:  1,
		ChainID: "nibiru-wasmnet-1",
		Time:    time.Now().UTC(),
	})
	coins := sdk.NewCoins(
		sdk.NewCoin(denoms.NIBI, sdk.NewInt(1_000)),
		sdk.NewCoin(denoms.NUSD, sdk.NewInt(420)),
	)
	s.NoError(testapp.FundAccount(nibiru.BankKeeper, ctx, sender, coins))

	nibiru, ctx = SetupAllContracts(s.T(), sender, nibiru, ctx)
	s.nibiru = nibiru
	s.ctx = ctx

	s.contractPerp = ContractMap[wasmbin.WasmKeyPerpBinding]
	s.OnSetupEnd()
}

func (s *TestSuiteQuerier) OnSetupEnd() {
	SetExchangeRates(s.Suite, s.nibiru, s.ctx)
}

func (s *TestSuiteQuerier) TestQueryReserves() {
	testCases := map[string]struct {
		pairStr   string
		wasmError bool
	}{
		"happy":                   {pairStr: s.fields.Pair, wasmError: false},
		"sad - non existent pair": {pairStr: "ftt:ust", wasmError: true},
	}

	for name, testCase := range testCases {
		s.T().Run(name, func(t *testing.T) {
			pairStr := testCase.pairStr
			bindingQuery := cw_struct.BindingQuery{
				Reserves: &cw_struct.ReservesRequest{Pair: pairStr},
			}
			bindingResp := new(cw_struct.ReservesResponse)

			if testCase.wasmError {
				_, err := DoCustomBindingQuery(
					s.ctx, s.nibiru, s.contractPerp,
					bindingQuery, bindingResp,
				)
				s.Assert().Contains(err.Error(), "query wasm contract failed")
				return
			}

			_, err := DoCustomBindingQuery(
				s.ctx, s.nibiru, s.contractPerp, bindingQuery, bindingResp,
			)
			s.Require().NoError(err)

			wantPair := asset.MustNewPair(pairStr)
			s.Assert().EqualValues(bindingResp.Pair, wantPair)
			s.Assert().EqualValues(
				bindingResp.BaseReserve.String(),
				genesis.START_MARKETS[wantPair].Amm.BaseReserve.String())
			s.Assert().EqualValues(
				bindingResp.QuoteReserve.String(),
				genesis.START_MARKETS[wantPair].Amm.QuoteReserve.String())
		})
	}
}

// Integration test for BindingQuery::AllMarkets against real contract
func (s *TestSuiteQuerier) TestQueryAllMarkets() {
	bindingQuery := cw_struct.BindingQuery{
		AllMarkets: &cw_struct.AllMarketsRequest{},
	}
	bindingResp := new(cw_struct.AllMarketsResponse)

	respBz, err := DoCustomBindingQuery(
		s.ctx, s.nibiru, s.contractPerp, bindingQuery, bindingResp,
	)
	s.Require().NoErrorf(err, "resp bytes: %s", respBz)

	for pair, marketAmm := range genesis.START_MARKETS {
		cwMarket := bindingResp.MarketMap[pair.String()]
		s.Assert().EqualValues(marketAmm.Amm.BaseReserve, cwMarket.BaseReserve)
		s.Assert().EqualValues(marketAmm.Amm.QuoteReserve, cwMarket.QuoteReserve)
		s.Assert().EqualValues(marketAmm.Amm.SqrtDepth, cwMarket.SqrtDepth)
		// s.Assert().EqualValues(
		// 	marketAmm.Amm.BaseReserve.Mul(marketAmm.Amm.QuoteReserve).String(),
		// 	cwMarket.Depth.ToDec().String())
		s.Assert().EqualValues(marketAmm.Amm.TotalLong, cwMarket.TotalLong)
		s.Assert().EqualValues(marketAmm.Amm.TotalShort, cwMarket.TotalShort)
		s.Assert().EqualValues(marketAmm.Amm.PriceMultiplier.String(), cwMarket.PegMult.String())
		s.Assert().EqualValues(marketAmm.Amm.MarkPrice().String(), cwMarket.MarkPrice.String())
		s.Assert().EqualValues(s.ctx.BlockHeight(), cwMarket.BlockNumber.Int64())
	}
}

// func (s *TestSuiteQuerier) TestQueryBasePrice() {
// 	cwReq := &cw_struct.BasePriceRequest{
// 		Pair:       s.fields.Pair,
// 		IsLong:     true,
// 		BaseAmount: sdk.NewInt(69_420),
// 	}
// 	bindingQuery := cw_struct.BindingQuery{
// 		BasePrice: cwReq,
// 	}
// 	bindingResp := new(cw_struct.BasePriceResponse)

// 	respBz, err := DoCustomBindingQuery(
// 		s.ctx, s.nibiru, s.contractPerp, bindingQuery, bindingResp,
// 	)
// 	s.Require().NoErrorf(err, "resp bytes: %s", respBz)

// 	s.Assert().EqualValues(cwReq.Pair, bindingResp.Pair)
// 	s.Assert().EqualValues(cwReq.IsLong, bindingResp.IsLong)
// 	s.Assert().EqualValues(cwReq.BaseAmount.ToDec(), bindingResp.BaseAmount)
// 	s.Assert().True(bindingResp.QuoteAmount.GT(sdk.ZeroDec()))

// 	cwReqBz, err := json.Marshal(cwReq)
// 	s.T().Logf("cwReq: %s", cwReqBz)
// 	s.NoError(err)

// 	cwRespBz, err := json.Marshal(bindingResp)
// 	s.T().Logf("cwResp: %s", cwRespBz)
// 	s.NoError(err)
// }

// func (s *TestSuiteQuerier) TestQueryPremiumFraction() {
// 	cwReq := &cw_struct.PremiumFractionRequest{
// 		Pair: s.fields.Pair,
// 	}

// 	bindingQuery := cw_struct.BindingQuery{
// 		PremiumFraction: cwReq,
// 	}
// 	bindingResp := new(cw_struct.PremiumFractionResponse)

// 	respBz, err := DoCustomBindingQuery(
// 		s.ctx, s.nibiru, s.contractPerp, bindingQuery, bindingResp,
// 	)
// 	s.Require().NoErrorf(err, "resp bytes: %s", respBz)

// 	s.Assert().EqualValues(cwReq.Pair, bindingResp.Pair)
// 	s.Assert().Truef(
// 		!bindingResp.CPF.IsNegative(),
// 		"cpf: %s",
// 		bindingResp.CPF)
// 	s.Assert().Truef(
// 		!bindingResp.EstimatedNextCPF.IsNegative(),
// 		"estimated_next_cpf: %s",
// 		bindingResp.EstimatedNextCPF)
// }

// func (s *TestSuiteQuerier) TestQueryMetrics() {
// 	cwReq := &cw_struct.MetricsRequest{
// 		Pair: s.fields.Pair,
// 	}

// 	bindingQuery := cw_struct.BindingQuery{
// 		Metrics: cwReq,
// 	}
// 	bindingResp := new(cw_struct.MetricsResponse)

// 	respBz, err := DoCustomBindingQuery(
// 		s.ctx, s.nibiru, s.contractPerp, bindingQuery, bindingResp,
// 	)
// 	s.Require().NoErrorf(err, "resp bytes: %s", respBz)
// }

// func (s *TestSuiteQuerier) TestQueryPerpParams() {
// 	cwReq := &cw_struct.PerpParamsRequest{}

// 	bindingQuery := cw_struct.BindingQuery{
// 		PerpParams: cwReq,
// 	}
// 	bindingResp := new(cw_struct.PerpParamsResponse)

// 	respBz, err := DoCustomBindingQuery(
// 		s.ctx, s.nibiru, s.contractPerp, bindingQuery, bindingResp,
// 	)
// 	s.Require().NoErrorf(err, "resp bytes: %s", respBz)
// }

func (s *TestSuiteQuerier) TestQueryPerpModuleAccounts() {
	cwReq := &cw_struct.ModuleAccountsRequest{}

	bindingQuery := cw_struct.BindingQuery{
		ModuleAccounts: cwReq,
	}
	bindingResp := new(cw_struct.ModuleAccountsResponse)

	respBz, err := DoCustomBindingQuery(
		s.ctx, s.nibiru, s.contractPerp, bindingQuery, bindingResp,
	)
	s.Require().NoErrorf(err, "resp bytes: %s", respBz)
}
