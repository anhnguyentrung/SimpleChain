package chain

import "time"

type BlockTimeStamp struct {
	BlockIntervalMs uint64 //ms
	Slot uint64
}

func NewBlockTimeStamp() BlockTimeStamp {
	return BlockTimeStamp{
		BlockIntervalMs: BLOCK_INTERVAL_MS,
	}
}

// t: ms
func (bt *BlockTimeStamp) SetTime(t uint64) {
	bt.Slot = t / bt.BlockIntervalMs
}

func (bt BlockTimeStamp) Next() BlockTimeStamp {
	result := NewBlockTimeStamp()
	result.Slot = bt.Slot + 1
	return result
}

func (bt BlockTimeStamp) ToTime() time.Time {
	msec := int64(bt.Slot) * int64(BLOCK_INTERVAL_MS)
	return time.Unix(0, msec * int64(time.Millisecond))
}
