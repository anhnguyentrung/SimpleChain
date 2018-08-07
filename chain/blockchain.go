package chain

import (
	"blockchain/database"
	bytes2 "bytes"
	"log"
)

type BlockChainConfig struct {
	BlocksDir string
	StateDir string
	StateSize uint64
	ReversibleCacheSize uint64
	Genesis GenesisState
}

type PendingState struct {
	PendingBlockState *BlockState
	Actions []ActionReceipt
	BlockStatus BlockStatus
}

type BlockChain struct {
	Config BlockChainConfig
	DB database.Database
	ReversibleBlocks database.Database
	Blog database.BlockLog
	Pending *PendingState
	Head *BlockState
	ForkDatabase database.ForkDatabase
	Authorization AuthorizationManager
	ChainId SHA256Type
	Replaying bool
	UnAppliedTransactions map[SHA256Type]*TransactionMetaData
}

func NewBlockChain() BlockChain {
	return BlockChain{}
}

func max(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

func (bc *BlockChain) LastIrreversibleBlockNum() uint32 {
	return max(bc.Head.DPOSIrreversibleBlockNum, bc.Head.BFTIrreversibleBlockNum)
}

func (bc *BlockChain) LastIrreversibleBlockId() *SHA256Type {
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

func (bc *BlockChain) GetBlockIdForNum(blockNum uint32) SHA256Type {
	blockState := bc.ForkDatabase.GetBlockInCurrentChainIdNum(blockNum)
	if blockState != nil {
		return blockState.Id
	}
	signedBlock := bc.Blog.ReadBlockByNum(blockNum)
	return signedBlock.Id()
}

func (bc *BlockChain) FetchBlockById(id SHA256Type) *SignedBlock {
	state := bc.ForkDatabase.GetBlock(id)
	if state != nil {
		return state.Block
	}
	block := bc.FetchBlockByNum(NumFromId(id))
	if block != nil && bytes2.Equal(block.Id()[:], id[:]) {
		return block
	}
	return nil
}

func (bc *BlockChain) FetchBlockByNum(num uint32) *SignedBlock {
	blockState := bc.ForkDatabase.GetBlockInCurrentChainIdNum(num)
	if blockState != nil {
		return blockState.Block
	}
	signedBlock := bc.Blog.ReadBlockByNum(num)
	return signedBlock
}

func (bc *BlockChain) PendingBlocKState() *BlockState {
	if bc.Pending != nil {
		return bc.Pending.PendingBlockState
	}
	return nil
}

func (bc *BlockChain) AbortBlock() {
	if bc.Pending != nil {
		for _, trx := range bc.Pending.PendingBlockState.Trxs {
			bc.UnAppliedTransactions[trx.Signed] = trx
		}
		bc.Pending = nil
	}
}

func (bc *BlockChain) StartBlock(when uint64, confirmBlockCount uint16, s BlockStatus) {
	if bc.Pending == nil {
		log.Fatal("pending block should be not nil")
	}
	// generate pending block
	bc.Pending.BlockStatus = s
	bc.Pending.PendingBlockState = NewBlockState(bc.Head.BlockHeaderState, when)
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

func removeTransactionObject(transactionObjects []*database.TransactionObject, index int) []*database.TransactionObject {
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
	for _, index := range expriedIndexes {
		bc.DB.TransactionObjects = removeTransactionObject(bc.DB.TransactionObjects, index)
	}

}