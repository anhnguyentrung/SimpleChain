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
	"io/ioutil"
	"os"
	"encoding/binary"
	"log"
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
	buf, err := chain.MarshalBinary(message.Content)
	if err != nil {
		fmt.Println("Encode content: ", err)
		return err
	}
	message.Header.Length = uint32(len(buf))
	data, err := chain.MarshalBinary(message)
	c.Conn.Write(data)
	return err
}

type Packet struct {
	connection *Connection
	message Message
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
	newConn chan *Connection // trigger when a inbound connection is accepted
	doneConn chan *Connection // triger when a inbound connection is disconnected
	newPacket chan *Packet // trigger when received message packet from a inbound connection
}

func NewNode (p2pAddress string, suppliedPeers []string) *Node {
	return &Node {
		P2PAddress:p2pAddress,
		SuppliedPeers:suppliedPeers,
		PrivateKeys: make(map[string]crypto.PrivateKey,0),
		InboundConns:make(map[string]*Connection,0),
		OutboundConns:make(map[string]*Connection,0),
		newConn: make(chan *Connection),
		doneConn: make(chan *Connection),
		newPacket: make(chan *Packet),
	}
}

func (node *Node) Start() {
	go node.ConnectToPeers()
	go node.ListenFromPeers()
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
				os.Exit(1)
			}
			ic := NewConnection("")
			ic.Conn = inboundConn
			node.newConn <- ic
		}
	}()
	for {
		select {
		case connection := <-node.newConn:
			fmt.Println("accepted new client from address ", connection.RemoteAddr().String())
			node.addConnection(connection)
			go node.handleConnection(connection)
		case packet := <-node.newPacket:
			go node.handleMessage(packet.connection, packet)
		case doneConnection := <-node.doneConn:
			fmt.Println("disconnected client from address ", doneConnection.RemoteAddr().String())
			node.removeConnection(doneConnection)
		}
	}
	return nil
}

func (node *Node) handleMessage(c *Connection, packet *Packet) {
	switch msg := packet.message.Content.(type) {
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
	fmt.Println("received handshake message", message.ChainId)
}

func (node *Node) Close(c *Connection) {
	c.Close()
}

func (node *Node) ConnectToPeers() {
	fmt.Println("connecting to peer ...")
	for _, peerAddr := range node.SuppliedPeers {
		c := NewConnection(peerAddr)
		err := node.Connect(c)
		if err != nil {
			fmt.Println("connecting to peer: ", err)
		}
	}
}

func (node *Node) Connect(c *Connection) error {
	if !strings.Contains(c.PeerAddress, ":") {
		node.removeOutbound(c)
		return fmt.Errorf("invalid peer address %s", c.PeerAddress)
	}
	conn, err := net.Dial(TCP, c.PeerAddress)
	if err != nil {
		fmt.Println(err)
		return err
	}
	c.Connecting = true
	c.Conn = conn
	node.newConn <- c
	return nil
}

func (node *Node) addConnection(c *Connection) {
	if c.PeerAddress == "" {
		node.addNewInbound(c)
	} else {
		node.addNewOutbound(c)
		node.sendHandshake(c)
	}
}

func (node *Node) removeConnection(c *Connection) {
	if c.PeerAddress == "" {
		node.removeInbound(c)
	} else {
		node.removeOutbound(c)
	}
}

func (node *Node) addNewOutbound(c *Connection) {
	node.Lock()
	defer node.Unlock()
	node.OutboundConns[c.PeerAddress] = c
}

func (node *Node) removeOutbound(c *Connection) {
	node.Lock()
	defer node.Unlock()
	delete(node.OutboundConns, c.PeerAddress)
}

func (node *Node) addNewInbound(c *Connection) {
	node.Lock()
	defer node.Unlock()
	node.InboundConns[c.Conn.RemoteAddr().String()] = c
}

func (node *Node) removeInbound(c *Connection) {
	node.Lock()
	defer node.Unlock()
	delete(node.InboundConns, c.Conn.RemoteAddr().String())
}

func (node *Node) handleConnection(c *Connection) {
	r := bufio.NewReader(c.Conn)
	for {
		typeBuf := make([]byte, 1, 1)
		_, err := io.ReadFull(r, typeBuf)
		if err != nil {
			fmt.Println("Type ", err)
			break
		}
		msgType := typeBuf[0]
		lenBuf := make([]byte, 4, 4)
		_, err = io.ReadFull(r, lenBuf)
		if err != nil {
			log.Println("Length ", err)
			break
		}
		length := binary.LittleEndian.Uint32(lenBuf)
		fmt.Println("size of recevied message: ", length)
		msgData := make([]byte, length, length)
		n, err := io.ReadFull(r, msgData)
		if uint32(n) != length {
			fmt.Println("length of message is wrong")
			break
		}
		msg := Message{
			Header:MessageHeader{
				Type:msgType,
				Length:length,
			},
		}
		switch msgType {
		case Handshake:
			var msgContent HandshakeMessage
			decoder := chain.NewDecoder(msgData)
			err = decoder.Decode(&msgContent)
			msg.Content = msgContent
		case ChainSize:
			var msgContent ChainSizeMessage
			decoder := chain.NewDecoder(msgData)
			err = decoder.Decode(&msgContent)
			msg.Content= msgContent
		case GoAway:
			var msgContent GoAwayMessage
			decoder := chain.NewDecoder(msgData)
			err = decoder.Decode(&msgContent)
			msg.Content= msgContent
		case Time:
			var msgContent TimeMessage
			decoder := chain.NewDecoder(msgData)
			err = decoder.Decode(&msgContent)
			msg.Content= msgContent
		case Notice:
			var msgContent NoticeMessage
			decoder := chain.NewDecoder(msgData)
			err = decoder.Decode(&msgContent)
			msg.Content= msgContent
		case Request:
			var msgContent RequestMessage
			decoder := chain.NewDecoder(msgData)
			err = decoder.Decode(&msgContent)
			msg.Content= msgContent
		case SyncRequest:
			var msgContent SyncRequestMessage
			decoder := chain.NewDecoder(msgData)
			err = decoder.Decode(&msgContent)
			msg.Content= msgContent
		case SignedBlock:
			var msgContent chain.SignedBlock
			decoder := chain.NewDecoder(msgData)
			err = decoder.Decode(&msgContent)
			msg.Content= msgContent
		case PackedTransaction:
			var msgContent chain.PackedTransaction
			decoder := chain.NewDecoder(msgData)
			err = decoder.Decode(&msgContent)
			msg.Content= msgContent
		}
		packet := &Packet{connection: c, message: msg}
		node.newPacket <- packet
	}
	node.doneConn <- c
}

func decodeMessageData(r io.Reader) (message Message, err error) {
	data, err := ioutil.ReadAll(r)
	decoder := chain.NewDecoder(data)
	err = decoder.Decode(&message)
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

func (node *Node) sendHandshake(c *Connection) (err error) {
	handshakeMsg := node.newHandshakeMessage()
	c.LastHandshakeSent = handshakeMsg
	msg := Message{
		Header: MessageHeader{
			Type:Handshake,
			Length:0,
		},
		Content:handshakeMsg,
	}
	fmt.Println("sending handshake to ", c.RemoteAddr().String())
	err = c.sendMessage(msg)
	if err != nil {
		fmt.Println("error occurred when sending message")
	}
	return
}

