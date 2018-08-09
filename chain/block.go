package chain

import (
	"encoding/binary"
	"crypto/sha256"
	"blockchain/crypto"
	"sort"
	"log"
)

type BlockHeader struct {
	Timestamp BlockTimeStamp
	Producer AccountName
	Confirmed uint16
	Previous SHA256Type
	TransactionMRoot SHA256Type
	ActionMRoot SHA256Type
	ScheducerVersion uint32
	NewProducer ProducerScheduleType
	HeaderExtensions []Extension
}

func (bh *BlockHeader) BlockNum() uint32 {
	return NumFromId(bh.Previous) + 1
}

func (bh *BlockHeader) Id() SHA256Type {
	bhEncoded, _ := MarshalBinary(*bh)
	h := sha256.Sum256(bhEncoded)
	binary.BigEndian.PutUint32(h[:], bh.BlockNum())
	return h
}

func NumFromId(id SHA256Type) uint32 {
	return binary.BigEndian.Uint32(id[:4])
}

type SignedBlockHeader struct {
	BlockHeader
	ProducerSignature crypto.Signature
}

type SignedBlock struct {
	SignedBlockHeader
	Transactions []TransactionReceipt
	BlockExtensions []Extension
}

type HeaderConfirmation struct {
	BlockId SHA256Type
	Producer AccountName
	ProducerSignature crypto.Signature
}

type BlockHeaderState struct {
	Id SHA256Type
	BlockNum uint32
	Header SignedBlockHeader
	DPOSProposedIrreversibleBlockNum uint32
	DPOSIrreversibleBlockNum uint32
	BFTIrreversibleBlockNum uint32
	PendingScheduleLibNum uint32
	PendingScheduleHash SHA256Type
	PendingSchedule ProducerScheduleType
	ActiveSchedule ProducerScheduleType
	ProducerToLastProduced map[AccountName]uint32
	ProducerToLastImpliedIRB map[AccountName]uint32
	BlockSigningKey crypto.PublicKey
	ConfirmCount []uint8
	Confirmations []HeaderConfirmation
}

func NewBlockHeaderState() BlockHeaderState {
	return BlockHeaderState{
		ProducerToLastProduced: make(map[AccountName]uint32, 0),
		ProducerToLastImpliedIRB: make(map[AccountName]uint32, 0),
	}
}

type BlockState struct {
	BlockHeaderState
	Block *SignedBlock
	Validated bool
	InCurrentChain bool
	Trxs []*TransactionMetaData
}

type TransactionReceiptHeader struct {
	Status TransactionStatus
	CPUUsageMs uint32
	NetUsageWords Varuint32
}

type TransactionReceipt struct {
	TransactionReceiptHeader
	Id SHA256Type
	PackedTrx PackedTransaction
}

func (bhs *BlockHeaderState) GetScheduledProducer(t uint64) ProducerKey  {
	blockTs := NewBlockTimeStamp()
	blockTs.SetTime(t)
	index := int(blockTs.Slot) % (len(bhs.ActiveSchedule.Producers) * PRODUCER_REPETITION)
	index /= PRODUCER_REPETITION
	return bhs.ActiveSchedule.Producers[index]
}

func (bhs *BlockHeaderState) GenerateNext(when uint64) BlockHeaderState {
	nextBhs := NewBlockHeaderState()
	newBlockTs := NewBlockTimeStamp()
	newBlockTs.SetTime(when)
	newBlockTs.Slot = bhs.Header.Timestamp.Slot + 1
	nextBhs.Header.Timestamp = newBlockTs
	nextBhs.Header.Previous = bhs.Id
	nextBhs.Header.ScheducerVersion = bhs.ActiveSchedule.Version
	prokey := bhs.GetScheduledProducer(when)
	nextBhs.BlockSigningKey = prokey.BlockSigningKey
	nextBhs.Header.Producer = prokey.ProducerName
	nextBhs.PendingScheduleLibNum = bhs.PendingScheduleLibNum
	nextBhs.PendingScheduleHash = bhs.PendingScheduleHash
	nextBhs.BlockNum = bhs.BlockNum + 1
	nextBhs.ProducerToLastProduced = bhs.ProducerToLastProduced
	nextBhs.ProducerToLastImpliedIRB = bhs.ProducerToLastImpliedIRB
	nextBhs.ProducerToLastProduced[prokey.ProducerName] = nextBhs.BlockNum
	nextBhs.ActiveSchedule = bhs.ActiveSchedule
	nextBhs.PendingSchedule = bhs.PendingSchedule
	nextBhs.DPOSProposedIrreversibleBlockNum = bhs.DPOSProposedIrreversibleBlockNum
	nextBhs.BFTIrreversibleBlockNum = bhs.BFTIrreversibleBlockNum
	nextBhs.ProducerToLastImpliedIRB[prokey.ProducerName] = nextBhs.DPOSProposedIrreversibleBlockNum
	nextBhs.DPOSIrreversibleBlockNum = nextBhs.CalcDposLastIrreversible()
	numActiveProducers := len(bhs.ActiveSchedule.Producers)
	requiredConfs := uint32(numActiveProducers*2/3) + 1
	if len(bhs.ConfirmCount) <= MAXIMUM_TRACKED_DPOS_CONFIRMATIONS {
		nextBhs.ConfirmCount = bhs.ConfirmCount
		nextBhs.ConfirmCount = append(nextBhs.ConfirmCount, uint8(requiredConfs))
	} else {
		nextBhs.ConfirmCount = bhs.ConfirmCount[1:]
		nextBhs.ConfirmCount = append(nextBhs.ConfirmCount, uint8(requiredConfs))
	}
	return nextBhs
}

func (bhs *BlockHeaderState) CalcDposLastIrreversible() uint32 {
	blockNums := make([]uint32, len(bhs.ProducerToLastImpliedIRB))
	for _, i := range bhs.ProducerToLastImpliedIRB {
		blockNums = append(blockNums, i)
	}
	if len(blockNums) == 0 {
		return 0
	}
	sort.Slice(blockNums, func(i, j int) bool {
		return blockNums[i] < blockNums[j]
	})
	return blockNums[(len(blockNums)-1)/3]
}

func (bhs *BlockHeaderState) SetConfirmed(numPreBlocks uint16) {
	bhs.Header.Confirmed = numPreBlocks
	i := uint32(len(bhs.ConfirmCount)-1)
	// confirm the head block too
	blocksToConfirm := numPreBlocks + 1
	for i >=0 && blocksToConfirm != 0 {
		bhs.ConfirmCount[i] -= 1
		if bhs.ConfirmCount[i] == 0 {
			blockNumForI := bhs.BlockNum - uint32(len(bhs.ConfirmCount)) - 1 - i
			bhs.DPOSIrreversibleBlockNum = blockNumForI
			if i == uint32(len(bhs.ConfirmCount)) - 1 {
				bhs.ConfirmCount = make([]uint8, 0)
			} else {
				bhs.ConfirmCount = bhs.ConfirmCount[i+1:]
			}
			return
		}
		i -= 1
		blocksToConfirm -= 1
	}
}

func (bhs *BlockHeaderState) MaybePromotePending() bool {
	if len(bhs.PendingSchedule.Producers) != 0 && bhs.DPOSIrreversibleBlockNum >= bhs.PendingScheduleLibNum {
		bhs.ActiveSchedule = bhs.PendingSchedule
		newProducerToLastProduced := map[AccountName]uint32{}
		for _ ,pro := range bhs.ActiveSchedule.Producers {
			value, existing := bhs.ProducerToLastProduced[pro.ProducerName]
			if existing {
				newProducerToLastProduced[pro.ProducerName] = value
			} else {
				newProducerToLastProduced[pro.ProducerName] = bhs.DPOSIrreversibleBlockNum
			}
		}
		newProducerToLastImpliedIRB := map[AccountName]uint32{}
		for _ ,pro := range bhs.ActiveSchedule.Producers {
			value, existing := bhs.ProducerToLastImpliedIRB[pro.ProducerName]
			if existing {
				newProducerToLastImpliedIRB[pro.ProducerName] = value
			} else {
				newProducerToLastImpliedIRB[pro.ProducerName] = bhs.DPOSIrreversibleBlockNum
			}
		}
		bhs.ProducerToLastProduced = newProducerToLastImpliedIRB
		bhs.ProducerToLastImpliedIRB = newProducerToLastImpliedIRB
		bhs.ProducerToLastProduced[bhs.Header.Producer] = bhs.BlockNum
		return true
	}
	return false
}

func (bhs *BlockHeaderState) SetNewProducer(pending ProducerScheduleType) {
	if pending.Version != bhs.ActiveSchedule.Version + 1 {
		log.Fatal("wrong producer schedule version specified")
	}
	if len(bhs.PendingSchedule.Producers) != 0 {
		log.Fatal("cannot set new pending producers until last pending is confirmed")
	}
	bhs.Header.NewProducer = pending
	buf, _ := MarshalBinary(bhs.Header.NewProducer)
	bhs.PendingScheduleHash = sha256.Sum256(buf)
	bhs.PendingSchedule = bhs.Header.NewProducer
	bhs.PendingScheduleLibNum = bhs.BlockNum
}

func (bhs *BlockHeaderState) Sign(signer SignerCallBack) {
	buf, _ := MarshalBinary(bhs.Header)
	d := sha256.Sum256(buf)
	bhs.Header.ProducerSignature = signer(d)
}

func NewBlockState(head BlockHeaderState, when uint64) *BlockState {
	newBs := &BlockState{
		head.GenerateNext(when),
		&SignedBlock{},
		false,
		false,
		[]*TransactionMetaData{},
	}
	newBs.Block.SignedBlockHeader = newBs.Header
	return newBs
}