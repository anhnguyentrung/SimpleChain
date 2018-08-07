package network

import (
	"blockchain/chain"
	"blockchain/crypto"
	"blockchain/database"
	"time"
	"fmt"
	"math"
	"bytes"
	"log"
)

type PendingBlockMode uint8

const (
	Producing PendingBlockMode = iota
	Speculating
)
type SignatureProviderType interface {
	Sign(hash chain.SHA256Type) crypto.Signature
}

type ProducerManager struct {
	SignatureProviders map[string]SignatureProviderType
	Producers []chain.AccountName
	ProducerWaterMarks map[chain.AccountName]uint32
	PendingBlockMode PendingBlockMode
	PersitentTransactions []*database.TransactionObject
	IrreversibleBlockTime time.Time
}

func NewProducerManager() ProducerManager {
	return ProducerManager{}
}

func (pm ProducerManager) isProducerKey(pub crypto.PublicKey) bool {
	if _, ok := pm.SignatureProviders[pub.String()]; ok {
		return true
	}
	return false
}

func (pm *ProducerManager) Startup(node *Node) {
	blockchain := node.BlockChain
	libNum := blockchain.LastIrreversibleBlockNum()
	lib := blockchain.FetchBlockByNum(libNum)
	if lib != nil {
		pm.onIrreversibleBlock(lib)
	} else {
		pm.IrreversibleBlockTime = maxTime()
	}
	if len(pm.Producers) > 0 {
		fmt.Printf("launching block production for %d producer at %s", len(pm.Producers), time.Now().String())
	}

}

func maxTime() time.Time {
	return time.Unix(1<<63-62135596801, 999999999)
}

func (pm *ProducerManager) onIrreversibleBlock(lib *chain.SignedBlock) {
	pm.IrreversibleBlockTime = lib.Timestamp.ToTime()
}

func (pm *ProducerManager) scheduleProductionLoop(blockchain chain.BlockChain) {

}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func (pm *ProducerManager) findProducer(name chain.AccountName) (chain.AccountName, error) {
	for _, producer := range pm.Producers {
		if producer == name {
			return producer, nil
		}
	}
	return "", fmt.Errorf("producer does not exist")
}

func (pm *ProducerManager) startBlock(blockchain chain.BlockChain) chain.BlockResult {
	headBlockState := blockchain.Head
	now := uint64(time.Now().UnixNano() / int64(time.Millisecond)) //ms
	headBlockTime := uint64(blockchain.Head.Header.Timestamp.ToTime().UnixNano() / int64(time.Millisecond)) //ms
	base := max(now, headBlockTime)
	minTimeToNextBlock := chain.BLOCK_INTERVAL_MS - (base % chain.BLOCK_INTERVAL_MS)
	blockTime := base + minTimeToNextBlock

	if (blockTime - minTimeToNextBlock) < (chain.BLOCK_INTERVAL_MS / 10) {
		blockTime += chain.BLOCK_INTERVAL_MS
	}
	pm.PendingBlockMode = Producing
	scheduledProducer := headBlockState.GetScheduledProducer(blockTime)
	currentWaterMark, hasWaterMark := pm.ProducerWaterMarks[scheduledProducer.ProducerName]
	_, hasSP := pm.SignatureProviders[scheduledProducer.BlockSigningKey.String()]
	_, err := pm.findProducer(scheduledProducer.ProducerName)
	if err != nil {
		pm.PendingBlockMode = Speculating
	}
	if !hasSP {
		pm.PendingBlockMode = Speculating
	}
	if pm.PendingBlockMode == Producing {
		if hasWaterMark {
			if currentWaterMark > headBlockState.BlockNum + 1 {
				fmt.Printf("Not producing block becuase %s signed a BFT confirmation " +
					"or block at a higher block number %d than current fork's head", scheduledProducer.ProducerName, currentWaterMark)
				pm.PendingBlockMode = Speculating
			}
		}
	}
	var blocksToConfirm uint16 = 0
	if pm.PendingBlockMode == Producing {
		if hasWaterMark {
			if currentWaterMark < headBlockState.BlockNum {
				blocksToConfirm = min(math.MaxUint16, uint16(headBlockState.BlockNum - currentWaterMark))
			}
		}
	}
	blockchain.StartBlock(blockTime, blocksToConfirm, chain.Incomplete)
	pbs := blockchain.PendingBlocKState()
	if pbs != nil {
		if pm.PendingBlockMode == Producing &&
			!bytes.Equal(pbs.BlockSigningKey.Content, scheduledProducer.BlockSigningKey.Content){
			pm.PendingBlockMode = Speculating
		}
		return chain.Succeeded
	}
	return chain.Failed
}

func (pm *ProducerManager) produceBlock(blockchain chain.BlockChain) {
	if pm.PendingBlockMode != Producing {
		log.Fatal("called produce_block while not actually producing")
	}
	pbs := blockchain.PendingBlocKState()
	hbs := blockchain.Head
	if pbs == nil {
		log.Fatal("pending_block_state does not exist but it should, another plugin may have corrupted it")
	}
	signatureProvider, hasSP := pm.SignatureProviders[pbs.BlockSigningKey.String()]
	if !hasSP {
		log.Fatal("Attempting to produce a block for which we don't have the private key")
	}
	blockchain.FinalizeBlock()
	blockchain.CommitBlock()
}

func min(a, b uint16) uint16 {
	if a < b {
		return  a
	}
	return b
}

// return ms
func (pm *ProducerManager) getIrriversibleBlockAge() uint64 {
	now := time.Now()
	if now.UnixNano() < pm.IrreversibleBlockTime.UnixNano() {
		return 0
	} else {
		return uint64((now.UnixNano() - pm.IrreversibleBlockTime.UnixNano()) / int64(time.Millisecond))
	}
}
