package precompile_test

import (
	"fmt"
	"math/big"
	"testing"

	bank "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/NibiruChain/nibiru/x/common/testutil"
	"github.com/NibiruChain/nibiru/x/evm"
	"github.com/NibiruChain/nibiru/x/evm/embeds"
	"github.com/NibiruChain/nibiru/x/evm/evmtest"
	"github.com/NibiruChain/nibiru/x/evm/precompile"

	"github.com/stretchr/testify/suite"
)

type Suite struct {
	suite.Suite
}

// TestPrecompileSuite: Runs all the tests in the suite.
func TestPrecompileSuite(t *testing.T) {
	s := new(Suite)
	suite.Run(t, s)
}

func (s *Suite) TestPrecompile_FunToken() {
	s.Run("PrecompileExists", s.FunToken_PrecompileExists)
	s.Run("HappyPath", s.FunToken_HappyPath)
}

func CreateFunTokenForBankCoin(
	deps *evmtest.TestDeps, bankDenom string, s *Suite,
) (funtoken evm.FunToken) {
	s.T().Log("Setup: Create a coin in the bank state")
	bankMetadata := bank.Metadata{
		DenomUnits: []*bank.DenomUnit{
			{
				Denom:    bankDenom,
				Exponent: 0,
			},
		},
		Base:    bankDenom,
		Display: bankDenom,
		Name:    bankDenom,
		Symbol:  "TOKEN",
	}
	deps.Chain.BankKeeper.SetDenomMetaData(deps.Ctx, bankMetadata)

	s.T().Log("happy: CreateFunToken for the bank coin")
	createFuntokenResp, err := deps.K.CreateFunToken(
		deps.GoCtx(),
		&evm.MsgCreateFunToken{
			FromBankDenom: bankDenom,
			Sender:        deps.Sender.NibiruAddr.String(),
		},
	)
	s.NoError(err, "bankDenom %s", bankDenom)
	erc20 := createFuntokenResp.FuntokenMapping.Erc20Addr
	funtoken = evm.FunToken{
		Erc20Addr:      erc20,
		BankDenom:      bankDenom,
		IsMadeFromCoin: true,
	}
	s.Equal(createFuntokenResp.FuntokenMapping, funtoken)

	s.T().Log("Expect ERC20 to be deployed")
	erc20Addr := erc20.ToAddr()
	queryCodeReq := &evm.QueryCodeRequest{
		Address: erc20Addr.String(),
	}
	_, err = deps.K.Code(deps.Ctx, queryCodeReq)
	s.NoError(err)

	return funtoken
}

// PrecompileExists: An integration test showing that a "PrecompileError" occurs
// when calling the FunToken
func (s *Suite) FunToken_PrecompileExists() {
	precompileAddr := precompile.PrecompileAddr_FuntokenGateway
	abi := embeds.Contract_Funtoken.ABI
	deps := evmtest.NewTestDeps()

	codeResp, err := deps.K.Code(
		deps.GoCtx(),
		&evm.QueryCodeRequest{
			Address: precompileAddr.String(),
		},
	)
	s.NoError(err)
	s.Equal(string(codeResp.Code), "")

	s.True(deps.K.PrecompileSet().Has(precompileAddr.ToAddr()),
		"did not see precompile address during \"InitPrecompiles\"")

	callArgs := []any{"nonsense", "args here", "to see if", "precompile is", "called"}
	methodName := string(precompile.FunTokenMethod_BankSend)
	packedArgs, err := abi.Pack(methodName, callArgs...)
	if err != nil {
		err = fmt.Errorf("failed to pack ABI args: %w", err) // easier to read
	}
	s.ErrorContains(
		err, fmt.Sprintf("argument count mismatch: got %d for 3", len(callArgs)),
		"callArgs: ", callArgs)

	fromEvmAddr := evm.ModuleAddressEVM()
	contractAddr := precompileAddr.ToAddr()
	commit := true
	bytecodeForCall := packedArgs
	_, err = deps.K.CallContractWithInput(
		deps.Ctx, fromEvmAddr, &contractAddr, commit,
		bytecodeForCall,
	)
	s.ErrorContains(err, "Precompile error")
}

func (s *Suite) FunToken_HappyPath() {
	precompileAddr := precompile.PrecompileAddr_FuntokenGateway
	abi := embeds.Contract_Funtoken.ABI
	deps := evmtest.NewTestDeps()

	theUser := deps.Sender.EthAddr
	theEvm := evm.ModuleAddressEVM()

	s.True(deps.K.PrecompileSet().Has(precompileAddr.ToAddr()),
		"did not see precompile address during \"InitPrecompiles\"")

	s.T().Log("Create FunToken mapping and ERC20")
	bankDenom := "ibc/usdc"
	funtoken := CreateFunTokenForBankCoin(&deps, bankDenom, s)
	contract := funtoken.Erc20Addr.ToAddr()

	s.T().Log("Balances of the ERC20 should start empty")
	evmtest.AssertERC20BalanceEqual(s.T(), &deps, contract, theUser, big.NewInt(0))
	evmtest.AssertERC20BalanceEqual(s.T(), &deps, contract, theEvm, big.NewInt(0))

	s.T().Log("Mint tokens - Fail from non-owner")
	{
		from := theUser
		to := theUser
		input, err := embeds.Contract_ERC20Minter.ABI.Pack("mint", to, big.NewInt(69_420))
		s.NoError(err)
		_, err = evmtest.DoEthTx(&deps, contract, from, input)
		s.ErrorContains(err, "Ownable: caller is not the owner")
	}

	s.T().Log("Mint tokens - Success")
	{
		from := theEvm
		to := theUser
		input, err := embeds.Contract_ERC20Minter.ABI.Pack("mint", to, big.NewInt(69_420))
		s.NoError(err)

		_, err = evmtest.DoEthTx(&deps, contract, from, input)
		s.NoError(err)
		evmtest.AssertERC20BalanceEqual(s.T(), &deps, contract, theUser, big.NewInt(69_420))
		evmtest.AssertERC20BalanceEqual(s.T(), &deps, contract, theEvm, big.NewInt(0))
	}

	s.T().Log("Transfer - Success (sanity check)")
	randomAcc := testutil.AccAddress()
	{
		from := theUser
		to := theEvm
		_, err := deps.K.ERC20().Transfer(contract, from, to, big.NewInt(1), deps.Ctx)
		s.NoError(err)
		evmtest.AssertERC20BalanceEqual(s.T(), &deps, contract, theUser, big.NewInt(69_419))
		evmtest.AssertERC20BalanceEqual(s.T(), &deps, contract, theEvm, big.NewInt(1))
		s.Equal("0",
			deps.Chain.BankKeeper.GetBalance(deps.Ctx, randomAcc, funtoken.BankDenom).Amount.String(),
		)
	}

	s.T().Log("Send using precompile")
	amtToSend := int64(419)
	callArgs := precompile.ArgsFunTokenBankSend(contract, big.NewInt(amtToSend), randomAcc)
	methodName := string(precompile.FunTokenMethod_BankSend)
	input, err := abi.Pack(methodName, callArgs...)
	s.NoError(err)

	from := theUser
	_, err = evmtest.DoEthTx(&deps, precompileAddr.ToAddr(), from, input)
	s.Require().NoError(err)

	evmtest.AssertERC20BalanceEqual(s.T(), &deps, contract, theUser, big.NewInt(69_419-amtToSend))
	evmtest.AssertERC20BalanceEqual(s.T(), &deps, contract, theEvm, big.NewInt(1))
	s.Equal(fmt.Sprintf("%d", amtToSend),
		deps.Chain.BankKeeper.GetBalance(deps.Ctx, randomAcc, funtoken.BankDenom).Amount.String(),
	)

	evmtest.AssertERC20BalanceEqual(s.T(), &deps, contract, theUser, big.NewInt(69_000))
	evmtest.AssertERC20BalanceEqual(s.T(), &deps, contract, theEvm, big.NewInt(1))
	s.Equal("419",
		deps.Chain.BankKeeper.GetBalance(deps.Ctx, randomAcc, funtoken.BankDenom).Amount.String(),
	)
}
