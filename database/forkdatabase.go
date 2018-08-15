package database

import (
	"blockchain/chain"
	"bytes"
	"log"
	"fmt"
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

func (fb *ForkDatabase) AddSignedBlock(signedBlock *chain.SignedBlock, trust bool) (*chain.BlockState, error) {
	fmt.Println("add signed block ", signedBlock.BlockNum())
	if signedBlock == nil {
		fmt.Println("block must not be nil")
		return nil, fmt.Errorf("block must not be nil")
	}
	if fb.Head == nil {
		fmt.Println("no head block")
		return nil, fmt.Errorf("no head block")
	}
	if fb.GetBlock(signedBlock.Id()) != nil {
		fmt.Println("this block existed in database")
		return nil, fmt.Errorf("this block existed in database")
	}
	previousBlock := fb.GetBlock(signedBlock.Previous)
	if previousBlock == nil {
		fmt.Println("unlinkable block ", signedBlock.BlockNum())
		return nil, fmt.Errorf("unlinkable block")
	}
	blockState := chain.NewBlockStateFromSignedBlock(&previousBlock.BlockHeaderState, signedBlock, trust)
	return fb.Add(blockState), nil
}

func (fb *ForkDatabase) MarkInCurrentChain(blockState *chain.BlockState, inCurrentChain bool) {
	if blockState.InCurrentChain == inCurrentChain {
		return
	}
	bs := fb.GetBlock(blockState.Id)
	if bs == nil {
		log.Fatal("can not find block in fork database")
	}
	bs.InCurrentChain = inCurrentChain
}

func (fb *ForkDatabase) SetValidity(blockState *chain.BlockState, valid bool) {
	if !valid {
		fb.remove(blockState.Id)
	} else {
		blockState.Validated = true
	}
}

func (fb *ForkDatabase) remove(id chain.SHA256Type) {
	for index, bs := range fb.BlockStates {
		if bytes.Equal(bs.Id[:], id[:]) {
			fb.BlockStates = append(fb.BlockStates[:index], fb.BlockStates[index+1:]...)
			return
		}
	}
}

func (fb *ForkDatabase) FectchBranchFrom(first chain.SHA256Type, second chain.SHA256Type) chain.Pair {
	result := chain.Pair{
		[]*chain.BlockState{},
		[]*chain.BlockState{},
	}
	firstBs := fb.GetBlock(first)
	secondBs := fb.GetBlock(second)
	for firstBs.BlockNum > secondBs.BlockNum {
		result.First = append(result.First.([]*chain.BlockState), firstBs)
		firstBs = fb.GetBlock(firstBs.Header.Previous)
		if firstBs == nil {
			log.Fatal("can not find previous block")
		}
	}
	for secondBs.BlockNum > firstBs.BlockNum {
		result.Second = append(result.Second.([]*chain.BlockState), secondBs)
		secondBs = fb.GetBlock(secondBs.Header.Previous)
		if secondBs == nil {
			log.Fatal("can not find previous block")
		}
	}
	for firstBs.Header.Previous != secondBs.Header.Previous {
		result.First = append(result.First.([]*chain.BlockState), firstBs)
		result.Second = append(result.Second.([]*chain.BlockState), secondBs)
		firstBs = fb.GetBlock(firstBs.Header.Previous)
		secondBs = fb.GetBlock(secondBs.Header.Previous)
		if firstBs == nil || secondBs == nil {
			log.Fatal("can not find previous block")
		}
	}

	if firstBs != nil && secondBs != nil {
		result.First = append(result.First.([]*chain.BlockState), firstBs)
		result.Second = append(result.Second.([]*chain.BlockState), secondBs)
	}
	return result
}
