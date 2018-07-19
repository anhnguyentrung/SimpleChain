package chain

type Name string
type AccountName Name
type PermissionName Name
type ActionName Name
type TableName Name
type ScopeName Name
type bytes []byte

const NEW_ACCOUNT  = "newaccount"

type Extension struct {
	Type uint16
	Buffer []byte
}

type CompressionType uint8
const(
	None CompressionType = iota
	Zlib
)
