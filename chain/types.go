package chain

type Name string
type AccountName Name
type PermissionName Name
type ActionName Name
type TableName Name
type ScopeName Name
type bytes []byte
type Varuint32 uint32
type SHA256Type [32]byte

const NEW_ACCOUNT  = "newaccount"
const OWNER = "owner"
const ACTIVE = "active"

type Extension struct {
	Type uint16
	Buffer []byte
}

type CompressionType uint8
const(
	None CompressionType = iota
	Zlib
)

type TransactionStatus uint8
const(
	Executed TransactionStatus = iota 	// succeed, no error handler executed
	Soft_Fail 							// objectively failed (not executed), error handler executed
	Hard_Fail 							// objectively failed and error handler objectively failed thus no state change
	Delayed 							// transaction delayed/deferred/scheduled for future execution
	Expired  							// transaction expired and storage space refuned to user
)
