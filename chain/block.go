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
	return NumFromId(bh.Previous) + 1
}

func (bh *BlockHeader) Id() SHA256Type {
	bhEncoded, _ := MarshalBinary(*bh)
	h := sha256.Sum256(bhEncoded)
	binary.BigEndian.PutUint32(h[:], bh.BlockNum())
	return h
}

func NumFromId(id SHA256Type) uint32 {
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

type HeaderConfirmation struct {
	BlockId SHA256Type
	Producer AccountName
	ProducerSignature crypto.Signature
}

type BlockHeaderState struct {
	Id SHA256Type
	BlockNum uint32
	Header SignedBlockHeader
	DPOSProposedIrreversibleBlockNum uint32
	DPOSIrreversibleBlockNum uint32
	BFTIrreversibleBlockNum uint32
	PendingScheduleLibNum uint32
	PendingScheduleHash SHA256Type
	PendingSchedule ProducerScheduleType
	ActiveSchedule ProducerScheduleType
	ProducerToLastProduced map[AccountName]uint32
	ProducerToLastImpliedIRB map[AccountName]uint32
	BlockSigningKey crypto.PublicKey
	ConfirmCount []uint8
	Confirmations []HeaderConfirmation
}

type BlockState struct {
	BlockHeaderState
	Block *SignedBlock
	Validated bool
	InCurrentChain bool
	Trxs []*TransactionMetaData
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