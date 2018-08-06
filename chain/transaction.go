package chain

import (
	"time"
	"blockchain/crypto"
	bytes2 "bytes"
	"compress/zlib"
	"log"
	"io/ioutil"
)

type TransactionHeader struct{
	Expiration time.Time
	RefBlockNum uint16
	RefBlockPrefix uint16
	MaxNetUsageWords Varuint32 // 1 word = 8 bytes
	MaxCPUUsageMs uint8
	DelaySec Varuint32
}

type SigningKeys struct {
	ChainId SHA256Type
	PublicKeys []crypto.PublicKey
}

type TransactionMetaData struct {
	Id SHA256Type
	Signed SHA256Type
	Trx SignedTransaction
	PackedTrx PackedTransaction
	SigningKeys SigningKeys
	Accepted bool
}

type Transaction struct {
	TransactionHeader
	ContextFreeActions []Action
	Actions []Action
	TransactionExtensions []Extension
}

type SignedTransaction struct {
	Transaction
	Signatures []crypto.Signature
	ContextFreeData []bytes
}

type PackedTransaction struct {
	Signatures []crypto.Signature
	Compression CompressionType
	PackedContextFreeData []byte
	PackedTrx []byte
	unPackedTrx *Transaction
}

type TransactionState struct {
	Id SHA256Type
	IsKnownByPeer bool
	IsNoticedByPeer bool
	BlockNum uint32
	Expires uint64
	RequestedTime time.Time
}

// when the sender sends a transaction, they will assign it an id which will be passed back to sender if the transaction fails for some reason
type DeferredTransaction struct {
	SenderId uint32
	Sender AccountName
	Payer AccountName
	ExecuteAfter time.Time
}

func NewSignedTransaction(tx *Transaction) *SignedTransaction {
	return &SignedTransaction{
		*tx,
		make([]crypto.Signature, 0),
		make([]bytes, 0),
	}
}

func (tx *Transaction) Pack(compression CompressionType) *PackedTransaction{
	var packedTrx []byte
	switch compression {
	case None:
		packedTrx, _ = MarshalBinary(*tx)
	case Zlib:
		packedTrx = tx.zlibCompress()
	}
	return &PackedTransaction{
		Signatures: make([]crypto.Signature, 0),
		Compression:compression,
		PackedContextFreeData:make([]byte, 0),
		PackedTrx: packedTrx,
	}
}

func (s *SignedTransaction) Pack(compression CompressionType) *PackedTransaction {
	tx := s.Transaction
	var packedTrx []byte
	packedContextFreeData, _ := MarshalBinary(s.ContextFreeData)
	switch compression {
	case None:
		packedTrx, _ = MarshalBinary(tx)
	case Zlib:
		packedTrx = tx.zlibCompress()
		packedContextFreeData = zlibCompress(packedContextFreeData)
	}
	return &PackedTransaction{
		Signatures:s.Signatures,
		Compression:compression,
		PackedContextFreeData:packedContextFreeData,
		PackedTrx:packedTrx,
	}
}

func (p *PackedTransaction) unPack() {
	out := zlibDecompress(p.PackedTrx)
	decoder := NewDecoder(out)
	var tx Transaction
	err := decoder.Decode(&tx)
	if err != nil {
		log.Fatal(err)
	}
	p.unPackedTrx = &tx
}

func (p *PackedTransaction) GetTransaction() *Transaction {
	p.unPack()
	return p.unPackedTrx
}

func (p *PackedTransaction) GetSingedTransaction() *SignedTransaction {
	tx := p.GetTransaction()
	s := NewSignedTransaction(tx)
	s.Signatures = p.Signatures
	switch p.Compression {
	case None:
		decoder := NewDecoder(p.PackedContextFreeData)
		err := decoder.Decode(&s.ContextFreeData)
		if err != nil {
			log.Fatal(err)
		}
	case Zlib:
		decompressed := zlibDecompress(p.PackedContextFreeData)
		decoder := NewDecoder(decompressed)
		err := decoder.Decode(&s.ContextFreeData)
		if err != nil {
			log.Fatal(err)
		}
	}
	return s
}

func (tx *Transaction) zlibCompress() bytes {
	in, _ := MarshalBinary(*tx)
	var buffer bytes2.Buffer
	w := zlib.NewWriter(&buffer)
	w.Write(in)
	w.Close()
	return buffer.Bytes()
}

func zlibCompress(data bytes) bytes {
	var buffer bytes2.Buffer
	w := zlib.NewWriter(&buffer)
	w.Write(data)
	w.Close()
	return buffer.Bytes()
}

func zlibDecompress(packedTrx []byte) bytes {
	buffer := bytes2.NewReader(packedTrx)
	r, err := zlib.NewReader(buffer)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()
	decompressed, err := ioutil.ReadAll(r)
	if err != nil {
		log.Fatal(err)
	}
	return decompressed
}




