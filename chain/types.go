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
const DEFAULT_MAX_TRX_LIFETIME = 60*60 // 60 minutes
const DEFAULT_DEFERRED_TRX_EXPIRATION_WINDOW = 10*60 // 10 minutes
const DEFAULT_MAX_TRX_DELAY = 45*24*3600 // 45 days
const DEFAULT_MAX_INLINE_ACTION_SIZE = 4*1024 // 4KB
const DEFAULT_MAX_INLINE_ACTION_DEPTH = 4
const DEFAULT_MAX_AUTH_DEPTH  = 6
const BLOCK_INTERVAL_MS = 500
const BLOCK_TIMESTAMP_EPOCH = 946684800000
const PRODUCER_REPETITION = 12
const MAXIMUM_TRACKED_DPOS_CONFIRMATIONS = 1024

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

type BlockStatus uint8
const(
	Irreversible BlockStatus = iota
	Validated   = 1
	complete   = 2
	incomplete  = 3
)
