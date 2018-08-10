package chain

import (
	"reflect"
	"fmt"
	"errors"
	"encoding/binary"
	"io"
	"io/ioutil"
	"blockchain/crypto"
	"time"
)

var TypeSize = struct {
	Byte           int
	Int8           int
	UInt16         int
	Int16          int
	UInt32         int
	UInt64         int
	SHA256Type     int
	PublicKey      int
	Signature      int
	Timestamp      int
	CurrencyName   int
	Bool           int
}{
	Byte:           1,
	Int8:           1,
	UInt16:         2,
	Int16:          2,
	UInt32:         4,
	UInt64:         8,
	SHA256Type:     32,
	PublicKey:      33,
	Signature:      65,
	Timestamp:      8,
	CurrencyName:   7,
	Bool:           1,
}

type Decoder struct {
	Data []byte
	Pos int
	Extension func(v interface{}) error
}

func NewDecoder(data []byte) *Decoder {
	return &Decoder{
		Data: data,
		Extension: nil,
	}
}

func (d *Decoder) Decode(v interface{}) (err error) {
	rv := reflect.Indirect(reflect.ValueOf(v))
	if !rv.CanAddr() {
		return errors.New("decode, can only Decode to pointer type")
	}
	t := rv.Type()
	if !rv.CanAddr() {
		return errors.New("binary: can only Decode to pointer type")
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		newRV := reflect.New(t)
		rv.Set(newRV)
		rv = reflect.Indirect(newRV)
	}

	switch v.(type) {
	case *string:
		s, e := d.readString()
		if e != nil {
			err = e
			return
		}
		rv.SetString(s)
		return
	case *Name, *AccountName, *PermissionName:
		var n uint64
		n, err = d.readUint64()
		name := NameToString(n)
		rv.SetString(name)
		return
	case *byte:
		//fmt.Println("*byte")
		var n byte
		n, err = d.ReadByte()
		rv.SetUint(uint64(n))
		return
	case *int16:
		var n int16
		n, err = d.readInt16()
		rv.SetInt(int64(n))
		return
	case *uint16:
		var n uint16
		n, err = d.ReadUint16()
		rv.SetUint(uint64(n))
		return
	case *uint32:
		var n uint32
		n, err = d.readUint32()
		rv.SetUint(uint64(n))
		return
	case *uint64:
		var n uint64
		n, err = d.readUint64()
		rv.SetUint(n)
		return
	case *bool:
		var r bool
		r, err = d.ReadBool()
		rv.SetBool(r)
		return
	case *[]byte:
		var data []byte
		data, err = d.ReadByteArray()
		rv.SetBytes(data)
		return
	case *SHA256Type:
		var s SHA256Type
		s, err = d.readSHA256()
		rv.Set(reflect.ValueOf(s))
		return
	case *crypto.PublicKey:
		var p crypto.PublicKey
		p, err = d.readPublicKey()
		rv.Set(reflect.ValueOf(p))
		return
	case *crypto.Signature:
		var s crypto.Signature
		s, err = d.ReadSignature()
		rv.Set(reflect.ValueOf(s))
		return
	case *time.Time:
		var ts time.Time
		ts, err = d.ReadTimestamp()
		rv.Set(reflect.ValueOf(ts))
		return
	}
	switch t.Kind() {
	case reflect.Array:
		len := t.Len()
		for i := 0; i < int(len); i++ {
			if err = d.Decode(rv.Index(i).Addr().Interface()); err != nil {
				return
			}
		}
		return
	case reflect.Slice:
		var l uint64
		if l, err = d.ReadUvarint(); err != nil {
			return
		}
		rv.Set(reflect.MakeSlice(t, int(l), int(l)))
		for i := 0; i < int(l); i++ {
			if err = d.Decode(rv.Index(i).Addr().Interface()); err != nil {
				return
			}
		}
	case reflect.Struct:
		err = d.DecodeStruct(v, t, rv)
		if err != nil {
			return
		}
	case reflect.Map:
		var l uint64
		if l, err = d.ReadUvarint(); err != nil {
			return
		}
		kt := t.Key()
		vt := t.Elem()
		rv.Set(reflect.MakeMap(t))
		for i := 0; i < int(l); i++ {
			kv := reflect.Indirect(reflect.New(kt))
			if err = d.Decode(kv.Addr().Interface()); err != nil {
				return
			}
			vv := reflect.Indirect(reflect.New(vt))
			if err = d.Decode(vv.Addr().Interface()); err != nil {
				return
			}
			rv.SetMapIndex(kv, vv)
		}
	default:
		if d.Extension != nil {
			return d.Extension(v)
		}
		return errors.New("Can not decode value of type " + t.String())
	}

	return
}

func (d *Decoder) DecodeStruct(v interface{}, t reflect.Type, rv reflect.Value) (err error) {
	l := rv.NumField()
	for i := 0; i < l; i++ {
		if tag := t.Field(i).Tag.Get("eos"); tag == "-" {
			continue
		}
		if v := rv.Field(i); v.CanSet() && t.Field(i).Name != "_" {
			iface := v.Addr().Interface()
			if err = d.Decode(iface); err != nil {
				return
			}
		}
	}
	return
}

var ErrVarIntBufferSize = errors.New("varint: invalid buffer size")

func (d *Decoder) ReadUvarint() (uint64, error) {
	l, read := binary.Uvarint(d.Data[d.Pos:])
	if read <= 0 {
		return l, ErrVarIntBufferSize
	}
	d.Pos += read
	return l, nil
}

func (d *Decoder) ReadByteArray() (out []byte, err error) {
	l, err := d.ReadUvarint()
	if err != nil {
		return nil, err
	}
	if len(d.Data) < d.Pos+int(l) {
		return nil, fmt.Errorf("byte array: varlen=%d, missing %d bytes", l, d.Pos+int(l)-len(d.Data))
	}
	out = d.Data[d.Pos : d.Pos+int(l)]
	d.Pos += int(l)
	return
}

func (d *Decoder) ReadByte() (out byte, err error) {
	if d.Remaining() < TypeSize.Byte {
		err = fmt.Errorf("byte required [1] byte, remaining [%d]", d.Remaining())
		return
	}
	out = d.Data[d.Pos]
	d.Pos++
	return
}

func (d *Decoder) ReadBool() (out bool, err error) {
	if d.Remaining() < TypeSize.Bool {
		err = fmt.Errorf("bool required [%d] byte, remaining [%d]", TypeSize.Bool, d.Remaining())
		return
	}
	b, err := d.ReadByte()
	if err != nil {
		err = fmt.Errorf("readBool, %s", err)
	}
	out = b != 0
	return

}

func (d *Decoder) ReadUint16() (out uint16, err error) {
	if d.Remaining() < TypeSize.UInt16 {
		err = fmt.Errorf("uint16 required [%d] bytes, remaining [%d]", TypeSize.UInt16, d.Remaining())
		return
	}

	out = binary.LittleEndian.Uint16(d.Data[d.Pos:])
	d.Pos += TypeSize.UInt16
	return
}

func (d *Decoder) readInt16() (out int16, err error) {
	n, err := d.ReadUint16()
	out = int16(n)
	return
}
func (d *Decoder) readInt64() (out int64, err error) {
	n, err := d.readUint64()
	out = int64(n)
	return
}

func (d *Decoder) readUint32() (out uint32, err error) {
	if d.Remaining() < TypeSize.UInt32 {
		err = fmt.Errorf("uint32 required [%d] bytes, remaining [%d]", TypeSize.UInt32, d.Remaining())
		return
	}

	out = binary.LittleEndian.Uint32(d.Data[d.Pos:])
	d.Pos += TypeSize.UInt32
	return
}

func (d *Decoder) readUint64() (out uint64, err error) {
	if d.Remaining() < TypeSize.UInt64 {
		err = fmt.Errorf("uint64 required [%d] bytes, remaining [%d]", TypeSize.UInt64, d.Remaining())
		return
	}

	data := d.Data[d.Pos : d.Pos+TypeSize.UInt64]
	out = binary.LittleEndian.Uint64(data)
	d.Pos += TypeSize.UInt64
	return
}

func (d *Decoder) readString() (out string, err error) {
	data, err := d.ReadByteArray()
	out = string(data)
	return
}

func (d *Decoder) readSHA256() (out SHA256Type, err error) {

	if d.Remaining() < TypeSize.SHA256Type {
		err = fmt.Errorf("sha256 required [%d] bytes, remaining [%d]", TypeSize.SHA256Type, d.Remaining())
		return
	}
	out = SHA256Type{}
	copy(out[:], d.Data[d.Pos : d.Pos+TypeSize.SHA256Type])
	d.Pos += TypeSize.SHA256Type
	return
}

func (d *Decoder) readPublicKey() (out crypto.PublicKey, err error) {

	if d.Remaining() < TypeSize.PublicKey {
		err = fmt.Errorf("publicKey required [%d] bytes, remaining [%d]", TypeSize.PublicKey, d.Remaining())
		return
	}
	out = crypto.PublicKey{
		Content: d.Data[d.Pos : d.Pos+TypeSize.PublicKey], // 33 bytes
	}
	d.Pos += TypeSize.PublicKey
	return
}

func (d *Decoder) ReadSignature() (out crypto.Signature, err error) {
	if d.Remaining() < TypeSize.Signature {
		err = fmt.Errorf("signature required [%d] bytes, remaining [%d]", TypeSize.Signature, d.Remaining())
		return
	}
	out = crypto.Signature{
		Content: d.Data[d.Pos : d.Pos+TypeSize.Signature], 	// 65 bytes
	}
	d.Pos += TypeSize.Signature
	return
}

func (d *Decoder) ReadTimestamp() (out time.Time, err error) {

	if d.Remaining() < TypeSize.Timestamp {
		err = fmt.Errorf("tstamp required [%d] bytes, remaining [%d]", TypeSize.Timestamp, d.Remaining())
		return
	}

	unixNano, err := d.readUint64()
	out = time.Unix(0, int64(unixNano))
	return
}

func (d *Decoder) Remaining() int {
	return len(d.Data) - d.Pos
}

func UnmarshalBinaryReader(reader io.Reader, v interface{}) (err error) {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return
	}
	return UnmarshalBinary(data, v)
}

func UnmarshalBinary(data []byte, v interface{}) (err error) {
	decoder := NewDecoder(data)
	return decoder.Decode(v)
}

