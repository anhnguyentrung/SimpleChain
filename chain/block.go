package chain

import (
	"time"
	"encoding/binary"
	"crypto/sha256"
	"blockchain/crypto"
)

type BlockHeader struct {
	Timestamp time.Time
	Producer AccountName
	Confirmed uint16
	Previous SHA256Type
	TransactionMRoot SHA256Type
	ActionMRoot SHA256Type
	ScheducerVersion uint32
	NewProducer ProducerScheduleType
	HeaderExtensions []Extension
}

func (bh *BlockHeader) BlockNum() uint32 {
	return numFromId(bh.Previous) + 1
}

func (bh *BlockHeader) Id() SHA256Type {
	bhEncoded, _ := MarshalBinary(*bh)
	h := sha256.Sum256(bhEncoded)
	binary.BigEndian.PutUint32(h[:], bh.BlockNum())
	return h
}

func numFromId(id SHA256Type) uint32 {
	return binary.BigEndian.Uint32(id[:4])
}

type SignedBlockHeader struct {
	BlockHeader
	ProducerSignature crypto.Signature
}

type SignedBlock struct {
	SignedBlockHeader
	Transactions []TransactionReceipt
	BlockExtensions []Extension
}

type TransactionReceiptHeader struct {
	Status TransactionStatus
	CPUUsageMs uint32
	NetUsageWords Varuint32
}

type TransactionReceipt struct {
	TransactionReceiptHeader
	Id SHA256Type
	PackedTrx PackedTransaction
}