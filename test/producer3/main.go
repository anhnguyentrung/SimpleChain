package main

import (
	"blockchain/crypto"
	"fmt"
	"log"
	"blockchain/network"
	"blockchain/wallet"
)

func createProducer3() (*wallet.SoftWallet, crypto.PublicKey) {
	sw := wallet.NewSoftWallet()
	sw.SetPassword("pass")
	sw.UnLock("pass")
	sw.WalletName = "wallet3.json"
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
	fmt.Println("pub ", pub.String())
	return sw, pub
}

func main() {
	sw, pub := createProducer3()
	sw.UnLock("pass")
	privateKey, _ := sw.GetPrivateKey(pub)
	if privateKey.PublicKey().String() != pub.String() {
		log.Fatal("key is wrong")
	}
	node := network.NewNode("0.0.0.0:2002", []string{"localhost:2000"})
	node.NetworkVersion = 1
	node.ChainId = node.BlockChain.ChainId
	node.NodeId = node.ChainId
	node.PrivateKeys[pub.String()] = privateKey
	done := make(chan bool)
	node.Start(true)
	<- done
}
