package chain

import (
	"reflect"
	"fmt"
	"errors"
	"encoding/binary"
	"encoding/hex"
	"io"
	"io/ioutil"
)

var TypeSize = struct {
	Byte           int
	Int8           int
	UInt16         int
	Int16          int
	UInt32         int
	UInt64         int
	SHA256Bytes    int
	PublicKey      int
	Signature      int
	Timestamp      int
	BlockTimestamp int
	CurrencyName   int
	Bool           int
}{
	Byte:           1,
	Int8:           1,
	UInt16:         2,
	Int16:          2,
	UInt32:         4,
	UInt64:         8,
	SHA256Bytes:    32,
	PublicKey:      34,
	Signature:      66,
	Timestamp:      8,
	BlockTimestamp: 4,
	CurrencyName:   7,
	Bool:           1,
}

var RegisteredActions = map[AccountName]map[ActionName]reflect.Type{}

// Registers Action objects..
func RegisterAction(accountName AccountName, actionName ActionName, obj interface{}) {
	// TODO: lock or som'th.. unless we never call after boot time..
	if RegisteredActions[accountName] == nil {
		RegisteredActions[accountName] = make(map[ActionName]reflect.Type)
	}
	RegisteredActions[accountName][actionName] = reflect.TypeOf(obj)
}

// Decoder implements the EOS unpacking, similar to FC_BUFFER
type Decoder struct {
	data               []byte
	pos                int
	decodeP2PMessage   bool
	decodeTransactions bool
	decodeActions      bool
}

//var prefix = make([]string, 0)

var Debug bool

var print = func(s string) {
	if Debug {
		//for _, s := range prefix {
		//fmt.Print(s)
		//}
		fmt.Print(s)
	}
}
var println = func(args ...interface{}) {
	print(fmt.Sprintf("%s\n", args...))
}

func NewDecoder(data []byte) *Decoder {
	return &Decoder{
		data:               data,
		decodeP2PMessage:   true,
		decodeTransactions: true,
		decodeActions:      true,
	}
}

func (d *Decoder) DecodeP2PMessage(decode bool) {
	d.decodeP2PMessage = decode
}

func (d *Decoder) DecodeActions(decode bool) {
	d.decodeActions = decode
}

func (d *Decoder) Decode(v interface{}) (err error) {
	rv := reflect.Indirect(reflect.ValueOf(v))
	if !rv.CanAddr() {
		return errors.New("decode, can only Decode to pointer type")
	}
	t := rv.Type()

	println(fmt.Sprintf("Decode type [%T]", v))
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
	case *Name, *AccountName, *PermissionName, *ActionName, *TableName, *ScopeName:
		var n uint64
		n, err = d.readUint64()
		name := NameToString(n)
		println(fmt.Sprintf("readName [%s]", name))
		rv.SetString(name)
		return
	case *byte:
		var n byte
		n, err = d.readByte()
		rv.SetUint(uint64(n))
		return
	case *int16:
		var n int16
		n, err = d.readInt16()
		rv.SetInt(int64(n))
		return
	case *uint16:
		var n uint16
		n, err = d.readUint16()
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
		r, err = d.readBool()
		rv.SetBool(r)
		return
	case *[]byte:
		var data []byte
		data, err = d.readByteArray()
		rv.SetBytes(data)
		return
	case **Action:
		err = d.decodeStruct(v, t, rv)
		if err != nil {
			return
		}
		action := rv.Interface().(Action)

		if d.decodeActions {
			err = d.readActionData(&action)
		}

		rv.Set(reflect.ValueOf(action))
		return
	}

	switch t.Kind() {
	case reflect.Array:
		print("Array")
		len := t.Len()
		for i := 0; i < int(len); i++ {
			if err = d.Decode(rv.Index(i).Addr().Interface()); err != nil {
				return
			}
		}
		return

	case reflect.Slice:
		print("Reading Slice length ")
		var l uint64
		if l, err = d.readUvarint(); err != nil {
			return
		}
		println(fmt.Sprintf("Slice [%T] of length: %d", v, l))
		rv.Set(reflect.MakeSlice(t, int(l), int(l)))
		for i := 0; i < int(l); i++ {
			if err = d.Decode(rv.Index(i).Addr().Interface()); err != nil {
				return
			}
		}

	case reflect.Struct:

		err = d.decodeStruct(v, t, rv)
		if err != nil {
			return
		}

	case reflect.Map:
		//fmt.Println("Map")
		var l uint64
		if l, err = d.readUvarint(); err != nil {
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
		return errors.New("decode, unsupported type " + t.String())
	}

	return
}

func (d *Decoder) decodeStruct(v interface{}, t reflect.Type, rv reflect.Value) (err error) {
	l := rv.NumField()

	//prefix = append(prefix, "     ")
	for i := 0; i < l; i++ {

		if tag := t.Field(i).Tag.Get("eos"); tag == "-" {
			continue
		}

		if v := rv.Field(i); v.CanSet() && t.Field(i).Name != "_" {
			iface := v.Addr().Interface()
			println(fmt.Sprintf("Field name: %s", t.Field(i).Name))
			if err = d.Decode(iface); err != nil {
				return
			}
		}
	}
	//prefix = prefix[:len(prefix)-1]
	return
}

var ErrVarIntBufferSize = errors.New("varint: invalid buffer size")

func (d *Decoder) readUvarint() (uint64, error) {

	l, read := binary.Uvarint(d.data[d.pos:])
	if read <= 0 {
		println(fmt.Sprintf("readUvarint [%d]", l))
		return l, ErrVarIntBufferSize
	}

	d.pos += read
	println(fmt.Sprintf("readUvarint [%d]", l))
	return l, nil
}

func (d *Decoder) readByteArray() (out []byte, err error) {

	l, err := d.readUvarint()
	if err != nil {
		return nil, err
	}

	if len(d.data) < d.pos+int(l) {
		return nil, fmt.Errorf("byte array: varlen=%d, missing %d bytes", l, d.pos+int(l)-len(d.data))
	}

	out = d.data[d.pos : d.pos+int(l)]
	d.pos += int(l)

	println(fmt.Sprintf("readByteArray [%s]", hex.EncodeToString(out)))
	return
}

func (d *Decoder) readByte() (out byte, err error) {

	if d.remaining() < TypeSize.Byte {
		err = fmt.Errorf("byte required [1] byte, remaining [%d]", d.remaining())
		return
	}

	out = d.data[d.pos]
	d.pos++
	println(fmt.Sprintf("readByte [%d]", out))
	return
}

func (d *Decoder) readBool() (out bool, err error) {

	if d.remaining() < TypeSize.Bool {
		err = fmt.Errorf("bool required [%d] byte, remaining [%d]", TypeSize.Bool, d.remaining())
		return
	}

	b, err := d.readByte()

	if err != nil {
		err = fmt.Errorf("readBool, %s", err)
	}
	out = b != 0
	return

}

func (d *Decoder) readUint16() (out uint16, err error) {
	if d.remaining() < TypeSize.UInt16 {
		err = fmt.Errorf("uint16 required [%d] bytes, remaining [%d]", TypeSize.UInt16, d.remaining())
		return
	}

	out = binary.LittleEndian.Uint16(d.data[d.pos:])
	d.pos += TypeSize.UInt16
	return
}

func (d *Decoder) readInt16() (out int16, err error) {
	n, err := d.readUint16()
	out = int16(n)
	return
}
func (d *Decoder) readInt64() (out int64, err error) {
	n, err := d.readUint64()
	out = int64(n)
	return
}

func (d *Decoder) readUint32() (out uint32, err error) {
	if d.remaining() < TypeSize.UInt32 {
		err = fmt.Errorf("uint32 required [%d] bytes, remaining [%d]", TypeSize.UInt32, d.remaining())
		return
	}

	out = binary.LittleEndian.Uint32(d.data[d.pos:])
	d.pos += TypeSize.UInt32
	println(fmt.Sprintf("readUint32 [%d]", out))
	return
}

func (d *Decoder) readUint64() (out uint64, err error) {
	if d.remaining() < TypeSize.UInt64 {
		err = fmt.Errorf("uint64 required [%d] bytes, remaining [%d]", TypeSize.UInt64, d.remaining())
		return
	}

	data := d.data[d.pos : d.pos+TypeSize.UInt64]
	out = binary.LittleEndian.Uint64(data)
	d.pos += TypeSize.UInt64
	println(fmt.Sprintf("readUint64 [%d] [%s]", out, hex.EncodeToString(data)))
	return
}

func (d *Decoder) readString() (out string, err error) {
	data, err := d.readByteArray()
	out = string(data)
	println(fmt.Sprintf("readString [%s]", out))
	return
}

func (d *Decoder) readActionData(action *Action) (err error) {

	actionMap := RegisteredActions[action.Account]

	var decodeInto reflect.Type
	if actionMap != nil {
		objType := actionMap[action.Name]
		println("object type :", objType)
		if objType != nil {
			decodeInto = objType
		}
	}
	if decodeInto == nil {
		return
	}

	println("Reflect type :", decodeInto)
	obj := reflect.New(decodeInto)
	println("obj :", obj)
	err = UnmarshalBinary(action.Data.HexData, obj.Interface())
	if err != nil {
		return fmt.Errorf("decoding Action [%s], %s", obj.Type().Name(), err)
	}

	println("Object type :", obj.Interface())
	action.Data.Data = obj.Interface()

	return
}

func (d *Decoder) remaining() int {
	return len(d.data) - d.pos
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

