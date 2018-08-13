package network

import (
	"blockchain/chain"
	"blockchain/crypto"
	"blockchain/database"
	"time"
	"fmt"
	"math"
	"bytes"
	"blockchain/utils"
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
	ProducerWaterMarks map[chain.AccountName]uint32
	PendingBlockMode PendingBlockMode
	PersitentTransactions []*database.TransactionObject
	IrreversibleBlockTime time.Time
	lastSignedBlockTime time.Time
	startTime time.Time
	lastSignedBlockNum uint32
	timer *time.Timer
}

func NewProducerManager() *ProducerManager {
	pm := ProducerManager{}
	pm.Producers = []chain.AccountName{chain.DEFAULT_PRODUCER_NAME}
	pm.SignatureProviders = make(map[string]SignatureProviderType, 0)
	pm.SignatureProviders[chain.DEFAULT_PUBLIC_KEY] = makeKeySignatureProvider(chain.DEFAULT_PRIVATE_KEY)
	pm.timer = time.NewTimer(time.Duration(maxTime().UnixNano()))
	pm.ProducerWaterMarks = make(map[chain.AccountName]uint32, 0)
	pm.startTime = time.Now()
	pm.lastSignedBlockNum = 0
	return &pm
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
	pm.scheduleProductionLoop(node)
}

func maxTime() time.Time {
	return time.Unix(1<<63-62135596801, 999999999)
}

func (pm *ProducerManager) onIrreversibleBlock(lib *chain.SignedBlock) {
	pm.IrreversibleBlockTime = lib.Timestamp.ToTime()
}

func (pm *ProducerManager) scheduleProductionLoop(node *Node) {
	blockchain := node.BlockChain
	pm.timer.Stop()
	result := pm.startBlock(blockchain)
	defer pm.timer.Stop()
	if result == chain.Failed {
		fmt.Println("Failed to start a pending block, will try again later")
		pm.timer = time.AfterFunc(chain.BLOCK_INTERVAL_NS/10, func() {
			fmt.Println("schedule when failed")
			pm.scheduleProductionLoop(node)
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
				fmt.Println("produce block at ", time.Now().UnixNano())
				err := pm.produceBlock(node)
				if err != nil {
					blockchain.AbortBlock()
				}
				pm.scheduleProductionLoop(node)
			})
		} else if pm.PendingBlockMode == Speculating && len(pm.Producers) != 0 {
			wakeupTime := uint64(0)
			for _, producer := range pm.Producers {
				nextProducerBlockTime := pm.calculateNextBlockTime(blockchain, producer) // nanosecond
				if nextProducerBlockTime != 0 {
					producerWakeupTime := nextProducerBlockTime - chain.BLOCK_INTERVAL_NS
					if wakeupTime != 0 {
						wakeupTime = utils.MinUint64(wakeupTime, producerWakeupTime)
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
					pm.scheduleProductionLoop(node)
				})
			} else {
				fmt.Println("Speculative Block Created; Not Scheduling Speculative/Production, no local producers had valid wake up times")
			}
		} else {
			fmt.Println("Speculative Block Created")
		}
	}
}

func (pm *ProducerManager) calculateNextBlockTime(blockchain *database.BlockChain, producerName chain.AccountName) uint64 {
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
	minimumOffset := uint32(1)
	currentWaterMark, ok := pm.ProducerWaterMarks[producerName]
	if ok {
		if currentWaterMark > pbs.BlockNum {
			minimumOffset = currentWaterMark - pbs.BlockNum + 1
		}
	}
	minimumSlot := hbt.Slot + uint64(minimumOffset)
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

func (pm *ProducerManager) findProducer(name chain.AccountName) (chain.AccountName, error) {
	for _, producer := range pm.Producers {
		if producer == name {
			return producer, nil
		}
	}
	return "", fmt.Errorf("producer does not exist")
}

func (pm *ProducerManager) startBlock(blockchain *database.BlockChain) chain.BlockResult {
	//fmt.Println("start block")
	headBlockState := blockchain.Head
	now := uint64(time.Now().UnixNano())
	headBlockTime := uint64(blockchain.Head.Header.Timestamp.ToTime().UnixNano()) //nanosecond
	base := utils.MaxUint64(now, headBlockTime)
	minTimeToNextBlock := chain.BLOCK_INTERVAL_NS - (base % chain.BLOCK_INTERVAL_NS)
	blockTime := base + minTimeToNextBlock
	if (blockTime - minTimeToNextBlock) < (chain.BLOCK_INTERVAL_NS / 10) {
		blockTime += chain.BLOCK_INTERVAL_NS
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
				blocksToConfirm = utils.Min(math.MaxUint16, uint16(headBlockState.BlockNum - currentWaterMark))
			}
		}
	}
	blockchain.StartBlock(blockTime, blocksToConfirm, chain.Incomplete)
	pbs := blockchain.Pending.PendingBlockState
	if pbs != nil {
		if pm.PendingBlockMode == Producing &&
			!bytes.Equal(pbs.BlockSigningKey.Content, scheduledProducer.BlockSigningKey.Content){
			pm.PendingBlockMode = Speculating
		}
		return chain.Succeeded
	}
	return chain.Failed
}

func (pm *ProducerManager) produceBlock(node *Node) error {
	blockchain := node.BlockChain
	if pm.PendingBlockMode != Producing {
		return fmt.Errorf("called produce_block while not actually producing")
	}
	pbs := blockchain.Pending.PendingBlockState
	if pbs == nil {
		return fmt.Errorf("pending_block_state does not exist but it should, another plugin may have corrupted it")
	}
	signatureProvider, hasSP := pm.SignatureProviders[pbs.BlockSigningKey.String()]
	if !hasSP {
		return fmt.Errorf("Attempting to produce a block for which we don't have the private key")
	}
	blockchain.FinalizeBlock()
	signer := func(digest chain.SHA256Type) crypto.Signature {
		return signatureProvider(digest)
	}
	blockchain.SignBlock(signer)
	blockchain.CommitBlock(node.acceptedBlock, true)
	newBs := blockchain.Head
	pm.ProducerWaterMarks[newBs.Header.Producer] = blockchain.Head.BlockNum
	return nil
}

// return ms
func (pm *ProducerManager) getIrriversibleBlockAge() uint64 {
	now := time.Now()
	if now.UnixNano() < pm.IrreversibleBlockTime.UnixNano() {
		return 0
	} else {
		return uint64(now.UnixNano() - pm.IrreversibleBlockTime.UnixNano())
	}
}

//func (pm *ProducerManager) onBlock(blockState *chain.BlockState) {
//	if blockState.Header.Timestamp.ToTime().UnixNano() <= pm.lastSignedBlockTime.UnixNano() {
//		fmt.Println("received block time is invalid")
//		return
//	}
//	if blockState.Header.Timestamp.ToTime().UnixNano() <= pm.startTime.UnixNano() {
//		fmt.Println("received block time is invalid")
//		return
//	}
//	if blockState.BlockNum <= pm.lastSignedBlockNum {
//		fmt.Println("received block num is invalid")
//		return
//	}
//	bsActiveProducers := blockState.ActiveSchedule.Producers
//	activeProducers := []chain.AccountName{}
//	for _, p := range bsActiveProducers {
//		activeProducers = append(activeProducers, p.ProducerName)
//	}
//	intersect, ok := arrayOperations.Intersect(activeProducers, pm.Producers)
//	if !ok {
//		return
//	}
//	intersectProducers, ok := intersect.Interface().([]chain.AccountName)
//	if !ok {
//		return
//	}
//	for _, p := range intersectProducers {
//		if p != blockState.Header.Producer {
//			producerKey, err := findProducerName(p, bsActiveProducers)
//			if err == nil {
//				if signer, ok := pm.SignatureProviders[producerKey.BlockSigningKey.String()]; ok {
//					digest := blockState.Digest()
//					sig := signer(digest)
//					pm.lastSignedBlockTime = blockState.Header.Timestamp.ToTime()
//					pm.lastSignedBlockNum = blockState.BlockNum
//				}
//			}
//		}
//	}
//}

func (pm *ProducerManager) onIncomingBlock(signedBlock *chain.SignedBlock, node *Node) {
	blockchain := node.BlockChain
	blockId := signedBlock.Id()
	// if incoming block is included in the local blockchain, we don't need handle it
	if blockchain.FetchBlockById(blockId) != nil {
		return
	}
	// abort pending block. We move transactions of pending block to un-applied transactions
	blockchain.AbortBlock()
	blockchain.PushBlock(signedBlock, chain.Complete, node.acceptedBlock)
}

func findProducerName(name chain.AccountName, producers []chain.ProducerKey) (chain.ProducerKey, error) {
	for _, producer := range producers {
		if producer.ProducerName == name {
			return producer, nil
		}
	}
	return chain.ProducerKey{}, fmt.Errorf("producer does not exist")
}
