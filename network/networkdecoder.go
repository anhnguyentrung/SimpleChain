package network

import (
	"blockchain/chain"
	"reflect"
	"errors"
)

type NetworkDecoder struct {
	*chain.Decoder
}

func unmarshalBinary(data []byte, v interface{}) (err error) {
	decoder := chain.NewDecoder(data)
	extension := func(v interface{}) error {
		rv := reflect.Indirect(reflect.ValueOf(v))
		switch v.(type) {
		case *MessageTypes, *GoAwayReason, *IdListModes:
			var n byte
			n, err := decoder.ReadByte()
			rv.SetUint(uint64(n))
			return err
		default:
			rv := reflect.Indirect(reflect.ValueOf(v))
			t := rv.Type()
			return errors.New("Can not encode value of type " + t.String())
		}
	}
	decoder.Extension = extension
	return decoder.Decode(v)
}