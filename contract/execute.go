package main

import (
	"os/exec"
	"fmt"
	"blockchain/crypto"
	"crypto/sha256"
	"os"
)

func main() {
	args := os.Args[1:]
	cmdArgs := []string{"wasm_exec.js"}
	message := "hello"
	priv, _ := crypto.NewRandomPrivateKey()
	pubStr := priv.PublicKey().String()
	hash := sha256.Sum256([]byte(message))
	sign, _ := priv.Sign(hash[:])
	signStr := sign.String()
	args = append(args, pubStr, signStr, message)
	cmdArgs = append(cmdArgs, args...)
	out, err := exec.Command("node", cmdArgs...).Output()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("結果: %s\n", out)
}
