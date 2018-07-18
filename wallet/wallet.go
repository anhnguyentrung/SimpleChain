package wallet

import (
	"blockchain/crypto"
	"crypto/sha512"
	"blockchain/chain"
	"errors"
	"log"
	"bytes"
	"crypto/aes"
	"encoding/json"
	"os"
	"io"
	"strings"
	"io/ioutil"
	"fmt"
	"crypto/rand"
	"crypto/cipher"
)

type PlainKeys struct {
	Keys map[string]string
	Checksum [64]byte
}

type WalletData struct {
	CipherKeys []byte
}

type SoftWallet struct {
	CipherKeys []byte
	WalletName string
	Keys map[string]*crypto.PrivateKey
	Checksum [64]byte
}

func NewSoftWallet() *SoftWallet {
	return &SoftWallet{
		CipherKeys: make([]byte, 0),
		WalletName:	"",
		Keys: make(map[string]*crypto.PrivateKey, 0),
		Checksum: sha512.Sum512([]byte("")),
	}
}

func (sw *SoftWallet) GetPrivateKey(publicKey crypto.PublicKey) (*crypto.PrivateKey, error) {
	hasKey := sw.tryGetPrivateKey(publicKey.String())
	if hasKey == nil {
		return nil, errors.New("private key doesn't exist")
	}
	return hasKey, nil
}

func (sw *SoftWallet) tryGetPrivateKey(publicKey string) *crypto.PrivateKey {
	if priv, ok := sw.Keys[publicKey]; ok {
		return priv
	}
	return nil
}

func (sw *SoftWallet) IsLocked() bool {
	checksum := sha512.Sum512([]byte(""))
	return bytes.Equal(sw.Checksum[:], checksum[:])
}

func (sw *SoftWallet) Lock() {
	if sw.IsLocked() { log.Fatal("wallet is locking") }
	sw.EncryptKeys()
	for k := range sw.Keys {
		delete(sw.Keys, k)
	}
	sw.Checksum = sha512.Sum512([]byte(""))
}

func (sw *SoftWallet) UnLock(password string) {
	if len(password) == 0 { log.Fatal("password must not empty") }
	pw := sha512.Sum512([]byte(password))
	block, err := aes.NewCipher(pw[0:32])
	if err != nil {
		log.Fatal("error: %s", err)
	}
	decrypted := make([]byte, len(sw.CipherKeys[aes.BlockSize:]))
	decryptStream := cipher.NewCTR(block, sw.CipherKeys[:aes.BlockSize])
	decryptStream.XORKeyStream(decrypted, sw.CipherKeys[aes.BlockSize:])
	//fmt.Println("decrypt",len(decrypted))
	decoder := chain.NewDecoder(decrypted)
	var plainKeys PlainKeys
	err = decoder.Decode(&plainKeys)
	if err != nil {
		log.Fatal(err)
	}
	if !bytes.Equal(plainKeys.Checksum[:], pw[:]) {
		log.Fatal("password is wrong")
	}
	for k := range plainKeys.Keys {
		privateKey, _ := crypto.NewPrivateKey(plainKeys.Keys[k])
		sw.Keys[k] = privateKey
	}
	sw.Checksum = plainKeys.Checksum
}

func (sw *SoftWallet) CheckPassword(password string) {
	if len(password) == 0 { log.Fatal("password must not empty") }
	pw := sha512.Sum512([]byte(password))
	block, err := aes.NewCipher(pw[0:32])
	if err != nil {
		log.Fatal("error: %s", err)
	}
	decrypted := make([]byte, len(sw.CipherKeys))
	block.Decrypt(decrypted, sw.CipherKeys)
	decoder := chain.NewDecoder(decrypted)
	var plainKeys PlainKeys
	err = decoder.Decode(&plainKeys)
	if err != nil {
		log.Fatal("unpacking key data")
	}
	if !bytes.Equal(plainKeys.Checksum[:], pw[:]) {
		log.Fatal("password is wrong")
	}
}

func (sw *SoftWallet) SetPassword(password string) {
	if !sw.IsNew() {
		if sw.IsLocked() { log.Fatal("The wallet must be unlocked before the password can be set") }
	}
	sw.Checksum = sha512.Sum512([]byte(password))
	sw.Lock()
}

func (sw *SoftWallet) ListKeys() map[string]*crypto.PrivateKey{
	if sw.IsLocked() {log.Fatal("The wallet is locking")}
	return sw.Keys
}

func (sw *SoftWallet) ListPublicKeys() []crypto.PublicKey {
	if sw.IsLocked() {log.Fatal("The wallet is locking")}
	var pubKeys []crypto.PublicKey
	for k := range sw.Keys {
		pubKey, _ := crypto.NewPublicKey(k)
		pubKeys = append(pubKeys, pubKey)
	}
	return pubKeys
}

func (sw *SoftWallet) CreateKey() string {
	if sw.IsLocked() {log.Fatal("The wallet is locking")}
	privateKey,_ := crypto.NewRandomPrivateKey()
	sw.ImportPrivateKey(privateKey.String())
	sw.SaveWalletFile()
	return privateKey.PublicKey().String()
}

func (sw *SoftWallet) ImportPrivateKey(wifKey string) error {
	privateKey, err := crypto.NewPrivateKey(wifKey)
	if err != nil {
		return err
	}
	wifPublicKey := privateKey.PublicKey().String()
	if _, ok := sw.Keys[wifPublicKey]; ok {
		return errors.New("Key already in wallet")
	}
	sw.Keys[wifPublicKey] = privateKey
	return nil
}

func (sw *SoftWallet) RemoveKey(key string) bool {
	if sw.IsLocked() {log.Fatal("The wallet is locking")}
	if _, ok := sw.Keys[key]; ok {
		delete(sw.Keys, key)
		sw.SaveWalletFile()
		return true
	}
	log.Fatal("Key not in wallet")
	return false
}

func (sw *SoftWallet) IsNew() bool {
	return len(sw.CipherKeys) == 0
}

func (sw *SoftWallet) SaveWalletFile() error {
	sw.EncryptKeys()
	walletData := &WalletData{
		CipherKeys: sw.CipherKeys,
	}
	data, err := json.Marshal(walletData)
	if err != nil {
		return err
	}
	fo, err := os.Create(sw.WalletName)
	if err != nil {
		return err
	}
	defer fo.Close()
	_, err = io.Copy(fo, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	return nil
}

func (sw *SoftWallet) LoadWalletFile() error {
	fi, err := os.Open(sw.WalletName)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer fi.Close()
	data, _ := ioutil.ReadAll(fi)
	walletData := &WalletData{
		CipherKeys: make([]byte, 0),
	}
	json.Unmarshal(data, walletData)
	sw.CipherKeys = walletData.CipherKeys
	return nil
}

func (sw *SoftWallet) EncryptKeys() {
	if !sw.IsLocked() {
		plainKeys := PlainKeys{
			Keys: make(map[string]string, 0),
			Checksum: sw.Checksum,
		}
		for k := range sw.Keys {
			plainKeys.Keys[k] = sw.Keys[k].String()
		}
		buf, _ := chain.MarshalBinary(plainKeys)
		//fmt.Println("encrypt", len(buf))
		block, err := aes.NewCipher(sw.Checksum[0:32])
		if err != nil {
			log.Fatal("error: %s", err)
		}
		sw.CipherKeys = make([]byte, aes.BlockSize+len(buf))
		iv := sw.CipherKeys[:aes.BlockSize]
		if _, err := io.ReadFull(rand.Reader, iv); err != nil {
			fmt.Printf("err: %s\n", err)
		}
		encryptStream := cipher.NewCTR(block, iv)
		encryptStream.XORKeyStream(sw.CipherKeys[aes.BlockSize:], buf)
	}
}

func (sw *SoftWallet) TrySignDigest(digest []byte, publicKey crypto.PublicKey) (crypto.Signature, error) {
	if privateKey, ok := sw.Keys[publicKey.String()]; ok {
		privateKey.Sign(digest)
	}
	return crypto.Signature{}, fmt.Errorf("private key not found for public key [%s]", publicKey.String())
}