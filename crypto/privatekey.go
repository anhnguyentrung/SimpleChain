package crypto

import (
	cryptorand "crypto/rand"
	"blockchain/btcsuite/btcd/btcec"
	"blockchain/btcsuite/btcd/chaincfg"
	"io"
	"fmt"
	"strings"
	"blockchain/btcsuite/btcutil"
	"crypto/sha256"
	"encoding/json"
)

const PrivateKeyPrefix = "PVT_"

func NewRandomPrivateKey() (*PrivateKey, error) {
	return newRandomPrivateKey(cryptorand.Reader)
}

func NewDeterministicPrivateKey(randSource io.Reader) (*PrivateKey, error) {
	return newRandomPrivateKey(randSource)
}

func newRandomPrivateKey(randSource io.Reader) (*PrivateKey, error) {
	rawPrivKey := make([]byte, 32)
	written, err := io.ReadFull(randSource, rawPrivKey)
	if err != nil {
		return nil, fmt.Errorf("error feeding crypto-rand numbers to seed ephemeral private key: %s", err)
	}
	if written != 32 {
		return nil, fmt.Errorf("couldn't write 32 bytes of randomness to seed ephemeral private key")
	}

	h := sha256.New()
	h.Write(rawPrivKey)
	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), h.Sum(nil))

	return &PrivateKey{privKey: privKey}, nil
}

func NewPrivateKeyForAccount(accountName string, role string) (*PrivateKey, error) {
	rawPrivKey := sha256.Sum256([]byte(accountName + role))
	h := sha256.New()
	h.Write(rawPrivKey[:])
	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), h.Sum(nil))

	return &PrivateKey{privKey: privKey}, nil
}

func NewPrivateKey(wif string) (*PrivateKey, error) {
	// Strip potential prefix, and set curve
	var privKeyMaterial string
	if strings.HasPrefix(wif, PrivateKeyPrefix) { // "PVT_"
		privKeyMaterial = wif[len(PrivateKeyPrefix):]

	} else { // no-prefix, like before
		privKeyMaterial = wif
	}

	wifObj, err := btcutil.DecodeWIF(privKeyMaterial)
	if err != nil {
		return nil, err
	}

	return &PrivateKey{privKey: wifObj.PrivKey}, nil
}

type PrivateKey struct {
	privKey *btcec.PrivateKey
}

func (p *PrivateKey) PublicKey() PublicKey {
	return PublicKey{Content: p.privKey.PubKey().SerializeCompressed()}
}

// Sign signs a 32 bytes SHA256 hash..
func (p *PrivateKey) Sign(hash []byte) (out Signature, err error) {
	if len(hash) != 32 {
		return out, fmt.Errorf("hash should be 32 bytes")
	}

	compactSig, err :=  btcec.SignCompact(btcec.S256(), p.privKey, hash, true)
	if err != nil {
		return out, fmt.Errorf("canonical, %s", err)
	}

	return Signature{Content: compactSig}, nil
}

func (p *PrivateKey) String() string {
	wif, _ := btcutil.NewWIF(p.privKey, &chaincfg.Params{PrivateKeyID:'\x80'}, false) // no error possible
	return wif.String()
}

func (p *PrivateKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

func (p *PrivateKey) UnmarshalJSON(v []byte) (err error) {
	var s string
	if err = json.Unmarshal(v, &s); err != nil {
		return
	}

	newPrivKey, err := NewPrivateKey(s)
	if err != nil {
		return
	}

	*p = *newPrivKey

	return
}

