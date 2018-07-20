package main

import "blockchain/network"

func main() {
	node := network.Node{
		P2PAddress:"0.0.0.0:2000",
		SuppliedPeers:[]string{"localhost:2001"},
	}
	node.Start()
}
