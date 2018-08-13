package utils

func Min(a, b uint16) uint16 {
	if a < b {
		return  a
	}
	return b
}

func MinUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func MaxUint64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func MaxUint32(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}