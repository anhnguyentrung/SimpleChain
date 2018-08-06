package network

import (
	"blockchain/chain"
	"fmt"
	"time"
)

type BlockRequest struct {
	Id chain.SHA256Type
	LocalRetry bool
}

type BlockOrigin struct {
	Id chain.SHA256Type
	Origin *Connection
}

type TransactionOrigin struct {
	Id chain.SHA256Type
	Origin *Connection
}

type PeerBlockState struct {
	Id chain.SHA256Type
	BlockNum uint32
	IsKnown bool
	IsNoticed bool
	RequestedTime time.Time
}

type DispatchManager struct {
	JustSendItMax uint32
	RequestBlocks []BlockRequest
	RequestTrx []chain.SHA256Type
	ReceivedBlocks []BlockOrigin
	ReceivedTransactions []TransactionOrigin
}

func (dm *DispatchManager) receiveNotice(c *Connection, node *Node, message NoticeMessage, generated bool) {
	req := RequestMessage{}
	req.ReqTrx.Mode = None
	req.ReqBlocks.Mode = None
	sendReq := false
	blockchain := node.BlockChain
	if message.KnownTrx.Mode == Normal {
		req.ReqTrx.Mode = Normal
		req.ReqTrx.Pending = 0
		for _, id := range message.KnownTrx.Ids {
			trx := node.findLocalTrx(id)
			if trx == nil {
				fmt.Println("did not find local transaction")
				trxState := chain.TransactionState{
					Id: id,
					IsKnownByPeer: true,
					IsNoticedByPeer: true,
					BlockNum: 0,
					Expires: uint64(time.Now().Unix()) + 120,
					RequestedTime: time.Now(),
				}
				c.TransactionStates = append(c.TransactionStates, trxState)
				req.ReqTrx.Ids = append(req.ReqTrx.Ids, id)
				dm.RequestTrx = append(dm.RequestTrx, id)
			} else {
				fmt.Println("found local transaction")
			}
		}
		sendReq = !(len(req.ReqTrx.Ids) == 0)
	} else if message.KnownTrx.Mode != None {
		return
	}
	if message.KnownBlocks.Mode == Normal {
		req.ReqBlocks.Mode = Normal
		for _, blockId := range message.KnownBlocks.Ids {
			entry := PeerBlockState{
				Id: blockId,
				BlockNum: 0,
				IsKnown: true,
				IsNoticed: true,
				RequestedTime: time.Now(),
			}
			block := blockchain.FetchBlockById(blockId)
			if block != nil {
				entry.BlockNum = block.BlockNum()
			} else {
				sendReq = true
				req.ReqBlocks.Ids = append(req.ReqBlocks.Ids, blockId)
				blockReq := BlockRequest{
					Id: blockId,
					LocalRetry: generated,
				}
				dm.RequestBlocks = append(dm.RequestBlocks, blockReq)
				entry.RequestedTime = time.Now()
				c.addPeerBlock(&entry)
			}
		}
	} else if message.KnownBlocks.Mode != None {
		return
	}
	if sendReq {
		msg := Message{
			Header: MessageHeader{
				Type:Request,
				Length:0,
			},
			Content:req,
		}
		c.sendMessage(msg)
		c.LastRequest = req
	}
}
