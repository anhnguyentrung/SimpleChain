package main

import "blockchain/network"

func main() {
	node := network.NewNode("0.0.0.0:2000", []string{"localhost:2001"})
	node.Start()
}
