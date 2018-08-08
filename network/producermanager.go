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
type SignatureProviderType func(hash chain.SHA256Type) crypto.Signature

type ProducerManager struct {
	SignatureProviders map[string]SignatureProviderType
	Producers []chain.AccountName
	ProducerWaterMarks map[chain.AccountName]uint64
	PendingBlockMode PendingBlockMode
	PersitentTransactions []*database.TransactionObject
	IrreversibleBlockTime time.Time
	timer *time.Timer
}

func NewProducerManager() ProducerManager {
	pm := ProducerManager{}
	pm.Producers = []chain.AccountName{"default"}
	pm.SignatureProviders[chain.DEFAULT_PUBLIC_KEY] = makeKeySignatureProvider(chain.DEFAULT_PRIVATE_KEY)
	pm.timer = time.NewTimer(time.Duration(maxTime().UnixNano()))
	return pm
}

func makeKeySignatureProvider(wif string) SignatureProviderType {
	priv, _ := crypto.NewPrivateKey(wif)
	return func(hash chain.SHA256Type) crypto.Signature {
		sig, _ := priv.Sign(hash[:])
		return sig
	}
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
	pm.timer.Stop()
	result := pm.startBlock(blockchain)
	defer pm.timer.Stop()
	if result == chain.Failed {
		fmt.Println("Failed to start a pending block, will try again later")
		pm.timer = time.AfterFunc(time.Millisecond * chain.BLOCK_INTERVAL_MS/10, func() {
			fmt.Println("schedule when failed")
			pm.scheduleProductionLoop(blockchain)
		})
	} else {
		if pm.PendingBlockMode == Producing {
			duration := time.Duration(0)
			if result == chain.Succeeded {
				expiryTime := blockchain.Pending.PendingBlockState.Header.Timestamp.ToTime().UnixNano()
				now := time.Now().UnixNano()
				duration = time.Duration(expiryTime - now)
			}
			pm.timer = time.AfterFunc(duration, func() {
				fmt.Println("schedule when failed")
				pm.scheduleProductionLoop(blockchain)
			})
		} else if pm.PendingBlockMode == Speculating && len(pm.Producers) != 0 {
			wakeupTime := uint64(0)
			for _, producer := range pm.Producers {
				nextProducerBlockTime := pm.calculateNextBlockTime(blockchain, producer) // nanosecond
				if nextProducerBlockTime != 0 {
					producerWakeupTime := nextProducerBlockTime - chain.BLOCK_INTERVAL_MS * uint64(time.Millisecond)
					if wakeupTime != 0 {
						wakeupTime = MinUint64(wakeupTime, producerWakeupTime)
					} else {
						wakeupTime = producerWakeupTime
					}
				}
			}
			if wakeupTime != 0 {
				expiryTime := int64(wakeupTime)
				fmt.Println("Specualtive Block Created; Scheduling Speculative/Production Change at ", expiryTime)
				now := time.Now().UnixNano()
				duration := time.Duration(expiryTime - now)
				pm.timer = time.AfterFunc(duration, func() {
					fmt.Println("schedule when failed")
					pm.scheduleProductionLoop(blockchain)
				})
			} else {
				fmt.Println("Speculative Block Created; Not Scheduling Speculative/Production, no local producers had valid wake up times")
			}
		} else {
			fmt.Println("Speculative Block Created")
		}
	}
}

func MinUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func (pm *ProducerManager) calculateNextBlockTime(blockchain chain.BlockChain, producerName chain.AccountName) uint64 {
	pbs := blockchain.Pending.PendingBlockState
	activeProducers := pbs.ActiveSchedule.Producers
	hbt := pbs.Header.Timestamp
	foundIndex := -1
	for i, producer := range activeProducers {
		if producer.ProducerName == producerName {
			foundIndex = i
			break
		}
	}
	if foundIndex == -1 {
		return 0
	}
	minimumOffset := uint64(1)
	currentWaterMark, ok := pm.ProducerWaterMarks[producerName]
	if ok {
		if currentWaterMark > pbs.BlockNum {
			minimumOffset = currentWaterMark - pbs.BlockNum + 1
		}
	}
	minimumSlot := hbt.Slot + minimumOffset
	minimumSlotProducerIndex := (minimumSlot % (uint64(len(activeProducers)) * chain.PRODUCER_REPETITION)) / chain.PRODUCER_REPETITION
	producerIndex := uint64(foundIndex)
	if producerIndex == minimumSlotProducerIndex {
		blockTs := chain.NewBlockTimeStamp()
		blockTs.Slot = minimumSlot
		return uint64(blockTs.ToTime().UnixNano())
	} else {
		producerDistance := producerIndex - minimumSlotProducerIndex
		if producerDistance > producerIndex {
			producerDistance += uint64(len(activeProducers))
		}
		firstMinimumProducerSlot := minimumSlot - (minimumSlot % chain.PRODUCER_REPETITION)
		nextBlockSlot := firstMinimumProducerSlot + producerDistance * chain.PRODUCER_REPETITION
		blockTs := chain.NewBlockTimeStamp()
		blockTs.Slot = nextBlockSlot
		return uint64(blockTs.ToTime().UnixNano())
	}
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
	blockTs := chain.NewBlockTimeStamp()
	blockTs.SetTime(blockTime)
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
	if pbs == nil {
		log.Fatal("pending_block_state does not exist but it should, another plugin may have corrupted it")
	}
	signatureProvider, hasSP := pm.SignatureProviders[pbs.BlockSigningKey.String()]
	if !hasSP {
		log.Fatal("Attempting to produce a block for which we don't have the private key")
	}
	blockchain.FinalizeBlock()
	signer := func(digest chain.SHA256Type) crypto.Signature {
		return signatureProvider(digest)
	}
	blockchain.SignBlock(signer)
	blockchain.CommitBlock()
	newBs := blockchain.Head
	pm.ProducerWaterMarks[newBs.Header.Producer] = blockchain.Head.BlockNum
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
