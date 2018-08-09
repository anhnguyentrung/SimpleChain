package chain

import (
	"crypto/sha256"
	"time"
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
	InitialTimestamp BlockTimeStamp
	InitialKey string
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
	initialKey := DEFAULT_PUBLIC_KEY
	initialTime := time.Date(2018, time.August, 8, 0, 0, 0, 0, time.UTC).UnixNano() / int64(time.Millisecond)
	initialTs := NewBlockTimeStamp()
	initialTs.SetTime(uint64(initialTime))
	return GenesisState{
		InitialConfiguration:initialConfig,
		InitialTimestamp: initialTs,
		InitialKey: initialKey,
	}
}

func (gs GenesisState) ComputeChainId() SHA256Type {
	buf, _ := MarshalBinary(gs)
	return sha256.Sum256(buf)
}
