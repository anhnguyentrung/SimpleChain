package chain

import "time"

type BlockTimeStamp struct {
	BlockIntervalNs uint64 //nanosecond
	Slot uint64
}

func NewBlockTimeStamp() BlockTimeStamp {
	return BlockTimeStamp{
		BlockIntervalNs: BLOCK_INTERVAL_NS,
	}
}

// t: ms
func (bt *BlockTimeStamp) SetTime(t uint64) {
	bt.Slot = t / bt.BlockIntervalNs
}

func (bt BlockTimeStamp) Next() BlockTimeStamp {
	result := NewBlockTimeStamp()
	result.Slot = bt.Slot + 1
	return result
}

func (bt BlockTimeStamp) ToTime() time.Time {
	nsec := int64(bt.Slot) * int64(BLOCK_INTERVAL_NS)
	return time.Unix(0, nsec)
}
