package cli

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/testutil/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/types"

	"github.com/NibiruChain/nibiru/x/common"
)

type ExecTxOption func(*execTxOptions)

func WithTxFees(feeCoins sdk.Coins) ExecTxOption {
	return func(options *execTxOptions) {
		options.fees = feeCoins
	}
}

func WithTxSkipConfirmation(skipConfirmation bool) ExecTxOption {
	return func(options *execTxOptions) {
		options.skipConfirmation = skipConfirmation
	}
}

func WithTxBroadcastMode(broadcastMode string) ExecTxOption {
	return func(options *execTxOptions) {
		options.broadcastMode = broadcastMode
	}
}

// WithTxCanFail will not make ExecTx return an error
// in case the response code of the TX is not ok.
func WithTxCanFail(canFail bool) ExecTxOption {
	return func(options *execTxOptions) {
		options.canFail = canFail
	}
}

func WithKeyringBackend(keyringBackend string) ExecTxOption {
	return func(options *execTxOptions) {
		options.keyringBackend = keyringBackend
	}
}

type execTxOptions struct {
	fees             sdk.Coins
	skipConfirmation bool
	broadcastMode    string
	canFail          bool
	keyringBackend   string
}

func ExecTx(network *Network, cmd *cobra.Command, txSender sdk.AccAddress, args []string, opt ...ExecTxOption) (*sdk.TxResponse, error) {
	if len(network.Validators) == 0 {
		return nil, fmt.Errorf("invalid network")
	}

	args = append(args, fmt.Sprintf("--%s=%s", flags.FlagFrom, txSender))

	options := execTxOptions{
		fees:             sdk.NewCoins(sdk.NewCoin(common.DenomNIBI, sdk.NewInt(10))),
		skipConfirmation: true,
		broadcastMode:    flags.BroadcastBlock,
		canFail:          false,
		keyringBackend:   keyring.BackendTest,
	}

	for _, o := range opt {
		o(&options)
	}

	args = append(args, fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, options.broadcastMode))
	args = append(args, fmt.Sprintf("--%s=%s", flags.FlagFees, options.fees))
	args = append(args, fmt.Sprintf("--%s=%s", flags.FlagKeyringBackend, options.keyringBackend))
	switch options.skipConfirmation {
	case true:
		args = append(args, fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation))
	case false:
		args = append(args, fmt.Sprintf("--%s=false", flags.FlagSkipConfirmation))
	}

	clientCtx := network.Validators[0].ClientCtx

	rawResp, err := cli.ExecTestCLICmd(clientCtx, cmd, args)
	if err != nil {
		return nil, err
	}

	resp := new(sdk.TxResponse)
	err = clientCtx.Codec.UnmarshalJSON(rawResp.Bytes(), resp)
	if err != nil {
		return nil, err
	}

	if options.canFail {
		return resp, nil
	}

	if resp.Code != types.CodeTypeOK {
		return nil, fmt.Errorf("tx failed with code %d: %s", resp.Code, resp.RawLog)
	}

	return resp, nil
}

func (n *Network) SendTx(addr sdk.AccAddress, msgs ...sdk.Msg) (*sdk.TxResponse, error) {
	cfg := n.Config
	kb, info := n.keyBaseAndInfoForAddr(addr)
	rpc := n.Validators[0].RPCClient
	txBuilder := cfg.TxConfig.NewTxBuilder()
	require.NoError(n.T, txBuilder.SetMsgs(msgs...))
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(cfg.BondDenom, sdk.NewInt(1))))
	txBuilder.SetGasLimit(1000000)

	acc, err := cfg.AccountRetriever.GetAccount(n.Validators[0].ClientCtx, addr)
	require.NoError(n.T, err)

	txFactory := tx.Factory{}
	txFactory = txFactory.
		WithChainID(cfg.ChainID).
		WithKeybase(kb).
		WithTxConfig(cfg.TxConfig).
		WithAccountRetriever(cfg.AccountRetriever).
		WithAccountNumber(acc.GetAccountNumber()).
		WithSequence(acc.GetSequence())

	err = tx.Sign(txFactory, info.GetName(), txBuilder, true)
	require.NoError(n.T, err)

	txBytes, err := cfg.TxConfig.TxEncoder()(txBuilder.GetTx())
	require.NoError(n.T, err)

	respRaw, err := rpc.BroadcastTxCommit(context.Background(), txBytes)
	require.NoError(n.T, err)

	require.Truef(n.T, respRaw.CheckTx.IsOK(), "tx failed: %s", respRaw.CheckTx.Log)
	require.Truef(n.T, respRaw.DeliverTx.IsOK(), "tx failed: %s", respRaw.DeliverTx.Log)

	return sdk.NewResponseFormatBroadcastTxCommit(respRaw), nil
}
