package crypto

import (
	"fmt"
	"blockchain/btcsuite/btcutil/base58"
	"blockchain/btcsuite/btcd/btcec"
	"strings"
	"bytes"
	"encoding/json"
)

// Signature represents a signature for some hash
type Signature struct {
	Content []byte // the Compact signature as bytes
}

// Verify checks the signature against the pubKey. `hash` is a sha256
// hash of the payload to verify.
func (s Signature) Verify(hash []byte, pubKey PublicKey) bool {
	recoveredKey, _, err := btcec.RecoverCompact(btcec.S256(), s.Content, hash)
	if err != nil {
		return false
	}
	key, err := pubKey.Key()
	if err != nil {
		return false
	}
	if recoveredKey.IsEqual(key) {
		return true
	}
	return false
}

// PublicKey retrieves the public key, but requires the
// payload.. that's the way to validate the signature. Use Verify() if
// you only want to validate.
func (s Signature) PublicKey(hash []byte) (out PublicKey, err error) {
	recoveredKey, _, err := btcec.RecoverCompact(btcec.S256(), s.Content, hash)
	if err != nil {
		return out, err
	}

	return PublicKey{
		Content: recoveredKey.SerializeCompressed(),
	}, nil
}

func (s Signature) String() string {
	checksum := ripemd160checksum(s.Content)
	buf := append(s.Content[:], checksum...)
	return "SIG_" + base58.Encode(buf)
}

func NewSignature(fromText string) (Signature, error) {
	if !strings.HasPrefix(fromText, "SIG_") {
		return Signature{}, fmt.Errorf("signature should start with SIG_")
	}
	if len(fromText) < 8 {
		return Signature{}, fmt.Errorf("invalid signature length")
	}

	fromText = fromText[4:] // remove the `SIG_` prefix

	sigbytes := base58.Decode(fromText)

	content := sigbytes[:len(sigbytes)-4]
	checksum := sigbytes[len(sigbytes)-4:]
	verifyChecksum := ripemd160checksum(content)
	if !bytes.Equal(verifyChecksum, checksum) {
		return Signature{}, fmt.Errorf("signature checksum failed, found %x expected %x", verifyChecksum, checksum)
	}

	return Signature{Content: content}, nil
}

func (a Signature) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

func (a *Signature) UnmarshalJSON(data []byte) (err error) {
	var s string
	err = json.Unmarshal(data, &s)
	if err != nil {
		return
	}

	*a, err = NewSignature(s)

	return
}
