package chain

import "blockchain/database"

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
	Pending PendingState
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

func (bc *BlockChain) GetBlockIdForNum(blockNum uint32) SHA256Type {
	blockState := bc.ForkDatabase.GetBlockInCurrentChainIdNum(blockNum)
	if blockState != nil {
		return blockState.Id
	}
	signedBlock := bc.Blog.ReadBlockByNum(blockNum)
	return signedBlock.Id()
}
