package types

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/testutil/sample"
)

func TestMsgAddMargin_ValidateBasic(t *testing.T) {
	type test struct {
		msg         *MsgAddMargin
		expectedErr error
	}

	cases := map[string]test{
		"ok": {
			msg: &MsgAddMargin{
				Sender:    sample.AccAddress().String(),
				TokenPair: "NIBI:NUSD",
				Margin:    sdk.NewInt64Coin("NUSD", 100),
			},
			expectedErr: nil,
		},
		"empty address": {
			msg: &MsgAddMargin{
				Sender:    "",
				TokenPair: "NIBI:NUSD",
				Margin:    sdk.NewInt64Coin("NUSD", 100),
			},
			expectedErr: fmt.Errorf("empty address string is not allowed"),
		},
		"invalid address": {
			msg: &MsgAddMargin{
				Sender:    "foobar",
				TokenPair: "NIBI:NUSD",
				Margin:    sdk.NewInt64Coin("NUSD", 100),
			},
			expectedErr: fmt.Errorf("decoding bech32 failed"),
		},
		"invalid token pair": {
			msg: &MsgAddMargin{
				Sender:    sample.AccAddress().String(),
				TokenPair: "NIBI-NUSD",
				Margin:    sdk.NewInt64Coin("NUSD", 100),
			},
			expectedErr: common.ErrInvalidTokenPair,
		},
		"invalid margin amount": {
			msg: &MsgAddMargin{
				Sender:    sample.AccAddress().String(),
				TokenPair: "NIBI:NUSD",
				Margin:    sdk.NewInt64Coin("NUSD", 0),
			},
			expectedErr: fmt.Errorf("margin must be positive"),
		},
		"invalid margin denom": {
			msg: &MsgAddMargin{
				Sender:    sample.AccAddress().String(),
				TokenPair: "NIBI:NUSD",
				Margin:    sdk.NewInt64Coin("USDC", 100),
			},
			expectedErr: fmt.Errorf("invalid margin denom"),
		},
	}

	for name, tc := range cases {
		tc := tc
		name := name
		t.Run(name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgRemoveMargin_ValidateBasic(t *testing.T) {
	type test struct {
		msg         *MsgRemoveMargin
		expectedErr error
	}

	cases := map[string]test{
		"ok": {
			msg: &MsgRemoveMargin{
				Sender:    sample.AccAddress().String(),
				TokenPair: "NIBI:NUSD",
				Margin:    sdk.NewInt64Coin("NUSD", 100),
			},
			expectedErr: nil,
		},
		"empty address": {
			msg: &MsgRemoveMargin{
				Sender:    "",
				TokenPair: "NIBI:NUSD",
				Margin:    sdk.NewInt64Coin("NUSD", 100),
			},
			expectedErr: fmt.Errorf("empty address string is not allowed"),
		},
		"invalid address": {
			msg: &MsgRemoveMargin{
				Sender:    "foobar",
				TokenPair: "NIBI:NUSD",
				Margin:    sdk.NewInt64Coin("NUSD", 100),
			},
			expectedErr: fmt.Errorf("decoding bech32 failed"),
		},
		"invalid token pair": {
			msg: &MsgRemoveMargin{
				Sender:    sample.AccAddress().String(),
				TokenPair: "NIBI-NUSD",
				Margin:    sdk.NewInt64Coin("NUSD", 100),
			},
			expectedErr: common.ErrInvalidTokenPair,
		},
		"invalid margin amount": {
			msg: &MsgRemoveMargin{
				Sender:    sample.AccAddress().String(),
				TokenPair: "NIBI:NUSD",
				Margin:    sdk.NewInt64Coin("NUSD", 0),
			},
			expectedErr: fmt.Errorf("margin must be positive"),
		},
		"invalid margin denom": {
			msg: &MsgRemoveMargin{
				Sender:    sample.AccAddress().String(),
				TokenPair: "NIBI:NUSD",
				Margin:    sdk.NewInt64Coin("USDC", 100),
			},
			expectedErr: fmt.Errorf("invalid margin denom"),
		},
	}

	for name, tc := range cases {
		tc := tc
		name := name
		t.Run(name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgOpenPosition_ValidateBasic(t *testing.T) {
	type test struct {
		msg     *MsgOpenPosition
		wantErr bool
	}

	cases := map[string]test{
		"ok": {
			msg: &MsgOpenPosition{
				Sender:               sample.AccAddress().String(),
				TokenPair:            "NIBI:NUSD",
				Side:                 Side_BUY,
				QuoteAssetAmount:     sdk.NewInt(100),
				Leverage:             sdk.NewDec(10),
				BaseAssetAmountLimit: sdk.NewInt(100),
			},
			wantErr: false,
		},

		"invalid side": {
			msg: &MsgOpenPosition{
				Sender:               sample.AccAddress().String(),
				TokenPair:            "NIBI:NUSD",
				Side:                 3,
				QuoteAssetAmount:     sdk.NewInt(100),
				Leverage:             sdk.NewDec(10),
				BaseAssetAmountLimit: sdk.NewInt(100),
			},
			wantErr: true,
		},
		"invalid side 2": {
			msg: &MsgOpenPosition{
				Sender:               sample.AccAddress().String(),
				TokenPair:            "NIBI:NUSD",
				Side:                 Side_SIDE_UNSPECIFIED,
				QuoteAssetAmount:     sdk.NewInt(100),
				Leverage:             sdk.NewDec(10),
				BaseAssetAmountLimit: sdk.NewInt(100),
			},
			wantErr: true,
		},
		"invalid address": {
			msg: &MsgOpenPosition{
				Sender:               "",
				TokenPair:            "NIBI:NUSD",
				Side:                 Side_SELL,
				QuoteAssetAmount:     sdk.NewInt(100),
				Leverage:             sdk.NewDec(10),
				BaseAssetAmountLimit: sdk.NewInt(100),
			},
			wantErr: true,
		},
		"invalid leverage": {
			msg: &MsgOpenPosition{
				Sender:               sample.AccAddress().String(),
				TokenPair:            "NIBI:NUSD",
				Side:                 Side_BUY,
				QuoteAssetAmount:     sdk.NewInt(100),
				Leverage:             sdk.ZeroDec(),
				BaseAssetAmountLimit: sdk.NewInt(100),
			},
			wantErr: true,
		},
		"invalid quote asset amount": {
			msg: &MsgOpenPosition{
				Sender:               sample.AccAddress().String(),
				TokenPair:            "NIBI:NUSD",
				Side:                 Side_BUY,
				QuoteAssetAmount:     sdk.NewInt(0),
				Leverage:             sdk.NewDec(10),
				BaseAssetAmountLimit: sdk.NewInt(100),
			},
			wantErr: true,
		},
		"invalid token pair": {
			msg: &MsgOpenPosition{
				Sender:               sample.AccAddress().String(),
				TokenPair:            "NIBI-NUSD",
				Side:                 Side_BUY,
				QuoteAssetAmount:     sdk.NewInt(0),
				Leverage:             sdk.NewDec(10),
				BaseAssetAmountLimit: sdk.NewInt(100),
			},
			wantErr: true,
		},
		"invalid base asset amount limit": {
			msg: &MsgOpenPosition{
				Sender:               sample.AccAddress().String(),
				TokenPair:            "NIBI:NUSD",
				Side:                 Side_BUY,
				QuoteAssetAmount:     sdk.NewInt(0),
				Leverage:             sdk.NewDec(10),
				BaseAssetAmountLimit: sdk.ZeroInt(),
			},
			wantErr: true,
		},
	}

	for name, tc := range cases {
		tc := tc
		name := name
		t.Run(name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if err != nil && tc.wantErr == false {
				t.Fatalf("unexpected error: %s", err)
			}
			if err == nil && tc.wantErr == true {
				t.Fatalf("expected error: %s", err)
			}
		})
	}
}

func TestMsgLiquidate_ValidateBasic(t *testing.T) {
	type test struct {
		msg     *MsgLiquidate
		wantErr bool
	}

	cases := map[string]test{
		"ok": {
			msg: &MsgLiquidate{
				Sender:    sample.AccAddress().String(),
				TokenPair: "NIBI:NUSD",
				Trader:    sample.AccAddress().String(),
			},
			wantErr: false,
		},
		"invalid pair": {
			msg: &MsgLiquidate{
				Sender:    sample.AccAddress().String(),
				TokenPair: "xxx:yyy:zzz",
				Trader:    sample.AccAddress().String(),
			},
			wantErr: true,
		},
		"invalid trader": {
			msg: &MsgLiquidate{
				Sender:    sample.AccAddress().String(),
				TokenPair: "NIBI:NUSD",
				Trader:    "",
			},
			wantErr: true,
		},
		"invalid liquidator": {
			msg: &MsgLiquidate{
				Sender:    "",
				TokenPair: "NIBI:NUSD",
				Trader:    sample.AccAddress().String(),
			},
			wantErr: true,
		},
	}

	for name, tc := range cases {
		tc := tc
		name := name
		t.Run(name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if err != nil && tc.wantErr == false {
				t.Fatalf("unexpected error: %s", err)
			}
			if err == nil && tc.wantErr == true {
				t.Fatalf("expected error: %s", err)
			}
		})
	}
}
