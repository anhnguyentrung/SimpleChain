package chain

import (
	"time"
	"crypto/sha256"
)

type ChainConfig struct {
	MaxTransactionLifetime uint32
	DeferedTrxExpirationWindow uint32
	MaxTransactionDelay uint32
	MaxInlineActionSize uint32
	MaxInlineActionDepth uint32
	MaxAuthorityDepth uint32
}

type GenesisState struct {
	RootKey string
	InitialConfiguration ChainConfig
	InitialTimestamp time.Time
	InitialKey SHA256Type
}

func NewGenesisState() GenesisState {
	initialConfig := ChainConfig{
		MaxTransactionLifetime:DEFAULT_MAX_TRX_LIFETIME,
		DeferedTrxExpirationWindow:DEFAULT_DEFERRED_TRX_EXPIRATION_WINDOW,
		MaxTransactionDelay:DEFAULT_MAX_TRX_DELAY,
		MaxInlineActionSize:DEFAULT_MAX_INLINE_ACTION_SIZE,
		MaxInlineActionDepth:DEFAULT_MAX_INLINE_ACTION_DEPTH,
		MaxAuthorityDepth:DEFAULT_MAX_AUTH_DEPTH,
	}
	return GenesisState{
		InitialConfiguration:initialConfig,
	}
}

func (gs GenesisState) ComputeChainId() SHA256Type {
	buf, _ := MarshalBinary(gs)
	return sha256.Sum256(buf)
}
