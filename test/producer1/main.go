package main

import (
	"blockchain/network"
	"blockchain/crypto"
	"blockchain/wallet"
	"fmt"
	"log"
	"blockchain/chain"
)

func createProducer1() (*wallet.SoftWallet, crypto.PublicKey) {
	sw := wallet.NewSoftWallet()
	sw.SetPassword("pass")
	sw.UnLock("pass")
	sw.WalletName = "wallet1.json"
	err := sw.LoadWalletFile()
	if err != nil {
		priv, _ := crypto.NewRandomPrivateKey()
		wif := priv.String()
		sw.ImportPrivateKey(wif)
		fmt.Println("pub ", priv.PublicKey().String())
		sw.SaveWalletFile()
		return sw, priv.PublicKey()
	}
	sw.UnLock("pass")
	pub := sw.ListPublicKeys()[0]
	priv := sw.ListKeys()[pub.String()]
	fmt.Println("pub ", pub.String())
	fmt.Println("priv ", priv.String())
	return sw, pub
}

func main() {
	sw, pub := createProducer1()
	sw.UnLock("pass")
	privateKey, _ := sw.GetPrivateKey(pub)
	if privateKey.PublicKey().String() != pub.String() {
		log.Fatal("key is wrong")
	}
	node := network.NewNode("0.0.0.0:2000", []string{""})
	producers := []chain.ProducerKey{}
	for index, producerName := range chain.PRODUCER_NAMES {
		producerPub, _ := crypto.NewPublicKey(chain.PRODUCER_PUBLIC_KEYS[index])
		producerKey := chain.ProducerKey{
			producerName,
			producerPub,
		}
		producers = append(producers, producerKey)
		node.AllowPeers = append(node.AllowPeers, producerPub)
		//node.Producer.ProducerWaterMarks[producerName] = 0
	}
	node.BlockChain.SetProposedProducers(producers)
	node.NetworkVersion = 1
	node.ChainId = node.BlockChain.ChainId
	node.NodeId = node.ChainId
	node.PrivateKeys[pub.String()] = privateKey
	node.Producer.Producers = append(node.Producer.Producers, chain.AccountName("producer1"))
	node.Producer.SignatureProviders[pub.String()] = network.MakeKeySignatureProvider(privateKey.String())
	done := make(chan bool)
	node.Start(true)
	<- done
}