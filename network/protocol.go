package network

import (
	"blockchain/chain"
	"blockchain/crypto"
	"time"
)

type ChainSizeMessage struct {
	LastIrreversibleBlockNum uint32
	LastIrreversibleBlockId chain.SHA256Type
	HeadNum uint32
	HeadId chain.SHA256Type
}

type HandshakeMessage struct {
	NetworkVersion uint16
	ChainId chain.SHA256Type
	NodeId chain.SHA256Type
	Key crypto.PublicKey
	Time time.Time
	Token chain.SHA256Type
	Sig crypto.Signature
	P2PAddress string
	LastIrreversibleBlockNum uint32
	LastIrreversibleBlockId chain.SHA256Type
	HeadNum uint32
	HeadId chain.SHA256Type
	OS string
	Agent string
	Generation int16
}

type GoAwayMessage struct {
	Reason GoAwayReason
	NodeId chain.SHA256Type
}

type TimeMessage struct {
	Origin time.Time
	Receive time.Time
	Transmit time.Time
	Destination time.Time
}

type OrderedTransactionIds struct {
	Mode IdListModes
	Pending uint32
	Ids []chain.SHA256Type
}

type OrderedBlockIds struct {
	Mode IdListModes
	Pending uint32
	Ids []chain.SHA256Type
}

type NoticeMessage struct {
	KnownTrx OrderedTransactionIds
	KnownBlocks OrderedBlockIds
}

type RequestMessage struct {
	ReqTrx OrderedTransactionIds
	ReqBlocks OrderedBlockIds
}

type SyncRequestMessage struct {
	StartBlock uint32
	EndBlock uint32
}
