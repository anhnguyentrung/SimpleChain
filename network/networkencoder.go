package network

import (
	"blockchain/chain"
	"bytes"
	"errors"
	"reflect"
)

func marshalBinary(v interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	encoder := chain.NewEncoder(buf)
	extension := func(v interface{}) error {
		switch cv := v.(type) {
		case MessageTypes:
			return encoder.WriteByte(byte(cv))
		case GoAwayReason:
			return encoder.WriteByte(byte(cv))
		default:
			rv := reflect.Indirect(reflect.ValueOf(v))
			t := rv.Type()
			return errors.New("Can not encode value of type " + t.String())
		}
	}
	encoder.Extension = extension
	err := encoder.Encode(v)
	return buf.Bytes(), err
}