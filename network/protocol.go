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
	NetworkVersion           uint16
	ChainId                  chain.SHA256Type
	NodeId                   chain.SHA256Type
	Key                      crypto.PublicKey
	Time                     time.Time
	Token                    chain.SHA256Type
	Sig                      crypto.Signature
	P2PAddress               string
	LastIrreversibleBlockNum uint32
	LastIrreversibleBlockId  chain.SHA256Type
	HeadNum                  uint32
	HeadId                   chain.SHA256Type
	OS                       string
	Agent                    string
	Generation               int16
}

type GoAwayMessage struct {
	Reason GoAwayReason
	NodeId chain.SHA256Type
}

func (gam GoAwayMessage) reasonToString() string {
	switch gam.Reason {
	case Self : return "self connect";
	case Duplicate : return "duplicate";
	case Wrong_Chain : return "wrong chain";
	case Wrong_Version : return "wrong version";
	case Forked : return "chain is forked";
	case Unlinkable : return "unlinkable block received";
	case Bad_Transaction : return "bad transaction";
	case Validation : return "invalid block";
	case Authentication : return "authentication failure";
	case Fatal_Other : return "some other failure";
	case Benign_Other : return "some other non-fatal condition";
	default : return "some crazy reason";
	}
}

func NewGoAwayMessage(reason GoAwayReason) GoAwayMessage {
	return GoAwayMessage{
		Reason:reason,
	}
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
