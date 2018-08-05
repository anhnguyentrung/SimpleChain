package chain

type PermissionLevel struct {
	Actor AccountName
	Permission PermissionName
}

type Action struct {
	Account 		AccountName
	Name 			ActionName
	Authorization 	[]PermissionLevel
	Data 			ActionData
}

type ActionReceipt struct {
	Receiver AccountName
	ActDigest SHA256Type
	GlobalSequence uint64
	RecvSequence uint64
	AuthSequence map[AccountName]uint64
}

type ActionData struct {
	HexData  []byte
	Data     interface{}
	abi      []byte
	toServer bool
}

func NewActionData(obj interface{}) ActionData {
	return ActionData{
		HexData:  []byte{},
		Data:     obj,
		toServer: true,
	}
}
