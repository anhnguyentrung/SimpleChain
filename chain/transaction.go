package chain

import (
	"time"
	"blockchain/crypto"
)

type TransactionHeader struct{
	Expiration time.Time
	RefBlockNum uint16
	RefBlockPrefix uint16
	MaxNetUsageWords uint32 // 1 word = 8 bytes
	MaxCPUUsageMs uint8
	DelaySec uint32
}

type Transaction struct {
	TransactionHeader
	ContextFreeActions []Action
	Actions []Action
	TransactionExtensions []Extension
}

type SignedTransaction struct {
	Transaction
	Signatures []crypto.Signature
	ContextFreeData []bytes
}

type PackedTransaction struct {
	Signatures []crypto.Signature
	Compression CompressionType
	PackedContextFreeData []byte
	PackedTransaction []byte
}

// when the sender sends a transaction, they will assign it an id which will be passed back to sender if the transaction fails for some reason
type DeferredTransaction struct {
	SenderId uint32
	Sender AccountName
	Payer AccountName
	ExecuteAfter time.Time
}




