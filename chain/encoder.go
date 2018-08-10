package chain

import (
	"io"
	"encoding/binary"
	"fmt"
	"reflect"
	bytes2 "bytes"
	"blockchain/crypto"
	"time"
	"errors"
)

type Encoder struct {
	Output io.Writer
	Order  binary.ByteOrder
	Count  int
	Extension func(v interface{}) error
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		Output: w,
		Order:  binary.LittleEndian,
		Count:  0,
		Extension: nil,
	}
}

func (e *Encoder) WriteName(name Name) error {
	val, err := StringToName(string(name))
	if err != nil {
		return fmt.Errorf("writeName: %s", err)
	}
	return e.WriteUint64(val)
}

func (e *Encoder) Encode(v interface{}) (err error) {
	switch cv := v.(type) {
	case Name:
		return e.WriteName(cv)
	case AccountName:
		name := Name(cv)
		return e.WriteName(name)
	case PermissionName:
		name := Name(cv)
		return e.WriteName(name)
	case string:
		return e.WriteString(cv)
	case byte:
		return e.WriteByte(cv)
	case int8:
		return e.WriteByte(byte(cv))
	case int16:
		return e.WriteInt16(cv)
	case uint16:
		return e.WriteUint16(cv)
	case uint32:
		return e.WriteUint32(cv)
	case uint64:
		return e.WriteUint64(cv)
	case bool:
		return e.WriteBool(cv)
	case []byte:
		return e.WriteByteArray(cv)
	case SHA256Type:
		return e.WriteSHA256(cv)
	case crypto.PublicKey:
		return e.WritePublicKey(cv)
	case crypto.Signature:
		return e.writeSignature(cv)
	case time.Time:
		return e.writeTimestamp(cv)
	default:

		rv := reflect.Indirect(reflect.ValueOf(v))
		t := rv.Type()

		switch t.Kind() {

		case reflect.Array:
			l := t.Len()
			for i := 0; i < l; i++ {
				if err = e.Encode(rv.Index(i).Interface()); err != nil {
					return
				}
			}
		case reflect.Slice:
			l := rv.Len()
			if err = e.WriteUVarInt(l); err != nil {
				return
			}
			for i := 0; i < l; i++ {
				if err = e.Encode(rv.Index(i).Interface()); err != nil {
					return
				}
			}
		case reflect.Struct:
			l := rv.NumField()
			for i := 0; i < l; i++ {
				field := t.Field(i)
				tag := field.Tag.Get("eos")
				if tag == "-" {
					continue
				}

				if v := rv.Field(i); t.Field(i).Name != "_" {
					if v.CanInterface() {
						isPresent := true
						if tag == "optional" {
							isPresent = !v.IsNil()
							e.WriteBool(isPresent)
						}
						if isPresent {
							if err = e.Encode(v.Interface()); err != nil {
								return
							}
						}
					}
				}
			}
		case reflect.Map:
			l := rv.Len()
			if err = e.WriteUVarInt(l); err != nil {
				return
			}
			for _, key := range rv.MapKeys() {
				value := rv.MapIndex(key)
				if err = e.Encode(key.Interface()); err != nil {
					return err
				}
				if err = e.Encode(value.Interface()); err != nil {
					return err
				}
			}
		default:
			if e.Extension != nil {
				return e.Extension(v)
			}
			return errors.New("Can not encode value of type " + t.String())
		}
	}
	return
}

func (e *Encoder) ToWriter(bytes []byte) (err error) {
	e.Count += len(bytes)
	_, err = e.Output.Write(bytes)
	return
}

func (e *Encoder) WriteByteArray(b []byte) error {
	if err := e.WriteUVarInt(len(b)); err != nil {
		return err
	}
	return e.ToWriter(b)
}

func (e *Encoder) WriteUVarInt(v int) (err error) {
	buf := make([]byte, 8)
	l := binary.PutUvarint(buf, uint64(v))
	return e.ToWriter(buf[:l])
}

func (e *Encoder) WriteByte(b byte) (err error) {
	return e.ToWriter([]byte{b})
}

func (e *Encoder) WriteBool(b bool) (err error) {
	var out byte
	if b {
		out = 1
	}
	return e.WriteByte(out)
}

func (e *Encoder) WriteUint16(i uint16) (err error) {
	buf := make([]byte, TypeSize.UInt16)
	binary.LittleEndian.PutUint16(buf, i)
	return e.ToWriter(buf)
}

func (e *Encoder) WriteInt16(i int16) (err error) {
	return e.WriteUint16(uint16(i))
}

func (e *Encoder) WriteUint32(i uint32) (err error) {
	buf := make([]byte, TypeSize.UInt32)
	binary.LittleEndian.PutUint32(buf, i)
	return e.ToWriter(buf)

}

func (e *Encoder) WriteUint64(i uint64) (err error) {
	buf := make([]byte, TypeSize.UInt64)
	binary.LittleEndian.PutUint64(buf, i)
	return e.ToWriter(buf)

}

func (e *Encoder) WriteString(s string) (err error) {
	return e.WriteByteArray([]byte(s))
}

func (e *Encoder) WriteSHA256(sha256 SHA256Type) error {
	return e.ToWriter(sha256[:])
}

func (e *Encoder) WritePublicKey(publicKey crypto.PublicKey) error {
	if len(publicKey.Content) != 33 {
		return fmt.Errorf("public key should be 33 bytes")
	}
	return e.ToWriter(publicKey.Content)
}

func (e *Encoder) writeSignature(sig crypto.Signature) error {
	if len(sig.Content) != 65 {
		return fmt.Errorf("signature should be 65 bytes")
	}

	return e.ToWriter(sig.Content) // should write 65 bytes
}

func (e *Encoder) writeTimestamp(t time.Time) error {
	n := uint64(t.UnixNano())
	//fmt.Println("encode pos ", e.Count)
	return e.WriteUint64(n)
}

func MarshalBinary(v interface{}) ([]byte, error) {
	buf := new(bytes2.Buffer)
	encoder := NewEncoder(buf)
	err := encoder.Encode(v)
	return buf.Bytes(), err
}
