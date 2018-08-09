package chain

import (
	"crypto/sha256"
	"time"
)

type ChainConfig struct {
}

type GenesisState struct {
	RootKey string
	InitialConfiguration ChainConfig
	InitialTimestamp BlockTimeStamp
	InitialKey string
}

func NewGenesisState() GenesisState {
	initialConfig := ChainConfig{}
	initialKey := DEFAULT_PUBLIC_KEY
	initialTime := time.Date(2018, time.August, 8, 0, 0, 0, 0, time.UTC).UnixNano()
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
