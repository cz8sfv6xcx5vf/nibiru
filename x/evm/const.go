// Copyright (c) 2023-2024 Nibi, Inc.
package evm

import (
	"github.com/NibiruChain/collections"
	"github.com/ethereum/go-ethereum/common"
)

const (
	// ModuleName string name of module
	ModuleName = "evm"

	// StoreKey: Persistent storage key for ethereum storage data, account code
	// (StateDB) or block related data for the Eth Web3 API.
	StoreKey = ModuleName

	// TransientKey is the key to access the EVM transient store, that is reset
	// during the Commit phase.
	TransientKey = "transient_" + ModuleName

	// RouterKey uses module name for routing
	RouterKey = ModuleName
)

// prefix bytes for the EVM persistent store
const (
	KeyPrefixAccCodes collections.Namespace = iota + 1
	KeyPrefixAccState
	KeyPrefixParams
	KeyPrefixEthAddrIndex
)

// KVStore transient prefix namespaces for the EVM Module. Transient stores only
// remain for current block, and have more gas efficient read and write access.
const (
	NamespaceBlockBloom collections.Namespace = iota + 1
	NamespaceBlockTxIndex
	NamespaceBlockLogSize
	NamespaceBlockGasUsed
)

var KeyPrefixBzAccState = KeyPrefixAccState.Prefix()

// PrefixAccStateEthAddr returns a prefix to iterate over a given account storage.
func PrefixAccStateEthAddr(address common.Address) []byte {
	return append(KeyPrefixBzAccState, address.Bytes()...)
}

// StateKey defines the full key under which an account state is stored.
func StateKey(address common.Address, key []byte) []byte {
	return append(PrefixAccStateEthAddr(address), key...)
}

const (
	// Amino names
	updateParamsName = "evm/MsgUpdateParams"
)

type CallType int

const (
	// CallTypeRPC call type is used on requests to eth_estimateGas rpc API endpoint
	CallTypeRPC CallType = iota + 1
	// CallTypeSmart call type is used in case of smart contract methods calls
	CallTypeSmart
)
