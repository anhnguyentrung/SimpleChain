package database

import "blockchain/chain"

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
