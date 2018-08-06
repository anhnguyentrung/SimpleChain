package database

import "blockchain/chain"

type BlockSummaryObject struct {
	BlockNum uint32
	BlockId chain.SHA256Type
}

type Database struct {
	BlockSummaryObjects []*BlockSummaryObject
}

func (db *Database) GetBlockSummaryObject(blockNum uint32) *BlockSummaryObject {
	var foundObject *BlockSummaryObject = nil
	for _, object := range db.BlockSummaryObjects {
		if object.BlockNum == blockNum {
			foundObject = object
			break
		}
	}
	return foundObject
}