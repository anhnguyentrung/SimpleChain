package network

import (
	"gopkg.in/karalabe/cookiejar.v1/collections/deque"
	"fmt"
	"bytes"
	"blockchain/chain"
	"time"
)

type states uint8
const(
	Lib_Catchup states = iota
	Head_Catchup
	In_Sync
)

type SyncState struct {
	StartBlock uint32
	EndBlock uint32
	Last uint32
	StartTime time.Time
}

type SyncManager struct {
	syncKnownLibNum uint32
	syncLastRequestedNum uint32
	syncNextExpectedNum uint32
	syncReqSpan uint32
	source *Connection
	state states
	blocks *deque.Deque
}

func NewSyncManager(span uint32) *SyncManager {
	return &SyncManager{
		syncKnownLibNum:0,
		syncLastRequestedNum:0,
		syncNextExpectedNum:1,
		syncReqSpan:span,
		state:In_Sync,
	}
}

func (sm *SyncManager) stageToString(state states) string {
	switch state {
	case Lib_Catchup:
		return "lib catchup"
	case Head_Catchup:
		return "head catchup"
	case In_Sync:
		return "in sync"
	default:
		return "unknown"
	}
}

func (sm *SyncManager) setState(newState states) {
	if sm.state == newState {
		return
	}
	sm.state = newState
}

func (sm *SyncManager) isActive(c *Connection, node *Node) bool {
	if c!= nil && sm.state == Head_Catchup {
		nullId := chain.SHA256Type{}
		return !bytes.Equal(c.ForkHead[:], nullId[:]) && c.ForkHeadNum < node.BlockChain.Head.BlockNum
	}
	return sm.state != In_Sync
}

func (sm *SyncManager) requestNextChunk(c *Connection, node *Node) {
	headBlock := node.BlockChain.ForkDatabase.Head.BlockNum
	if headBlock < sm.syncLastRequestedNum && (sm.source != nil) && sm.source.Current() {
		return
	}
	if c.Current() {
		sm.source = c
	} else {
		if len(node.Conns) == 1 {
			if sm.source == nil {
				for k := range node.Conns {
					sm.source = node.Conns[k]
					break
				}
			}
		} else {
			if sm.source != nil {
				foundSource := false
				for _, conn := range node.Conns {
					if conn == sm.source {
						foundSource = true
						break
					}
				}
				if !foundSource {
					for _, conn := range node.Conns {
						if conn.Current() {
							sm.source = conn
							break
						}
					}
				}
			}
		}
	}

	if sm.source == nil || !sm.source.Current() {
		fmt.Println("Unable to continue syncing at this time")
		sm.syncKnownLibNum = node.BlockChain.LastIrreversibleBlockNum()
		sm.syncLastRequestedNum = 0
		sm.setState(In_Sync)
		return
	}
	if sm.syncLastRequestedNum != sm.syncKnownLibNum {
		var start uint32 = sm.syncNextExpectedNum
		var end uint32 = start + SyncReqSpan - 1
		if end > sm.syncKnownLibNum {
			end = sm.syncKnownLibNum
		}
		if end > 0 && end >= start {
			sm.requestSyncBlocks(c, start, end)
			sm.syncLastRequestedNum = end
		}
	}
}

func (sm *SyncManager) requestSyncBlocks(c *Connection, start, end uint32) {
	syncMessage := SyncRequestMessage{
		StartBlock: start,
		EndBlock: end,
	}
	msg := Message{
		Header: MessageHeader{
			Type:byte(SyncRequest),
			Length:0,
		},
		Content:syncMessage,
	}
	c.sendMessage(msg)
}

func (sm *SyncManager) resetLibNum(c *Connection, node *Node) {
	if sm.state == In_Sync {
		sm.source = nil
	}
	if c.Current() {
		if c.LastHandshakeReceived.LastIrreversibleBlockNum > sm.syncKnownLibNum {
			sm.syncKnownLibNum = c.LastHandshakeReceived.LastIrreversibleBlockNum
		}
	} else if c == sm.source {
		sm.syncLastRequestedNum = 0
		sm.requestNextChunk(c, node)
	}
}

func (sm *SyncManager) syncRequired(node *Node) bool {
	return sm.syncLastRequestedNum < sm.syncKnownLibNum || node.BlockChain.ForkDatabase.Head.BlockNum < sm.syncLastRequestedNum
}

func (sm *SyncManager) startSync(c *Connection, node *Node, target uint32) {
	if target > sm.syncKnownLibNum {
		sm.syncKnownLibNum = target
	}
	if !sm.syncRequired(node) {
		return
	}
	if sm.state == In_Sync {
		sm.setState(Lib_Catchup)
		sm.syncNextExpectedNum = node.BlockChain.LastIrreversibleBlockNum() + 1
	}
	sm.requestNextChunk(c, node)
}

func (sm *SyncManager) verifyCatchup(c *Connection, node *Node, num uint32, id chain.SHA256Type) {
	req := RequestMessage{}
	req.ReqBlocks.Mode = Catch_Up
	for _, cc := range node.Conns {
		if bytes.Equal(cc.ForkHead[:], id[:]) || cc.ForkHeadNum > num {
			req.ReqBlocks.Mode = None
			break
		}
	}
	if req.ReqBlocks.Mode == Catch_Up {
		c.ForkHead = id
		c.ForkHeadNum = num
		if sm.state == Lib_Catchup {
			return
		}
		sm.setState(Head_Catchup)
	} else {
		c.ForkHead = chain.SHA256Type{}
		c.ForkHeadNum = 0
	}
	req.ReqTrx.Mode = None
	msg := Message{
		Header: MessageHeader{
			Type:byte(Request),
			Length:0,
		},
		Content:req,
	}
	c.sendMessage(msg)
}

func (sm *SyncManager) ReceiveHanshake(message HandshakeMessage, c *Connection, node *Node) {
	libNum := node.BlockChain.LastIrreversibleBlockNum()
	peerLib := message.LastIrreversibleBlockNum
	sm.resetLibNum(c, node)
	c.Syncing = false
	head := node.BlockChain.ForkDatabase.Head.BlockNum
	headId := node.BlockChain.ForkDatabase.Head.Id
	if bytes.Equal(headId[:], message.HeadId[:]) {
		noticeMsg := NoticeMessage{}
		noticeMsg.KnownBlocks.Mode = None
		noticeMsg.KnownTrx.Mode = Catch_Up
		noticeMsg.KnownTrx.Pending = uint32(len(node.LocalTrxs))
		msg := Message{
			Header: MessageHeader{
				Type:byte(Notice),
				Length:0,
			},
			Content:noticeMsg,
		}
		c.sendMessage(msg)
		return
	}
	if head < peerLib {
		sm.startSync(c, node, peerLib)
		return
	}
	if libNum > message.HeadNum {
		if message.Generation > 1 {
			noticeMsg := NoticeMessage{}
			noticeMsg.KnownBlocks.Mode = Last_Irr_Catch_Up
			noticeMsg.KnownTrx.Mode = Last_Irr_Catch_Up
			noticeMsg.KnownTrx.Pending = libNum
			noticeMsg.KnownBlocks.Pending = head
			msg := Message{
				Header: MessageHeader{
					Type:byte(Notice),
					Length:0,
				},
				Content:noticeMsg,
			}
			c.sendMessage(msg)
			return
		}
	}
	if head <= message.HeadNum {
		sm.verifyCatchup(c, node, message.HeadNum, message.HeadId)
		return
	} else {
		if message.Generation > 1 {
			noticeMsg := NoticeMessage{}
			noticeMsg.KnownTrx.Mode = None
			noticeMsg.KnownBlocks.Mode = Catch_Up
			noticeMsg.KnownBlocks.Pending = head
			noticeMsg.KnownTrx.Ids = append(noticeMsg.KnownTrx.Ids, headId)
			msg := Message{
				Header: MessageHeader{
					Type:byte(Notice),
					Length:0,
				},
				Content:noticeMsg,
			}
			c.sendMessage(msg)
			return
		}
		c.Syncing = true
		return
	}
}

func (sm *SyncManager) receiveNotice(c *Connection, node *Node, message NoticeMessage) {
	if message.KnownBlocks.Mode == Catch_Up {
		if len(message.KnownBlocks.Ids) == 0 {
			fmt.Println("got a catch up with ids size = 0")
		} else {
			sm.verifyCatchup(c, node, message.KnownBlocks.Pending, message.KnownBlocks.Ids[len(message.KnownBlocks.Ids)-1])
		}
	} else {
		c.LastHandshakeReceived.LastIrreversibleBlockNum = message.KnownTrx.Pending
		sm.resetLibNum(c, node)
		sm.startSync(c, node, message.KnownBlocks.Pending)
	}
}

func (sm *SyncManager) receiveBlock(c *Connection, node *Node, blockId chain.SHA256Type, blockNum uint32) {
	fmt.Printf("Got block %d from %s\n", blockNum, c.PeerName())
	if sm.state == Lib_Catchup {
		if blockNum != sm.syncNextExpectedNum {
			fmt.Println("Expected block num %d but god %d\n", sm.syncNextExpectedNum, blockNum)
			return
		}
		sm.syncNextExpectedNum = blockNum + 1
	}
	if sm.state == Head_Catchup {
		sm.setState(In_Sync)
		sm.source = nil
		nullId := chain.SHA256Type{}
		for _, connection := range node.Conns {
			if bytes.Equal(nullId[:], connection.ForkHead[:]) {
				continue
			}
			if bytes.Equal(blockId[:], connection.ForkHead[:]) || connection.ForkHeadNum < blockNum {
				connection.ForkHead = nullId
				connection.ForkHeadNum = 0
			} else {
				sm.setState(Head_Catchup)
			}
		}
	} else if sm.state == Lib_Catchup {
		if blockNum == sm.syncKnownLibNum {
			sm.setState(In_Sync)
		} else if blockNum == sm.syncLastRequestedNum {
			sm.requestNextChunk(c, node)
		}
	}
}

func (sm *SyncManager) sendHanshakes(node *Node) {
	for _,c := range node.Conns {
		if c.Current() {
			node.sendHandshake(c)
		}
	}
}
