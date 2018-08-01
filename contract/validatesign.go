package main

import (
	"os"
	"blockchain/crypto"
	"crypto/sha256"
	"fmt"
)

func main() {
	pubString := os.Args[1]
	signString := os.Args[2]
	message := os.Args[3]
	hash := sha256.Sum256([]byte(message))
	sign, _ := crypto.NewSignature(signString)
	pub, _ := crypto.NewPublicKey(pubString)
	fmt.Println(sign.Verify(hash[:], pub))
}
