package database

import (
	bytes2 "bytes"
	"log"
	"fmt"
	"blockchain/crypto"
	"crypto/sha256"
	"blockchain/chain"
	"blockchain/utils"
)

type BlockChainConfig struct {
	Genesis chain.GenesisState
}

func NewBlockChainConfig() BlockChainConfig {
	return BlockChainConfig{
		Genesis: chain.NewGenesisState(),
	}
}

type PendingState struct {
	PendingBlockState *chain.BlockState
	BlockStatus chain.BlockStatus
}

type BlockChain struct {
	Config BlockChainConfig
	DB Database
	ReversibleBlocks []*chain.SignedBlock
	Blog BlockLog
	Pending *PendingState
	Head *chain.BlockState
	ForkDatabase ForkDatabase
	ChainId chain.SHA256Type
	Replaying bool
	UnAppliedTransactions map[chain.SHA256Type]*chain.TransactionMetaData
}

func NewBlockChain() *BlockChain {
	bc := BlockChain{}
	bc.Config = NewBlockChainConfig()
	bc.initializeForkDatabase()
	bc.Blog = BlockLog{}
	bc.Blog.Blocks = []*chain.SignedBlock{bc.Head.Block}
	bc.Blog.Head = bc.Blog.Blocks[len(bc.Blog.Blocks) - 1]
	bc.Blog.GenesisWrittenToBlockLog = true
	bc.Blog.HeadId = bc.Blog.Head.Id()
	bc.ChainId = bc.Config.Genesis.ComputeChainId()
	bc.Pending = &PendingState{}
	bc.Pending.PendingBlockState = &chain.BlockState{}
	return &bc
}

func (bc *BlockChain) initializeForkDatabase() {
	fmt.Println("Initializing new blockchain with genesis state")
	pub, _ := crypto.NewPublicKey(bc.Config.Genesis.InitialKey)
	producerKey := chain.ProducerKey{
		chain.DEFAULT_PRODUCER_NAME,
		pub,
	}
	initialSchedule := chain.ProducerScheduleType{
		Version: 0,
		Producers: []chain.ProducerKey{producerKey},
	}
	genHeader := chain.NewBlockHeaderState()
	genHeader.ActiveSchedule = initialSchedule
	genHeader.PendingSchedule = initialSchedule
	initialScheduleBytes, _ := chain.MarshalBinary(initialSchedule)
	genHeader.PendingScheduleHash = sha256.Sum256(initialScheduleBytes)
	genHeader.Header.Timestamp = bc.Config.Genesis.InitialTimestamp
	genHeader.Id = genHeader.Header.Id()
	genHeader.BlockNum = genHeader.Header.BlockNum()
	bc.Head = &chain.BlockState{}
	bc.Head.BlockHeaderState = genHeader
	bc.Head.Block = &chain.SignedBlock{}
	bc.Head.Block.SignedBlockHeader = genHeader.Header
	bc.ForkDatabase = ForkDatabase{}
	bc.ForkDatabase.BlockStates = []*chain.BlockState{}
	bc.ForkDatabase.Add(bc.Head)
	bc.initializeDatabase()
}

func (bc *BlockChain) initializeDatabase() {
	bc.DB = Database{}
	bc.DB.GPO = &GlobalPropertyBlock{}
	bc.DB.GPO.Configuation = bc.Config.Genesis.InitialConfiguration
	taposBlockSumary := BlockSummaryObject{}
	taposBlockSumary.BlockId = bc.Head.Id
	bc.DB.BlockSummaryObjects = []*BlockSummaryObject{&taposBlockSumary}
}

func (bc *BlockChain) LastIrreversibleBlockNum() uint32 {
	return utils.MaxUint32(bc.Head.DPOSIrreversibleBlockNum, bc.Head.BFTIrreversibleBlockNum)
}

func (bc *BlockChain) LastIrreversibleBlockId() *chain.SHA256Type {
	libNum := bc.LastIrreversibleBlockNum()
	taposBlockSummary := bc.DB.GetBlockSummaryObject(libNum)
	if taposBlockSummary != nil {
		return &taposBlockSummary.BlockId
	}
	block := bc.FetchBlockByNum(libNum)
	if block != nil {
		blockId := block.Id()
		return &blockId
	}
	return nil
}

func (bc *BlockChain) GetBlockIdForNum(blockNum uint32) chain.SHA256Type {
	blockState := bc.ForkDatabase.GetBlockInCurrentChainIdNum(blockNum)
	if blockState != nil {
		return blockState.Id
	}
	signedBlock := bc.Blog.ReadBlockByNum(blockNum)
	return signedBlock.Id()
}

func (bc *BlockChain) FetchBlockById(id chain.SHA256Type) *chain.SignedBlock {
	state := bc.ForkDatabase.GetBlock(id)
	if state != nil {
		return state.Block
	}
	block := bc.FetchBlockByNum(chain.NumFromId(id))
	if block != nil {
		blockId := block.Id()
		if bytes2.Equal(blockId[:], id[:]) {
			return block
		}
	}
	return nil
}

func (bc *BlockChain) FetchBlockByNum(num uint32) *chain.SignedBlock {
	blockState := bc.ForkDatabase.GetBlockInCurrentChainIdNum(num)
	if blockState != nil {
		return blockState.Block
	}
	signedBlock := bc.Blog.ReadBlockByNum(num)
	return signedBlock
}

func (bc *BlockChain) AbortBlock() {
	if bc.Pending != nil {
		for _, trx := range bc.Pending.PendingBlockState.Trxs {
			bc.UnAppliedTransactions[trx.Signed] = trx
		}
		bc.Pending = nil
	}
}

func (bc *BlockChain) StartBlock(when uint64, confirmBlockCount uint16, s chain.BlockStatus) {
	if bc.Pending == nil {
		bc.Pending = &PendingState{}
	}
	// generate pending block
	bc.Pending.BlockStatus = s
	//fmt.Println("old confirm count: ", bc.Head.BlockHeaderState.ConfirmCount)
	bc.Pending.PendingBlockState = chain.NewBlockState(bc.Head.BlockHeaderState, when)
	//fmt.Println("next confirm count: ", bc.Pending.PendingBlockState.ConfirmCount)
	bc.Pending.PendingBlockState.InCurrentChain = true
	bc.Pending.PendingBlockState.SetConfirmed(confirmBlockCount)
	bc.Pending.PendingBlockState.MaybePromotePending()
	//gpo := bc.DB.GPO
	//if gpo.ProposedScheduleBlockNum != nil &&
	//	*gpo.ProposedScheduleBlockNum <= bc.Pending.PendingBlockState.DPOSIrreversibleBlockNum &&
	//	len(bc.Pending.PendingBlockState.PendingSchedule.Producers) == 0 &&
	//	!wasPendingPromoted {
	//		bc.Pending.PendingBlockState.SetNewProducer(*gpo.ProposedSchedule)
	//		bc.DB.GPO.ProposedSchedule = nil
	//		bc.DB.GPO.ProposedScheduleBlockNum = nil
	//}
	fmt.Println("start block: producers ", bc.Pending.PendingBlockState.GetScheduledProducer(when).ProducerName)
	// remove expired transaction in database
	bc.ClearExpiredInputTransaction()
}

func removeTransactionObject(transactionObjects []*TransactionObject, index int) []*TransactionObject {
	return append(transactionObjects[:index], transactionObjects[index+1:]...)
}

func (bc *BlockChain) ClearExpiredInputTransaction() {
	expriedIndexes := make([]int, 0)
	now := bc.Pending.PendingBlockState.Header.Timestamp.ToTime()
	for i, obj := range bc.DB.TransactionObjects {
		if now.After(obj.Expiration) {
			expriedIndexes = append(expriedIndexes, i)
		}
	}
	for _, index := range expriedIndexes{
		bc.DB.TransactionObjects = removeTransactionObject(bc.DB.TransactionObjects, index)
	}
}

func (bc *BlockChain) FinalizeBlock() {
	//fmt.Println("finalize block")
	if bc.Pending == nil {
		log.Fatal("it is not valid to finalize when there is no pending block")
	}
	// the part relating to transaction will be added later
	p := bc.Pending.PendingBlockState
	p.Id = p.Header.Id()
	bc.CreateBlockSumary(p.Id)
}

func (bc *BlockChain) CreateBlockSumary(id chain.SHA256Type) {
	blockNum := chain.NumFromId(id)
	bso := bc.DB.FindBlockSummaryObject(blockNum)
	bso.BlockId = id
}

func (bc *BlockChain) CommitBlock(acceptedBlock func(bs *chain.BlockState), addToForkFB bool) {
	//fmt.Println("commit block")
	// add block which this producer has produced to fork database
	if addToForkFB {
		bc.Pending.PendingBlockState.Validated = true
		newBsp := bc.ForkDatabase.Add(bc.Pending.PendingBlockState)
		bc.Head = bc.ForkDatabase.Head
		if newBsp != bc.Head {
			log.Fatal("committed block did not become the new head in fork database")
		}
	}
	if !bc.Replaying {
		// append block produced from other producers to reversible blocks
		bc.ReversibleBlocks = append(bc.ReversibleBlocks, bc.Pending.PendingBlockState.Block)
	}
	//fmt.Println("block id: ", bc.Pending.PendingBlockState.BlockNum)
	acceptedBlock(bc.Pending.PendingBlockState)
}

func (bc *BlockChain) SignBlock(signer chain.SignerCallBack) {
	p := bc.Pending.PendingBlockState
	p.Sign(signer)
	p.Block.SignedBlockHeader = p.Header
}

func (bc *BlockChain) PushBlock(signedBlock *chain.SignedBlock, blockStatus chain.BlockStatus, acceptedBlock func(bs *chain.BlockState)) {
	if bc.Pending != nil {
		log.Fatal("pending block should be nil")
	}
	if signedBlock == nil {
		log.Fatal("block should not be nil")
	}
	if blockStatus == chain.Incomplete {
		log.Fatal("invalid block status")
	}
	trust := (blockStatus == chain.Irreversible || blockStatus == chain.Validated)
	var initialSchedule chain.ProducerScheduleType
	previousBlock := bc.ForkDatabase.GetBlock(signedBlock.Previous)
	if previousBlock != nil && previousBlock.BlockHeaderState.BlockNum == 1 {
		initialSchedule = previousBlock.BlockHeaderState.ActiveSchedule
		previousBlock.BlockHeaderState.ActiveSchedule = *bc.DB.GPO.ProposedSchedule
		previousBlock.BlockHeaderState.PendingSchedule = *bc.DB.GPO.ProposedSchedule
	}
	// add signed block to fork database
	newBlockState, _ := bc.ForkDatabase.AddSignedBlock(signedBlock, trust)
	if previousBlock != nil && previousBlock.BlockHeaderState.BlockNum == 1 {
		previousBlock.BlockHeaderState.ActiveSchedule = initialSchedule
		previousBlock.BlockHeaderState.PendingSchedule = initialSchedule
	}
	if newBlockState != nil {
		fmt.Println("accepted block ", newBlockState.BlockNum)
		bc.switchForks(blockStatus, acceptedBlock)
	}
}

func (bc *BlockChain) switchForks(blockStatus chain.BlockStatus, acceptedBlock func(bs *chain.BlockState)) {
	newHead := bc.ForkDatabase.Head
	// if new head's previous block and blockchain's head have the same id, they are in the same branch.
	// otherwise, if new head id is different from blockchain's head id, we have to switch between branches
	if bytes2.Equal(newHead.Header.Previous[:], bc.Head.Id[:]) {
		bc.applyBlock(newHead.Block, blockStatus, acceptedBlock)
		bc.ForkDatabase.MarkInCurrentChain(newHead, true)
		bc.ForkDatabase.SetValidity(newHead, true)
		bc.Head = newHead
	} else if !bytes2.Equal(newHead.Id[:], bc.Head.Id[:]) {
		fmt.Printf("switch forks from %d to %d%\n", bc.Head.BlockNum, newHead.BlockNum)
		// get blocks on two branches
		branches := bc.ForkDatabase.FectchBranchFrom(newHead.Id, bc.Head.Id)
		for _, bs := range branches.Second.([]*chain.BlockState) {
			bc.ForkDatabase.MarkInCurrentChain(bs, false)
			bc.popBlock()
		}
		secondBranchHeadId := branches.Second.([]*chain.BlockState)[len(branches.Second.([]*chain.BlockState))-1].Header.Previous
		if !bytes2.Equal(bc.Head.Id[:], secondBranchHeadId[:]) {
			log.Fatal("loss sync")
		}
		for _, bs := range branches.First.([]*chain.BlockState) {
			var bStatus chain.BlockStatus = chain.Validated
			if !bs.Validated {
				bStatus = chain.Complete
			}
			bc.applyBlock(bs.Block, bStatus, acceptedBlock)
			bc.Head = bs
			bc.ForkDatabase.MarkInCurrentChain(bs, true)
			bs.Validated = true
		}
	}
}

func (bc *BlockChain) applyBlock(signedBlock *chain.SignedBlock, blockStatus chain.BlockStatus, acceptedBlock func(bs *chain.BlockState)) {
	bc.StartBlock(uint64(signedBlock.Timestamp.ToTime().UnixNano()), signedBlock.Confirmed, blockStatus)
	bc.FinalizeBlock()
	bc.Pending.PendingBlockState.Header.ProducerSignature = signedBlock.ProducerSignature
	bc.Pending.PendingBlockState.Block.SignedBlockHeader = bc.Pending.PendingBlockState.Header
	bc.CommitBlock(acceptedBlock, false)
}

func (bc *BlockChain) popBlock() {
	previousBlock := bc.ForkDatabase.GetBlock(bc.Head.Header.Previous)
	if previousBlock == nil {
		log.Fatal("don't pop last block")
	}
	// remove the block from reversible blocks
	for index, rbo := range bc.ReversibleBlocks {
		// find by block num
		if rbo.BlockNum() == bc.Head.BlockNum {
			bc.ReversibleBlocks = append(bc.ReversibleBlocks[:index], bc.ReversibleBlocks[index+1:]...)
			break
		}
	}
	// move transactions in the block to un-applied transactions
	for _, trx := range bc.Head.Trxs {
		bc.UnAppliedTransactions[trx.Signed] = trx
	}
	// assign previous block to head
	bc.Head = previousBlock
}

func compareTwoProducers(p1 []chain.ProducerKey, p2 []chain.ProducerKey) bool {
	if len(p1) != len(p2) {
		return false
	}
	for i, v := range p1 {
		if v.ProducerName != p2[i].ProducerName {
			return false
		}
		if v.BlockSigningKey.String() != p2[i].BlockSigningKey.String() {
			return false
		}
	}
	return true
}

func (bc *BlockChain) SetProposedProducers(producers []chain.ProducerKey) int64 {
	gpo := bc.DB.GPO
	currentBlockNum := bc.Head.BlockNum + 1
	if gpo.ProposedScheduleBlockNum != nil {
		if *gpo.ProposedScheduleBlockNum != currentBlockNum {
			fmt.Println("there is already proposed producer schedule in previous block, wait for it to become pending")
			return -1
		}
		if compareTwoProducers(producers, gpo.ProposedSchedule.Producers) {
			fmt.Println("the proposed producer schedule does not change")
			return -1
		}
	}
	var schedule *chain.ProducerScheduleType = nil
	if len(bc.Pending.PendingBlockState.PendingSchedule.Producers) == 0 {
		schedule = &bc.Pending.PendingBlockState.ActiveSchedule
		//schedule.Producers = schedule.Producers
		schedule.Version = schedule.Version + 1
	} else {
		schedule = &bc.Pending.PendingBlockState.PendingSchedule
		//schedule.Producers = schedule.Producers
		schedule.Version = schedule.Version + 1
	}
	if compareTwoProducers(producers, schedule.Producers) {
		fmt.Println("the proposed producer schedule does not change")
		return -1
	}
	schedule.Producers = producers
	version := schedule.Version
	gpo.ProposedScheduleBlockNum = &currentBlockNum
	gpo.ProposedSchedule = schedule
	return int64(version)
}