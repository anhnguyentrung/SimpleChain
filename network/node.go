package network

import (
	"blockchain/chain"
	"net"
	"blockchain/crypto"
	"strings"
	"fmt"
	"sync"
	"bufio"
	"io"
	"time"
	"crypto/sha256"
	"runtime"
	"log"
	"io/ioutil"
)

type Connection struct {
	net.Conn
	PeerAddress string
	Connecting bool
	Syncing bool
	NodeId chain.SHA256Type
	LastHandshakeReceived HandshakeMessage
	LastHandshakeSent HandshakeMessage
	SendHandshakeCount int16
}

func NewConnection(peerAddr string) *Connection {
	return &Connection{
		PeerAddress:peerAddr,
		Connecting:false,
		Syncing:false,
		LastHandshakeReceived:HandshakeMessage{},
		LastHandshakeSent:HandshakeMessage{},
		SendHandshakeCount:0,
	}
}

func (c *Connection) Close() {
	c.Connecting = false
	c.Syncing = false
	c.SendHandshakeCount = 0
	c.LastHandshakeReceived = HandshakeMessage{}
	c.LastHandshakeSent = HandshakeMessage{}
	c.Conn.Close()
}

func (c *Connection) sendMessage(message Message) error {
	packet := MessagePacket{
		Message: message,
	}
	encoder := chain.NewEncoder(c.Conn)
	return encoder.Encode(packet)
}

type messageFromConnection struct {
	connection *Connection
	packet *MessagePacket
}

type Node struct {
	sync.Mutex
	P2PAddress string
	SuppliedPeers []string
	AllowPeers []crypto.PublicKey
	PrivateKeys map[string]crypto.PrivateKey
	ChainId chain.SHA256Type
	NodeId chain.SHA256Type
	UserAgentName string
	InboundConns map[string]*Connection // p2p-listen-endpoint
	OutboundConns map[string]*Connection // p2p-peer-address
	NetworkVersion uint16
	newInConn chan *Connection // trigger when a inbound connection is accepted
	doneInConn chan *Connection // triger when a inbound connection is disconnected
	newInPacket chan *messageFromConnection // trigger when received message packet from a inbound connection
}

func NewNode (p2pAddress string, suppliedPeers []string) *Node {
	return &Node {
		P2PAddress:p2pAddress,
		SuppliedPeers:suppliedPeers,
		PrivateKeys: make(map[string]crypto.PrivateKey,0),
		InboundConns:make(map[string]*Connection,0),
		OutboundConns:make(map[string]*Connection,0),
	}
}

func (node *Node) Start() {
	go node.ListenFromPeers()
	go node.ConnectToPeers()
}

// Receive message
func (node *Node) ListenFromPeers() error {
	ln, err := net.Listen("tcp", node.P2PAddress)
	if err != nil {
		fmt.Println("start listening: ", err)
		return err
	}
	go func() {
		for {
			inboundConn, err := ln.Accept()
			if err != nil {
				fmt.Println("accepting connection: ", err)
			}
			ic := &Connection{}
			ic.Conn = inboundConn
			node.newInConn <- ic
		}
	}()
	for {
		select {
		case inConn := <-node.newInConn:
			fmt.Println("accepted new client from address ", inConn.RemoteAddr().String())
			node.InboundConns[inConn.Conn.RemoteAddr().String()] = inConn
			go node.handleConnection(inConn)
		case inPacket := <-node.newInPacket:
			go node.handleMessage(inPacket.connection, inPacket.packet)
		case doneInConn := <-node.doneInConn:
			fmt.Println("disconnected client from address ", doneInConn.RemoteAddr().String())
			delete(node.InboundConns, doneInConn.RemoteAddr().String())
		}
	}
	return nil
}

func (node *Node) handleMessage(c *Connection, packet *MessagePacket) {
	switch msg := packet.Message.(type) {
	case HandshakeMessage:
		c.LastHandshakeReceived = msg
		node.handleHandshakeMessage(c, msg)
	case ChainSizeMessage:
	case GoAwayMessage:
	case TimeMessage:
	case NoticeMessage:
	case RequestMessage:
	case SyncRequestMessage:
	case chain.SignedBlock:
	case chain.PackedTransaction:
	}
}

func (node *Node) handleHandshakeMessage(c *Connection, message HandshakeMessage) {
	fmt.Println("received handshake message")
}

func (node *Node) Close(c *Connection) {
	c.Close()
}

func (node *Node) ConnectToPeers() {
	for i := 0; i < len(node.SuppliedPeers); i++ {
		peerAddr := node.SuppliedPeers[i]
		c := NewConnection(peerAddr)
		err := node.Connect(c)
		if err != nil {
			fmt.Println("connecting to peer: ", err)
		}
	}
}

func (node *Node) Connect(c *Connection) error {
	if !strings.Contains(c.PeerAddress, ":") {
		delete(node.OutboundConns, c.PeerAddress)
		return fmt.Errorf("invalid peer address %s", c.PeerAddress)
	}
	conn, err := net.Dial(TCP, c.PeerAddress)
	if err != nil {
		fmt.Println(err)
		return err
	}
	c.Connecting = true
	c.Conn = conn
	node.addNewOutbound(c)
	return nil
}

func (node *Node) addNewOutbound(c *Connection) {
	node.Lock()
	defer node.Unlock()
	node.OutboundConns[c.PeerAddress] = c
}

func (node *Node) handleConnection(c *Connection) {
	r := bufio.NewReader(c.Conn)
	for {
		packet, err := decodeMessageData(r)
		if err != nil {
			log.Println("Error reading from p2p client:", err)
			break
		}
		msgFromConn := &messageFromConnection{connection: c, packet: packet}
		node.newInPacket <- msgFromConn
	}
	node.doneInConn <- c
}

func decodeMessageData(r io.Reader) (packet *MessagePacket, err error) {
	data, err := ioutil.ReadAll(r)
	packet = &MessagePacket{}
	decoder := chain.NewDecoder(data)
	err = decoder.Decode(packet)
	return
}

func (node *Node) SignCompact(signer crypto.PublicKey, digest chain.SHA256Type) crypto.Signature {
	if privateKey, ok := node.PrivateKeys[signer.String()]; ok {
		sig, _ := privateKey.Sign(digest[:])
		return sig
	}
	return crypto.Signature{
		Curve: crypto.CurveK1,
		Content: make([]byte, 65, 65),
	}
}

func (node *Node) newHandshakeMessage() HandshakeMessage {
	publicKey := crypto.PublicKey{}
	if len(node.PrivateKeys) > 0 {
		for k := range node.PrivateKeys {
			publicKey, _ = crypto.NewPublicKey(k)
			break
		}
	}
	currentTime := time.Now()
	timestamp := currentTime.UnixNano()
	bytesOfTimestamp, _ := chain.MarshalBinary(timestamp)
	token := sha256.Sum256(bytesOfTimestamp)
	return HandshakeMessage{
		NetworkVersion: node.NetworkVersion,
		ChainId: node.ChainId,
		NodeId: node.NodeId,
		Key: publicKey,
		Time: time.Now(),
		Token: token,
		Sig: node.SignCompact(publicKey, token),
		P2PAddress: node.P2PAddress,
		LastIrreversibleBlockNum: 0,
		LastIrreversibleBlockId: sha256.Sum256([]byte("")),
		HeadNum: 0,
		HeadId: sha256.Sum256([]byte("")),
		OS: runtime.GOOS,
		Agent: node.UserAgentName,
		Generation: int16(1),
	}
}

func (node *Node) sendHandshake(c *Connection) {
	c.LastHandshakeSent = node.newHandshakeMessage()
}

