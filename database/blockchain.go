package database

import (
	bytes2 "bytes"
	"log"
	"fmt"
	"blockchain/crypto"
	"crypto/sha256"
	"blockchain/chain"
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
	ReversibleBlocks []*ReversibleBlockObject
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

func max(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
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
	bc.ForkDatabase.BlockStates = []*chain.BlockState{bc.Head}
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
	return max(bc.Head.DPOSIrreversibleBlockNum, bc.Head.BFTIrreversibleBlockNum)
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
	blockId := block.Id()
	if block != nil && bytes2.Equal(blockId[:], id[:]) {
		return block
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
		log.Fatal("pending block should be not nil")
	}
	// generate pending block
	bc.Pending.BlockStatus = s
	//fmt.Println("old block id: ", bc.Pending.PendingBlockState.BlockNum)
	bc.Pending.PendingBlockState = chain.NewBlockState(bc.Head.BlockHeaderState, when)
	//fmt.Println("next block id: ", bc.Pending.PendingBlockState.BlockNum)
	bc.Pending.PendingBlockState.InCurrentChain = true
	bc.Pending.PendingBlockState.SetConfirmed(confirmBlockCount)
	wasPendingPromoted := bc.Pending.PendingBlockState.MaybePromotePending()
	gpo := bc.DB.GPO
	if gpo.ProposedScheduleBlockNum != nil &&
		*gpo.ProposedScheduleBlockNum <= bc.Pending.PendingBlockState.DPOSIrreversibleBlockNum &&
		len(bc.Pending.PendingBlockState.PendingSchedule.Producers) == 0 &&
		!wasPendingPromoted {
			bc.Pending.PendingBlockState.SetNewProducer(gpo.ProposedSchedule.ProducerSchedulerType())
			bc.DB.GPO.ProposedSchedule = nil
			bc.DB.GPO.ProposedScheduleBlockNum = nil
	}
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

func (bc *BlockChain) CommitBlock() {
	//fmt.Println("commit block")
	bc.Pending.PendingBlockState.Validated = true
	newBsp := bc.ForkDatabase.Add(bc.Pending.PendingBlockState)
	bc.Head = bc.ForkDatabase.Head
	if newBsp != bc.Head {
		log.Fatal("committed block did not become the new head in fork database")
	}
	if !bc.Replaying {
		ubo := ReversibleBlockObject{}
		ubo.BlockNum = bc.Pending.PendingBlockState.BlockNum
		ubo.SetBlock(bc.Pending.PendingBlockState.Block)
	}
	//fmt.Println("block id: ", bc.Pending.PendingBlockState.BlockNum)
	//accept block. Use invoked method later
	// TODO
}

func (bc *BlockChain) SignBlock(signer chain.SignerCallBack) {
	p := bc.Pending.PendingBlockState
	p.Sign(signer)
	p.Block.SignedBlockHeader = p.Header
}