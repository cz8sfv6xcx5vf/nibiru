package types

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/NibiruChain/nibiru/x/common"
	"github.com/NibiruChain/nibiru/x/testutil/sample"
)

func TestPosition_Validate(t *testing.T) {
	type test struct {
		p       *Position
		wantErr bool
	}

	cases := map[string]test{
		"success": {
			p: &Position{
				TraderAddress:                   sample.AccAddress().String(),
				Pair:                            common.MustNewAssetPair("valid:pair"),
				Size_:                           sdk.MustNewDecFromStr("1000"),
				Margin:                          sdk.MustNewDecFromStr("1000"),
				OpenNotional:                    sdk.MustNewDecFromStr("1000"),
				LatestCumulativePremiumFraction: sdk.MustNewDecFromStr("1"),
				BlockNumber:                     0,
			},
			wantErr: false,
		},
		"bad trader address": {
			p:       &Position{TraderAddress: "invalid"},
			wantErr: true,
		},

		"bad pair": {
			p: &Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.AssetPair{},
			},
			wantErr: true,
		},

		"bad size": {
			p: &Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.MustNewAssetPair("valid:pair"),
				Size_:         sdk.ZeroDec(),
			},
			wantErr: true,
		},

		"bad margin": {
			p: &Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.MustNewAssetPair("valid:pair"),
				Size_:         sdk.MustNewDecFromStr("1000"),
				Margin:        sdk.MustNewDecFromStr("-1000"),
			},
			wantErr: true,
		},
		"bad open notional": {
			p: &Position{
				TraderAddress: sample.AccAddress().String(),
				Pair:          common.MustNewAssetPair("valid:pair"),
				Size_:         sdk.MustNewDecFromStr("1000"),
				Margin:        sdk.MustNewDecFromStr("1000"),
				OpenNotional:  sdk.MustNewDecFromStr("-1000"),
			},
			wantErr: true,
		},

		"bad block number": {
			p: &Position{
				TraderAddress:                   sample.AccAddress().String(),
				Pair:                            common.MustNewAssetPair("valid:pair"),
				Size_:                           sdk.MustNewDecFromStr("1000"),
				Margin:                          sdk.MustNewDecFromStr("1000"),
				OpenNotional:                    sdk.MustNewDecFromStr("1000"),
				LatestCumulativePremiumFraction: sdk.MustNewDecFromStr("1"),
				BlockNumber:                     -1,
			},
			wantErr: true,
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			err := tc.p.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected an error")
			} else if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		})
	}
}

func TestPairMetadata_Validate(t *testing.T) {
	type test struct {
		p       *PairMetadata
		wantErr bool
	}

	cases := map[string]test{
		"success": {
			p: &PairMetadata{
				Pair:                       common.MustNewAssetPair("pair1:pair2"),
				CumulativePremiumFractions: []sdk.Dec{sdk.MustNewDecFromStr("0.1")},
			},
		},

		"invalid pair": {
			p:       &PairMetadata{},
			wantErr: true,
		},

		"invalid cumulative funding rate": {
			p: &PairMetadata{
				Pair:                       common.MustNewAssetPair("pair1:pair2"),
				CumulativePremiumFractions: []sdk.Dec{{}},
			},
			wantErr: true,
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			err := tc.p.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected an error")
			} else if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		})
	}
}

func BenchmarkPosition_Validate(b *testing.B) {
	t := &Position{
		TraderAddress:                   sample.AccAddress().String(),
		Pair:                            common.MustNewAssetPair("valid:pair"),
		Size_:                           sdk.MustNewDecFromStr("1000"),
		Margin:                          sdk.MustNewDecFromStr("1000"),
		OpenNotional:                    sdk.MustNewDecFromStr("1000"),
		LatestCumulativePremiumFraction: sdk.MustNewDecFromStr("1"),
		BlockNumber:                     0,
	}

	for i := 0; i < b.N; i++ {
		err := t.Validate()
		_ = err
	}
}
