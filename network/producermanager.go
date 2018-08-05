package network

import (
	"blockchain/chain"
	"blockchain/crypto"
)

type SignatureProviderType interface {
	Sign(hash chain.SHA256Type) crypto.Signature
}

type ProducerManager struct {
	SignatureProviders map[string]SignatureProviderType
	Producers []chain.AccountName
}

func NewProducerManager() ProducerManager {
	return ProducerManager{}
}

func (pm ProducerManager) isProducerKey(pub crypto.PublicKey) bool {
	if _, ok := pm.SignatureProviders[pub.String()]; ok {
		return true
	}
	return false
}
