package network

import (
	"gopkg.in/karalabe/cookiejar.v1/collections/deque"
)

type states uint8
const(
	Lib_Catchup states = iota
	Head_Catchup
	In_Sync
)

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

func (sm *SyncManager) isActive(c *Connection) {
	if sm.state == Head_Catchup && c != nil {

	}
}
