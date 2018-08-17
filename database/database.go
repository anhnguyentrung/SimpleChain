package database

import (
	"blockchain/chain"
	"time"
	"bytes"
)

type BlockSummaryObject struct {
	BlockNum uint32
	BlockId chain.SHA256Type
}

type TransactionObject struct {
	TransactionId chain.SHA256Type
	Expiration time.Time
}

type GlobalPropertyBlock struct {
	ProposedScheduleBlockNum *uint32
	ProposedSchedule *chain.ProducerScheduleType
	Configuation chain.ChainConfig
}

type Database struct {
	BlockSummaryObjects []*BlockSummaryObject
	TransactionObjects []*TransactionObject
	GPO *GlobalPropertyBlock
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

func (db *Database) FindTransactionObject(id chain.SHA256Type) *TransactionObject {
	var trx *TransactionObject = nil
	for _, obj := range db.TransactionObjects {
		if bytes.Equal(trx.TransactionId[:], obj.TransactionId[:]) {
			trx = obj
			break
		}
	}
	return trx
}

func (db *Database) FindBlockSummaryObject(blockNum uint32) *BlockSummaryObject {
	var bso *BlockSummaryObject = &BlockSummaryObject{BlockNum:blockNum}
	for _, obj := range db.BlockSummaryObjects {
		if obj.BlockNum == blockNum {
			bso = obj
			break
		}
	}
	return bso
}