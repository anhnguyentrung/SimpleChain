package chain

import "time"

type BlockTimeStamp struct {
	BlockIntervalMs uint64 //ms
	BlockTimestampEpoch uint64 // ms
	Slot uint32
}

func NewBlockTimeStamp() BlockTimeStamp {
	return BlockTimeStamp{
		BlockIntervalMs:     BLOCK_INTERVAL_MS,
		BlockTimestampEpoch: BLOCK_TIMESTAMP_EPOCH,
	}
}

// t: ms
func (bt *BlockTimeStamp) SetTime(t uint64) {
	bt.Slot = uint32((t - bt.BlockTimestampEpoch) / bt.BlockIntervalMs)
}

func (bt BlockTimeStamp) Next() BlockTimeStamp {
	result := NewBlockTimeStamp()
	result.Slot = bt.Slot + 1
	return result
}

func (bt BlockTimeStamp) ToTime() time.Time {
	msec := int64(bt.Slot) * int64(BLOCK_INTERVAL_MS)
	msec += BLOCK_TIMESTAMP_EPOCH
	return time.Unix(0, msec * int64(time.Millisecond))
}
