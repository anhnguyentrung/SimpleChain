package chain

type PermissionLevelWeight struct {
	Permission 	PermissionLevel
	Weight 		uint16
}

type KeyWeight struct {
	Key 	string
	Weight 	uint16
}

type WaitWeight struct {
	WaitSecond 	uint32
	Weight 		uint16
}

type Authority struct {
	Threshold 	uint32
	Keys 		[]KeyWeight
	Accounts 	[]PermissionLevelWeight
	Waits 		[]WaitWeight
}

