// Copyright (c) 2023-2024 Nibi, Inc.
package keeper

import (
	"context"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	gethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/evm"
)

// Compile-time interface assertion
var _ evm.QueryServer = Keeper{}

// EthAccount: Implements the gRPC query for "/eth.evm.v1.Query/EthAccount".
// EthAccount retrieves the account details for a given Ethereum hex address.
//
// Parameters:
//   - goCtx: The context.Context object representing the request context.
//   - req: Request containing the Ethereum hexadecimal address.
//
// Returns:
//   - A pointer to the QueryEthAccountResponse object containing the account details.
//   - An error if the account retrieval process encounters any issues.
func (k Keeper) EthAccount(
	goCtx context.Context, req *evm.QueryEthAccountRequest,
) (*evm.QueryEthAccountResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	addr := gethcommon.HexToAddress(req.Address)
	ctx := sdk.UnwrapSDKContext(goCtx)
	acct := k.GetAccountOrEmpty(ctx, addr)

	return &evm.QueryEthAccountResponse{
		Balance:  acct.Balance.String(),
		CodeHash: gethcommon.BytesToHash(acct.CodeHash).Hex(),
		Nonce:    acct.Nonce,
	}, nil
}

// NibiruAccount: Implements the gRPC query for "/eth.evm.v1.Query/NibiruAccount".
// NibiruAccount retrieves the Cosmos account details for a given Ethereum address.
//
// Parameters:
//   - goCtx: The context.Context object representing the request context.
//   - req: The QueryNibiruAccountRequest object containing the Ethereum address.
//
// Returns:
//   - A pointer to the QueryNibiruAccountResponse object containing the Cosmos account details.
//   - An error if the account retrieval process encounters any issues.
func (k Keeper) NibiruAccount(
	goCtx context.Context, req *evm.QueryNibiruAccountRequest,
) (resp *evm.QueryNibiruAccountResponse, err error) {
	if err := req.Validate(); err != nil {
		return resp, err
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	ethAddr := gethcommon.HexToAddress(req.Address)
	nibiruAddr := sdk.AccAddress(ethAddr.Bytes())

	accountOrNil := k.accountKeeper.GetAccount(ctx, nibiruAddr)
	resp = &evm.QueryNibiruAccountResponse{
		Address: nibiruAddr.String(),
	}

	if accountOrNil != nil {
		resp.Sequence = accountOrNil.GetSequence()
		resp.AccountNumber = accountOrNil.GetAccountNumber()
	}

	return resp, nil
}

// ValidatorAccount: Implements the gRPC query for "/eth.evm.v1.Query/ValidatorAccount".
// ValidatorAccount retrieves the account details for a given validator consensus address.
//
// Parameters:
//   - goCtx: The context.Context object representing the request context.
//   - req: The QueryValidatorAccountRequest object containing the validator consensus address.
//
// Returns:
//   - A pointer to the QueryValidatorAccountResponse object containing the account details.
//   - An error if the account retrieval process encounters any issues.
func (k Keeper) ValidatorAccount(
	goCtx context.Context, req *evm.QueryValidatorAccountRequest,
) (*evm.QueryValidatorAccountResponse, error) {
	// TODO: feat(evm): impl query ValidatorAccount
	return &evm.QueryValidatorAccountResponse{
		AccountAddress: "",
		Sequence:       0,
		AccountNumber:  0,
	}, common.ErrNotImplementedGprc()
}

// Balance: Implements the gRPC query for "/eth.evm.v1.Query/Balance".
// Balance retrieves the balance of an Ethereum address in "Ether", which
// actually refers to NIBI tokens on Nibiru EVM.
//
// Parameters:
//   - goCtx: The context.Context object representing the request context.
//   - req: The QueryBalanceRequest object containing the Ethereum address.
//
// Returns:
//   - A pointer to the QueryBalanceResponse object containing the balance.
//   - An error if the balance retrieval process encounters any issues.
func (k Keeper) Balance(goCtx context.Context, req *evm.QueryBalanceRequest) (*evm.QueryBalanceResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	balanceInt := k.GetEvmGasBalance(ctx, gethcommon.HexToAddress(req.Address))
	return &evm.QueryBalanceResponse{
		Balance: balanceInt.String(),
	}, nil
}

// BaseFee implements the Query/BaseFee gRPC method
func (k Keeper) BaseFee(
	goCtx context.Context, _ *evm.QueryBaseFeeRequest,
) (*evm.QueryBaseFeeResponse, error) {
	// TODO: feat(evm): impl query BaseFee
	return &evm.QueryBaseFeeResponse{
		BaseFee: &sdkmath.Int{},
	}, common.ErrNotImplementedGprc()
}

// Storage: Implements the gRPC query for "/eth.evm.v1.Query/Storage".
// Storage retrieves the storage value for a given Ethereum address and key.
//
// Parameters:
//   - goCtx: The context.Context object representing the request context.
//   - req: The QueryStorageRequest object containing the Ethereum address and key.
//
// Returns:
//   - A pointer to the QueryStorageResponse object containing the storage value.
//   - An error if the storage retrieval process encounters any issues.
func (k Keeper) Storage(
	goCtx context.Context, req *evm.QueryStorageRequest,
) (*evm.QueryStorageResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	address := gethcommon.HexToAddress(req.Address)
	key := gethcommon.HexToHash(req.Key)

	state := k.GetState(ctx, address, key)
	stateHex := state.Hex()

	return &evm.QueryStorageResponse{
		Value: stateHex,
	}, nil
}

// Code: Implements the gRPC query for "/eth.evm.v1.Query/Code".
// Code retrieves the smart contract bytecode associated with a given Ethereum
// address.
//
// Parameters:
//   - goCtx: The context.Context object representing the request context.
//   - req: The QueryCodeRequest object containing the Ethereum address.
//
// Returns:
//   - A pointer to the QueryCodeResponse object containing the code.
//   - An error if the code retrieval process encounters any issues.
func (k Keeper) Code(goCtx context.Context, req *evm.QueryCodeRequest) (*evm.QueryCodeResponse, error) {
	// TODO: feat(evm): impl query Code
	return &evm.QueryCodeResponse{
		Code: []byte{},
	}, common.ErrNotImplementedGprc()
}

// Params: Implements the gRPC query for "/eth.evm.v1.Query/Params".
// Params retrieves the EVM module parameters.
//
// Parameters:
//   - goCtx: The context.Context object representing the request context.
//   - req: The QueryParamsRequest object (unused).
//
// Returns:
//   - A pointer to the QueryParamsResponse object containing the EVM module parameters.
//   - An error if the parameter retrieval process encounters any issues.
func (k Keeper) Params(goCtx context.Context, _ *evm.QueryParamsRequest) (*evm.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(ctx)
	return &evm.QueryParamsResponse{
		Params: params,
	}, nil
}

// EthCall: Implements the gRPC query for "/eth.evm.v1.Query/EthCall".
// EthCall performs a smart contract call using the eth_call JSON-RPC method.
//
// Parameters:
//   - goCtx: The context.Context object representing the request context.
//   - req: The EthCallRequest object containing the call parameters.
//
// Returns:
//   - A pointer to the MsgEthereumTxResponse object containing the result of the eth_call.
//   - An error if the eth_call process encounters any issues.
func (k Keeper) EthCall(
	goCtx context.Context, req *evm.EthCallRequest,
) (*evm.MsgEthereumTxResponse, error) {
	// TODO: feat(evm): impl query EthCall
	return &evm.MsgEthereumTxResponse{
		Hash:    "",
		Logs:    []*evm.Log{},
		Ret:     []byte{},
		VmError: "",
		GasUsed: 0,
	}, common.ErrNotImplementedGprc()
}

// EstimateGas: Implements the gRPC query for "/eth.evm.v1.Query/EstimateGas".
// EstimateGas implements eth_estimateGas rpc api.
func (k Keeper) EstimateGas(
	goCtx context.Context, req *evm.EthCallRequest,
) (*evm.EstimateGasResponse, error) {
	// TODO: feat(evm): impl query EstimateGas
	return k.EstimateGasForEvmCallType(goCtx, req, evm.CallTypeRPC)
}

// EstimateGasForEvmCallType estimates the gas cost of a transaction. This can be
// called with the "eth_estimateGas" JSON-RPC method or an smart contract query.
//
// When [EstimateGas] is called from the JSON-RPC client, we need to reset the
// gas meter before simulating the transaction (tx) to have an accurate gas
// estimate txs using EVM extensions.
//
// Parameters:
//   - goCtx: The context.Context object representing the request context.
//   - req: The EthCallRequest object containing the transaction parameters.
//
// Returns:
//   - A response containing the estimated gas cost.
//   - An error if the gas estimation process encounters any issues.
func (k Keeper) EstimateGasForEvmCallType(
	goCtx context.Context, req *evm.EthCallRequest, fromType evm.CallType,
) (*evm.EstimateGasResponse, error) {
	// TODO: feat(evm): impl query EstimateGasForEvmCallType
	return &evm.EstimateGasResponse{
		Gas: 0,
	}, common.ErrNotImplementedGprc()
}

// TraceTx configures a new tracer according to the provided configuration, and
// executes the given message in the provided environment. The return value will
// be tracer dependent.
func (k Keeper) TraceTx(
	goCtx context.Context, req *evm.QueryTraceTxRequest,
) (*evm.QueryTraceTxResponse, error) {
	// TODO: feat(evm): impl query TraceTx
	return &evm.QueryTraceTxResponse{
		Data: []byte{},
	}, common.ErrNotImplementedGprc()
}

// TraceBlock: Implements the gRPC query for "/eth.evm.v1.Query/TraceBlock".
// Configures a Nibiru EVM tracer that is used to "trace" and analyze
// the execution of transactions within a given block. Block information is read
// from the context (goCtx). [TraceBlock] is responsible iterates over each Eth
// transacion message and calls [TraceEthTxMsg] on it.
func (k Keeper) TraceBlock(
	goCtx context.Context, req *evm.QueryTraceBlockRequest,
) (*evm.QueryTraceBlockResponse, error) {
	// TODO: feat(evm): impl query TraceBlock
	return &evm.QueryTraceBlockResponse{
		Data: []byte{},
	}, common.ErrNotImplementedGprc()
}
