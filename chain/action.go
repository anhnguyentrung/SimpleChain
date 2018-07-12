package chain

type PermissionLevel struct {
	Actor string
	Permission string
}

type Action struct {
	Account 		string
	Name 			string
	Authorization 	[]PermissionLevel
	Data 			[]byte
}
