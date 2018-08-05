package database

import "blockchain/chain"

type BlockLog struct {
	Head *chain.SignedBlock
	HeadId chain.SHA256Type
	Blocks []*chain.SignedBlock
	GenesisWrittenToBlockLog bool
}

func (bl BlockLog) ReadBlockByNum(blockNum uint32) *chain.SignedBlock {
	if bl.Head != nil && blockNum <= chain.NumFromId(bl.HeadId) && blockNum > 0 {
		return bl.Blocks[blockNum-1]
	}
	return nil
}