package chain

type ControllerConfig struct {
	BlocksDir string
	StateDir string
	StateSize uint64
	ReversibleCacheSize uint64
	Genesis GenesisState
}

type Controller struct {

}
