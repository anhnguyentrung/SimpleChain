package database

import (
	"blockchain/chain"
	"bytes"
)

type ForkDatabase struct {
	Head *chain.BlockState
	BlockStates []*chain.BlockState
}

func (fb *ForkDatabase) GetBlockInCurrentChainIdNum(n uint32) *chain.BlockState {
	for _, bs := range fb.BlockStates {
		if bs.BlockNum == n {
			return bs
		}
	}
	return nil
}

func (fb *ForkDatabase) GetBlock(id chain.SHA256Type) *chain.BlockState {
	for _, bs := range fb.BlockStates {
		if bytes.Equal(bs.Id[:], id[:]) {
			return bs
		}
	}
	return nil
}
