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
