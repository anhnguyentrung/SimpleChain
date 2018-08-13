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
	"os"
	"encoding/binary"
	"log"
	"bytes"
	"blockchain/btcsuite/btcd/btcec"
	"blockchain/database"
)

type Connection struct {
	net.Conn
	PeerAddress string
	Connected bool
	Syncing bool
	NodeId chain.SHA256Type
	LastHandshakeReceived HandshakeMessage
	LastHandshakeSent HandshakeMessage
	SendHandshakeCount int16
	ForkHead chain.SHA256Type
	ForkHeadNum uint32
	NetworkVersion uint16
	TransactionStates []chain.TransactionState
	BlockStates []*PeerBlockState
	LastRequest *RequestMessage
	PeerRequested *SyncState
}

func NewConnection(peerAddr string) *Connection {
	return &Connection{
		PeerAddress:peerAddr,
		Connected:false,
		Syncing:false,
		LastHandshakeReceived:HandshakeMessage{},
		LastHandshakeSent:HandshakeMessage{},
		SendHandshakeCount:0,
	}
}

func (c *Connection) Close() {
	c.Connected = false
	c.Syncing = false
	c.SendHandshakeCount = 0
	c.LastHandshakeReceived = HandshakeMessage{}
	c.LastHandshakeSent = HandshakeMessage{}
	c.Conn.Close()
}

func (c *Connection) PeerName() string {
	if c.LastHandshakeReceived.P2PAddress != "" {
		return c.LastHandshakeReceived.P2PAddress
	}
	if c.PeerAddress != "" {
		return c.PeerAddress
	}
	return "connecting client"
}

func (c *Connection) Current() bool {
	return c.Connected && !c.Syncing
}

func (c *Connection) sendMessage(message Message) error {
	buf, err := marshalBinary(message.Content)
	if err != nil {
		fmt.Println("Encode content: ", err)
		return err
	}
	message.Header.Length = uint32(len(buf))
	data, err := marshalBinary(message)
	c.Conn.Write(data)
	return err
}

func (c *Connection) sendGoAwayMessage(reason GoAwayReason) error {
	goAwayMsg := NewGoAwayMessage(reason)
	content, _ := marshalBinary(goAwayMsg)
	msg := Message{
		Header: MessageHeader{
			Type:Handshake,
			Length:0,
		},
		Content: content,
	}
	return c.sendMessage(msg)
}

func (c *Connection) addPeerBlock(entry *PeerBlockState) bool {
	var block *PeerBlockState = nil
	for _, blk := range c.BlockStates {
		if bytes.Equal(blk.Id[:], entry.Id[:]) {
			block = blk
			break
		}
	}
	added := (block == nil)
	if added {
		c.BlockStates = append(c.BlockStates, entry)
	} else {
		block.IsKnown = true
		if block.BlockNum == 0 {
			block.BlockNum = entry.BlockNum
		} else {
			block.RequestedTime = time.Now()
		}
	}
	return added
}

type Packet struct {
	connection *Connection
	message Message
}

type NodeTransactionState struct{
	Id chain.SHA256Type
	Expires time.Time
	PackedTrx chain.PackedTransaction
	//serializedTrx []byte
	BlockNum uint32
	TrueBlock uint32
	Requests uint32
}

type Node struct {
	sync.Mutex
	P2PAddress string
	SuppliedPeers []string
	AllowPeers []crypto.PublicKey
	PrivateKeys map[string]*crypto.PrivateKey
	ChainId chain.SHA256Type
	NodeId chain.SHA256Type
	UserAgentName string
	Conns map[string]*Connection
	NetworkVersion uint16
	NetworkVersionMatch bool
	newConn chan *Connection // trigger when a inbound connection is accepted
	doneConn chan *Connection // triger when a inbound connection is disconnected
	newPacket chan *Packet // trigger when received message packet from a inbound connection
	SyncManager *SyncManager
	Dispatcher *DispatchManager
	BlockChain *database.BlockChain
	LocalTrxs []NodeTransactionState
	Producer *ProducerManager
}

func NewNode (p2pAddress string, suppliedPeers []string) *Node {
	return &Node {
		P2PAddress:p2pAddress,
		SuppliedPeers:suppliedPeers,
		PrivateKeys: make(map[string]*crypto.PrivateKey,0),
		Conns:make(map[string]*Connection,0),
		newConn: make(chan *Connection),
		doneConn: make(chan *Connection),
		newPacket: make(chan *Packet),
		NetworkVersionMatch: false,
		BlockChain: database.NewBlockChain(),
		Producer: NewProducerManager(),
		Dispatcher: &DispatchManager{},
		UserAgentName: "",
	}
}

func (node *Node) Start(isProducer bool) {
	go node.ConnectToPeers()
	go node.ListenFromPeers()
	if isProducer {
		node.Producer.Startup(node)
	}
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
		node.handleGoAwayMessage(c, msg)
	case TimeMessage:
	case NoticeMessage:
		node.handleNotice(c, msg)
	case RequestMessage:
		node.handleRequest(c, msg)
	case SyncRequestMessage:
		node.handleSyncRequest(c, msg)
	case chain.SignedBlock:
		node.handleSignedBlock(c, msg)
	case chain.PackedTransaction:
	}
}

func isValidHandshakeMessage(message HandshakeMessage) bool {
	valid := true
	if message.LastIrreversibleBlockNum > message.HeadNum {
		valid = false
	}
	if message.P2PAddress == "" {
		valid = false
	}
	if message.OS == "" {
		valid = false
	}
	emptySig := make([]byte, 65, 65)
	emptyHash := sha256.Sum256([]byte(""))
	isEmptySig := bytes.Equal(message.Sig.Content, emptySig)
	isEmptyHash := bytes.Equal(message.Token[:], emptyHash[:])
	bytesOfTimestamp, _ := marshalBinary(message.Time)
	hashOfTimestamp := sha256.Sum256(bytesOfTimestamp)
	fmt.Println("receive time ", message.Time.UnixNano())
	if ((!isEmptySig || !isEmptyHash) && (!bytes.Equal(message.Token[:], hashOfTimestamp[:])))    {
		valid = false
	}
	return valid
}

func (node *Node) handleGoAwayMessage(c *Connection, message GoAwayMessage) {
	if message.Reason == Duplicate {
		c.NodeId = message.NodeId
	}
	c.Close()
}

func (node *Node) handleHandshakeMessage(c *Connection, message HandshakeMessage) {
	fmt.Println("received handshake message", message.ChainId)
	if !isValidHandshakeMessage(message) {
		fmt.Println("bad handshake message")
		c.sendGoAwayMessage(Fatal_Other)
		return
	}
	//libNum := node.BlockChain.LastIrreversibleBlockNum()
	//peerLib := message.LastIrreversibleBlockNum
	//if !c.Connected {
	//	c.Connected = true
	//}
	//if message.Generation == 1 {
	//	if message.NodeId == node.NodeId {
	//		c.sendGoAwayMessage(Self)
	//		return
	//	}
	//	if c.PeerAddress == "" || c.LastHandshakeReceived.NodeId == sha256.Sum256([]byte("")) {
	//		fmt.Println("checking for duplicate")
	//		for _, check := range node.Conns {
	//			if check == c {
	//				continue
	//			}
	//			if check.Connected && check.PeerAddress == message.P2PAddress {
	//				if message.Time.UnixNano() + c.LastHandshakeSent.Time.UnixNano()  <= check.LastHandshakeSent.Time.UnixNano() + check.LastHandshakeReceived.Time.UnixNano() {
	//					continue
	//				}
	//				c.sendGoAwayMessage(Duplicate)
	//				return
	//			}
	//		}
	//	} else {
	//		fmt.Println("skipping duplicate check")
	//	}
	//	if message.ChainId != node.ChainId {
	//		fmt.Println("Peer on a different chain. Closing connection")
	//		c.sendGoAwayMessage(Wrong_Chain)
	//		return
	//	}
	//	c.NetworkVersion = message.NetworkVersion
	//	if c.NetworkVersion != node.NetworkVersion {
	//		if node.NetworkVersionMatch {
	//			fmt.Println("Peer network version does not match")
	//			c.sendGoAwayMessage(Wrong_Version)
	//			return
	//		} else {
	//			fmt.Printf("Local network version %d, remote network version %d\n", node.NetworkVersion, c.NetworkVersion)
	//
	//		}
	//	}
	//	if c.NodeId !=  message.NodeId {
	//		c.NodeId = message.NodeId
	//	}
	//	if !node.authenticatePeer(message) {
	//		fmt.Println("Peer not authenticated")
	//		c.sendGoAwayMessage(Authentication)
	//		return
	//	}
	//	onFork := false
	//	if peerLib <= libNum && peerLib > 0 {
	//		peerLibId := node.BlockChain.GetBlockIdForNum(peerLib)
	//		onFork = (message.LastIrreversibleBlockId != peerLibId)
	//		if onFork {
	//			c.sendGoAwayMessage(Forked)
	//			return
	//		}
	//	}
	//	if c.SendHandshakeCount == 0 {
	//		node.sendHandshake(c)
	//
	//	}
	//}
	c.LastHandshakeReceived = message
	node.SyncManager.ReceiveHanshake(message, c, node)
}

func (node *Node) handleNotice(c *Connection, message NoticeMessage) {
	c.Connected = true
	req:= RequestMessage{}
	sendReq := false
	switch message.KnownTrx.Mode {
	case None:
		break
	case Last_Irr_Catch_Up:
		c.LastHandshakeReceived.HeadNum = message.KnownTrx.Pending
		req.ReqTrx.Mode = None
	case Catch_Up:
		if message.KnownTrx.Pending > 0 {
			req.ReqTrx.Mode = Catch_Up
			sendReq = true
			knownSum := len(node.LocalTrxs)
			if knownSum > 0 {
				for _, trx := range node.LocalTrxs {
					req.ReqTrx.Ids = append(req.ReqTrx.Ids, trx.Id)
				}
			}
		}
	case Normal:
		node.Dispatcher.receiveNotice(c, node, message, false)
	}
	switch message.KnownBlocks.Mode {
	case None:
		if message.KnownTrx.Mode != Normal {
			return
		}
	case Catch_Up, Last_Irr_Catch_Up:
		node.SyncManager.receiveNotice(c, node, message)
	case Normal:
		node.Dispatcher.receiveNotice(c, node, message, false)
	default:
		break
	}
	if sendReq {
		content, _ := marshalBinary(req)
		msg := Message{
			Header: MessageHeader{
				Type:Request,
				Length:0,
			},
			Content:content,
		}
		c.sendMessage(msg)
	}
}

func (node *Node) handleRequest(c *Connection, message RequestMessage) {
	switch message.ReqBlocks.Mode {
	case Catch_Up:
		fmt.Println("Received request message: catch up")
		node.blockSendBranch(c)
	case Normal:
		fmt.Println("Received request message: normal")
		node.blockSend(c, message.ReqBlocks.Ids)
	default:
		break
	}

	switch message.ReqTrx.Mode {
	case Catch_Up:
		node.transactionSendPending(c, message.ReqTrx.Ids)
	case Normal:
		node.transactionSend(c, message.ReqTrx.Ids)
	case None:
		if message.ReqBlocks.Mode == None {
			c.Syncing = false
		}
	default:
		break
	}
}

func (node *Node) handleSyncRequest(c *Connection, message SyncRequestMessage) {
	if message.EndBlock == 0 {
		c.PeerRequested = nil
	} else {
		c.PeerRequested = &SyncState{
			StartBlock:message.StartBlock,
			EndBlock:message.EndBlock,
			Last:message.StartBlock - 1,
		}
		blockchain := node.BlockChain
		c.PeerRequested.Last += 1
		num := c.PeerRequested.Last
		triggerSend := num == c.PeerRequested.StartBlock
		if num == c.PeerRequested.EndBlock {
			c.PeerRequested = nil
		}
		block := blockchain.FetchBlockByNum(num)
		if block != nil && triggerSend {
			content, _ := marshalBinary(*block)
			msg := Message{
				Header: MessageHeader{
					Type:SignedBlock,
					Length:0,
				},
				Content: content,
			}
			c.sendMessage(msg)
		}
	}
}

func (node *Node) handleSignedBlock(c *Connection, signedBlock chain.SignedBlock) {
	blockchain := node.BlockChain
	blockId := signedBlock.Id()
	blockNum := signedBlock.BlockNum()
	fmt.Println("receive block ", blockNum)
	if blockchain.FetchBlockById(blockId) != nil {
		node.SyncManager.receiveBlock(c, node, blockId, blockNum)
	}
	node.Dispatcher.receiveBlock(c, blockId, blockNum)

}

func (node *Node) transactionSendPending(c *Connection, ids []chain.SHA256Type) {
	for _, tx := range node.LocalTrxs {
		if tx.BlockNum == 0 {
			found := false
			for _, known := range ids {
				if known == tx.Id {
					found = true
					break
				}
			}
			if !found {
				msg := Message{
					Header: MessageHeader{
						Type:PackedTransaction,
						Length:0,
					},
					Content:tx.PackedTrx,
				}
				c.sendMessage(msg)
			}
		}
	}
}

func (node *Node) transactionSend(c *Connection, ids []chain.SHA256Type) {
	for _, id := range ids {
		tx := node.findLocalTrx(id)
		if tx != nil {
			msg := Message{
				Header: MessageHeader{
					Type:PackedTransaction,
					Length:0,
				},
				Content:tx.PackedTrx,
			}
			c.sendMessage(msg)
		}
	}
}

func (node *Node) blockSend(c *Connection, ids []chain.SHA256Type) {
	blockchain := node.BlockChain
	count := 0
	for _, id := range ids {
		count += 1
		block := blockchain.FetchBlockById(id)
		if block != nil {
			msg := Message{
				Header: MessageHeader{
					Type:SignedBlock,
					Length:0,
				},
				Content:*block,
			}
			c.sendMessage(msg)
		} else {
			break
		}
	}
}

func (node *Node) blockSendBranch(c *Connection) {
	blockchain := node.BlockChain
	headNum := blockchain.Head.BlockNum
	noticeMsg := NoticeMessage{}
	noticeMsg.KnownBlocks.Mode = Normal
	noticeMsg.KnownBlocks.Pending = 0
	if headNum == 0 {
		msg := Message{
			Header: MessageHeader{
				Type:Notice,
				Length:0,
			},
			Content:noticeMsg,
		}
		c.sendMessage(msg)
		return
	}
	libId := blockchain.LastIrreversibleBlockId()
	headId := blockchain.Head.Id
	if libId == nil {
		fmt.Println("unable to retrieve block data")
		msg := Message{
			Header: MessageHeader{
				Type:Notice,
				Length:0,
			},
			Content:noticeMsg,
		}
		c.sendMessage(msg)
		return
	}
	bStack := make([]*chain.SignedBlock, 0)
	nullId := chain.SHA256Type{}
	for bid := headId; !bytes.Equal(bid[:], libId[:]) && !bytes.Equal(bid[:], nullId[:]); {
		b := blockchain.FetchBlockById(bid)
		if b != nil {
			bid = b.Previous
			bStack = append(bStack, b)
		} else {
			break
		}
	}
	if len(bStack) > 0 {
		last := bStack[len(bStack)-1]
		if bytes.Equal(last.Previous[:], libId[:]) {
			for len(bStack) > 0 {
				msg := Message{
					Header: MessageHeader{
						Type:SignedBlock,
						Length:0,
					},
					Content:*bStack[len(bStack)-1],
				}
				c.sendMessage(msg)
				// pop back
				bStack[len(bStack)-1] = nil
				bStack = bStack[:len(bStack)-1]
			}
		} else {
			fmt.Println("Nothing to send on fork request")
		}
	}
	c.Syncing = false
}

func (node *Node) authenticatePeer(message HandshakeMessage) bool {
	var pub *crypto.PublicKey = nil
	for _, peer := range node.AllowPeers {
		if bytes.Equal(peer.Content, message.Key.Content) {
			pub = &peer
			break
		}
	}
	priv := node.PrivateKeys[message.Key.String()]
	producer := NewProducerManager()
	foundProducerKey := producer.isProducerKey(message.Key)
	if pub == nil && priv == nil && !foundProducerKey {
		return false
	}
	now := time.Now().Unix()
	if now - message.Time.Unix() > PeerAuthenticationInterval {
		return false
	}
	emptySig := make([]byte, 65, 65)
	emptyHash := sha256.Sum256([]byte(""))
	isEmptySig := bytes.Equal(message.Sig.Content, emptySig)
	isEmptyHash := bytes.Equal(message.Token[:], emptyHash[:])
	if !isEmptySig && !isEmptyHash {
		bytesOfTimestamp, _ := marshalBinary(message.Time)
		hash := sha256.Sum256(bytesOfTimestamp)
		if !bytes.Equal(hash[:], message.Token[:]) {
			fmt.Println("invalid token")
		}
		peerPub, _, err := btcec.RecoverCompact(btcec.S256(), message.Sig.Content, message.Token[:])
		if err != nil {
			fmt.Println("unrecoverable key")
			return false
		}
		if !bytes.Equal(peerPub.SerializeCompressed(), message.Key.Content[:]) {
			fmt.Println("unauthenticated key")
			return false
		}
	} else {
		return false
	}
	return true
}

func (node *Node) handlePackedTransaction(c *Connection, packedTransaction chain.PackedTransaction) {
	fmt.Println("received packed transaction")
	//if node.SyncManager.isActive(c, node) {
	//	fmt.Println("got a trx during sync - dropping")
	//	return
	//}
	//tid := packedTransaction.Id()
	//foundTx := node.findLocalTrx(tid)
	//if foundTx != nil {
	//	fmt.Println("got a duplicate transaction - dropping")
	//	return
	//}
	//node.Dispatcher.receiveTransaction(c, tid)
}

func (node *Node) onIncomingTransaction(packedTrx *chain.PackedTransaction) {
	//blockchain := node.BlockChain
	//pendingBlockState := blockchain.PendingBlocKState()
	//if pendingBlockState == nil {
	//	return
	//}
	//blockTime := pendingBlockState.Header.Timestamp.ToTime()
	//id := packedTrx.Id()
	//if packedTrx.Expiration().Before(blockTime) {
	//	fmt.Println("expired transaction ", id)
	//	return
	//}
	//if node.isKnownUnexpiredTransaction(id) {
	//	fmt.Println("duplicate transaction ", id)
	//	return
	//}
	//deadline := time.Now().Unix() * 1000 + MaxTransactionTime // ms
	//deadlineIsSubjective := false
	//if node.Producer.PendingBlockMode == Producing && (blockTime.Unix() * 1000) < deadline {
	//	deadlineIsSubjective = true
	//	deadline = blockTime.Unix() * 1000
	//}

}

// entry point for new transaction to the block state
// Check authorization here
func (node *Node) pushTransaction(trx *chain.TransactionMetaData, deadline time.Time) {

}

func (node *Node) isKnownUnexpiredTransaction(id chain.SHA256Type) bool {
	return node.BlockChain.DB.FindTransactionObject(id) != nil
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
	c.Conn = conn
	node.newConn <- c
	return nil
}

func (node *Node) addConnection(c *Connection) {
	if c.PeerAddress == "" {
		node.addNewInbound(c)
	} else {
		node.addNewOutbound(c)
		//node.sendHandshake(c)
	}
	c.Connected = true
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
	node.Conns[c.PeerAddress] = c
}

func (node *Node) removeOutbound(c *Connection) {
	node.Lock()
	defer node.Unlock()
	delete(node.Conns, c.PeerAddress)
}

func (node *Node) addNewInbound(c *Connection) {
	node.Lock()
	defer node.Unlock()
	node.Conns[c.Conn.RemoteAddr().String()] = c
}

func (node *Node) removeInbound(c *Connection) {
	node.Lock()
	defer node.Unlock()
	delete(node.Conns, c.Conn.RemoteAddr().String())
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
		msgType := MessageTypes(typeBuf[0])
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
			err = unmarshalBinary(msgData, &msgContent)
			msg.Content = msgContent
		case ChainSize:
			var msgContent ChainSizeMessage
			err = unmarshalBinary(msgData, &msgContent)
			msg.Content= msgContent
		case GoAway:
			var msgContent GoAwayMessage
			err = unmarshalBinary(msgData, &msgContent)
			msg.Content= msgContent
		case Time:
			var msgContent TimeMessage
			err = unmarshalBinary(msgData, &msgContent)
			msg.Content= msgContent
		case Notice:
			var msgContent NoticeMessage
			err = unmarshalBinary(msgData, &msgContent)
			msg.Content= msgContent
		case Request:
			var msgContent RequestMessage
			err = unmarshalBinary(msgData, &msgContent)
			msg.Content= msgContent
		case SyncRequest:
			var msgContent SyncRequestMessage
			err = unmarshalBinary(msgData, &msgContent)
			msg.Content= msgContent
		case SignedBlock:
			var msgContent chain.SignedBlock
			err = unmarshalBinary(msgData, &msgContent)
			msg.Content= msgContent
		case PackedTransaction:
			var msgContent chain.PackedTransaction
			err = unmarshalBinary(msgData, &msgContent)
			msg.Content= msgContent
		}
		packet := &Packet{connection: c, message: msg}
		node.newPacket <- packet
	}
	node.doneConn <- c
}

func (node *Node) SignCompact(signer crypto.PublicKey, digest chain.SHA256Type) crypto.Signature {
	if privateKey, ok := node.PrivateKeys[signer.String()]; ok {
		sig, _ := privateKey.Sign(digest[:])
		return sig
	}
	return crypto.Signature{
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
	fmt.Println("send time ", currentTime.UnixNano())
	bytesOfTimestamp, _ := marshalBinary(currentTime)
	token := sha256.Sum256(bytesOfTimestamp)
	return HandshakeMessage{
		NetworkVersion: node.NetworkVersion,
		ChainId: node.ChainId,
		NodeId: node.NodeId,
		Key: publicKey,
		Time: currentTime,
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
	fmt.Println("sending handshake ", handshakeMsg.LastIrreversibleBlockNum, handshakeMsg.HeadNum)
	err = c.sendMessage(msg)
	if err != nil {
		fmt.Println("error occurred when sending message ", err)
	}
	return
}

func (node *Node) findLocalTrx(id chain.SHA256Type) *NodeTransactionState {
	var foundTrx *NodeTransactionState = nil
	for _, trx := range node.LocalTrxs {
		if bytes.Equal(trx.Id[:], id[:]) {
			foundTrx = &trx
			break
		}
	}
	return foundTrx
}

func (node *Node) sendAll(ignoredConnection *Connection, message Message) {
	for _, c := range node.Conns {
		if c == ignoredConnection || !c.Connected {
			continue
		}
		c.sendMessage(message)
	}
}

func (node *Node) acceptedBlock(block *chain.BlockState) {
	fmt.Println("accepted block ", block.BlockNum)
	node.Dispatcher.broadcastBlock(block.Block, node)
}

