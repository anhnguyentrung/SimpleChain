package main

import (
	"strconv"
	"os"
	"fmt"
)

func add(a uint32, b uint32) uint64 {
	return uint64(a + b)
}

func main() {
	ia, _ := strconv.Atoi(os.Args[1])
	a := uint32(ia)
	ib, _ := strconv.Atoi(os.Args[2])
	b := uint32(ib)
	r := add(a, b)
	fmt.Println(r)
}