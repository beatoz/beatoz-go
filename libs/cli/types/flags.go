package types

type RootFlags struct {
	RPCUrl   string `mapstructure:"rpc"`
	From     string `mapstructure:"from"`
	To       []byte `mapstructure:"to"`
	Gas      uint64 `mapstructure:"gas"`
	GasPrice string `mapstructure:"gas_price"`
	Amount   string `mapstructure:"amt"`
}

type TrxUnstakingFlags struct {
	StakeID []byte
}
