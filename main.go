package main

import (
	"blockchain/wallet"
	"log"
	"blockchain/crypto"
	"fmt"
)

func main() {
	sw := wallet.NewSoftWallet()
	checkLock(sw)
	sw.SetPassword("pass")
	checkLock(sw)
	sw.UnLock("pass")
	checkUnLock(sw)
	sw.WalletName = "wallet_test.json"
	if len(sw.ListPublicKeys()) > 0 {
		log.Fatal("should not contain key")
	}
	priv, _ := crypto.NewRandomPrivateKey()
	pub := priv.PublicKey()
	wif := priv.String()
	sw.ImportPrivateKey(wif)
	fmt.Println(wif)
	if len(sw.ListPublicKeys()) != 1 {
		log.Fatal("contain too much key")
	}
	privCopy,_ := sw.GetPrivateKey(pub)
	if privCopy.String() != wif {
		log.Fatal("private key not same")
	}
	sw.Lock()
	checkLock(sw)
	sw.UnLock("pass")
	checkUnLock(sw)
	if len(sw.ListPublicKeys()) != 1 {
		log.Fatal("contain too much key")
	}
	sw.SaveWalletFile()
	sw2 := wallet.NewSoftWallet()
	sw2.WalletName = "wallet_test.json"
	checkLock(sw2)
	sw2.LoadWalletFile()
	checkLock(sw2)
	sw2.UnLock("pass")
	if len(sw2.ListPublicKeys()) != 1 {
		log.Fatal("contain too much key")
	}
	privCopy2,_ := sw.GetPrivateKey(pub)
	if privCopy2.String() != wif {
		log.Fatal("private key not same")
	}
}

func checkLock(sw *wallet.SoftWallet) {
	if !sw.IsLocked() {
		log.Fatal("wallet is not locked")
	}
}

func checkUnLock(sw *wallet.SoftWallet) {
	if sw.IsLocked() {
		log.Fatal("wallet is locked")
	}
}
