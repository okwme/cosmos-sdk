// +build !test_amino

package main

import (
	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
)

func MakeTxCLIContext() context.CLIContext {
	cliCtx := context.CLIContext{}
	protoCdc := codec.NewProtoCodec(encodingConfig.InterfaceRegistry)
	return cliCtx.
		WithJSONMarshaler(protoCdc).
		WithTxGenerator(encodingConfig.TxGenerator).
		WithTxJSONDecoder(encodingConfig.TxJSONDecoder).
		WithAccountRetriever(types.NewAccountRetriever(encodingConfig.Marshaler)).
		WithCodec(encodingConfig.Amino)
}
