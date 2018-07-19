package test

import (
	"testing"
	"blockchain/wallet"
	"blockchain/crypto"
	"fmt"
)

func TestWallet(t *testing.T) {
	sw := wallet.NewSoftWallet()
	checkLock(sw, t)
	sw.SetPassword("pass")
	checkLock(sw, t)
	sw.UnLock("pass")
	checkUnLock(sw, t)
	sw.WalletName = "wallet_test.json"
	if len(sw.ListPublicKeys()) > 0 {
		t.Fatal("should not contain key")
	}
	priv, _ := crypto.NewRandomPrivateKey()
	pub := priv.PublicKey()
	wif := priv.String()
	sw.ImportPrivateKey(wif)
	fmt.Println(wif)
	if len(sw.ListPublicKeys()) != 1 {
		t.Fatal("contain too much key")
	}
	privCopy,_ := sw.GetPrivateKey(pub)
	if privCopy.String() != wif {
		t.Fatal("private key not same")
	}
	sw.Lock()
	checkLock(sw, t)
	sw.UnLock("pass")
	checkUnLock(sw, t)
	if len(sw.ListPublicKeys()) != 1 {
		t.Fatal("contain too much key")
	}
	sw.SaveWalletFile()
	sw2 := wallet.NewSoftWallet()
	sw2.WalletName = "wallet_test.json"
	checkLock(sw2, t)
	sw2.LoadWalletFile()
	checkLock(sw2, t)
	sw2.UnLock("pass")
	if len(sw2.ListPublicKeys()) != 1 {
		t.Fatal("contain too much key")
	}
	privCopy2,_ := sw.GetPrivateKey(pub)
	if privCopy2.String() != wif {
		t.Fatal("private key not same")
	}
}

func checkLock(sw *wallet.SoftWallet, t *testing.T) {
	if !sw.IsLocked() {
		t.Fatal("wallet is not locked")
	}
}

func checkUnLock(sw *wallet.SoftWallet, t *testing.T) {
	if sw.IsLocked() {
		t.Fatal("wallet is locked")
	}
}