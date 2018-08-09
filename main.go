package main

import (
	"blockchain/network"
	"encoding/hex"
	"blockchain/crypto"
)

func main() {
	node := network.NewNode("0.0.0.0:2001", []string{"localhost:2000"})
	node.NetworkVersion = 1
	chainId, _ := hex.DecodeString("bf4dd1a5e59c6dc90bddb4f678ec24a2d2d4678b1513f5b483f134e586fc4643")
	copy(node.ChainId[:], chainId)
	node.NodeId = node.ChainId
	privateKey, _ := crypto.NewPrivateKey("5JRFptLJfq16Qk9fZummqyKkhuaDjK5R1PAp1uGZ1E29SXVUfbJ")
	node.PrivateKeys[privateKey.PublicKey().String()] = privateKey
	done := make(chan bool)
	node.Start()
	<- done
}
