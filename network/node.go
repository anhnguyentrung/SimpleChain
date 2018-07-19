package network

import (
	"blockchain/chain"
	"net"
	"blockchain/crypto"
)

type Node struct {
	PeerAddress string
	AllowPeers []crypto.PublicKey
	PrivateKeys map[string]crypto.PrivateKey
	ChainId chain.SHA256Type
	NodeId chain.SHA256Type
	UserAgentName string
	Connections map[string]*net.Conn
	NetworkVersionMatch bool
}

func (node *Node) FindConnection(host string) *net.Conn {
	return node.Connections[host]
}
