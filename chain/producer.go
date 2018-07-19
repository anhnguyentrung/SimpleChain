package chain

import "blockchain/crypto"

type ProducerKey struct {
	ProducerName AccountName
	BlockSigningKey crypto.PublicKey
}

type ProducerScheduleType struct {
	Version uint32
	Producers []ProducerKey
}
