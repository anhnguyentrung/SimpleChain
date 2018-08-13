package database

import (
	"blockchain/chain"
	"bytes"
	"log"
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

func (fb *ForkDatabase) Add(bs *chain.BlockState) *chain.BlockState {
	fb.BlockStates = append(fb.BlockStates, bs)
	fb.Head = fb.BlockStates[len(fb.BlockStates) - 1]
	return bs
}

func (fb *ForkDatabase) AddSignedBlock(signedBlock *chain.SignedBlock, trust bool) *chain.BlockState {
	if signedBlock == nil {
		log.Fatal("block must not be nil")
	}
	if fb.Head == nil {
		log.Fatal("no head block")
	}
	if fb.GetBlock(signedBlock.Id()) != nil {
		log.Fatal("this block existed in database")
	}
	previousBlock := fb.GetBlock(signedBlock.Previous)
	if previousBlock == nil {
		log.Fatal("unlinkable block")
	}
	blockState := chain.NewBlockStateFromSignedBlock(&previousBlock.BlockHeaderState, signedBlock, trust)
	return fb.Add(blockState)
}
