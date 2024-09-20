## Usages and features

usage | committable | gettable | settable | at height | type
-|:-----------:|:--------:|:--------:|:---------:|-
query        |      X      |    O     |    X     |     O     | ImmutableLedger
evm call |      X      |    O     |    O     |     O     | ImmutableLedger ~~mempool?~~ (copy from mutable tree)
tx exec |      O      |    O     |    O     |     X     | MutableLedger 


```go
type StateLedger struct {
    storageTree iavl.MutableTree
	memTree map[key]Item
}

func (ledger *StateLedger) Set_From_Simulation(...) {
// set to memTree
}

func (ledger *StateLedger) Get_In_Simulation(...) {
// get from memTree
// if not found in memTree, get from storageTree and add(set) to memTree. 
}

func (ledger *StateLedger) Set_From_Execution(...) {
	// set to storageTree
}

func (ledger *StateLedger) Get_In_Execution(...) {
    // get from storageTree
}

func (ledger *StateLedger) Iterate(...) {
	// iterate on storageTree
	// if found at memTree, return item in memTree
	// if not found at memTree, return item in storageTree
}

func (ledger *StateLedger) ImmutableLedgerAt(...) {
	// return StateLedger that contains storageTree.GetImmutable()
}

func (ledger *StateLedger) MempoolLedgerAt(...) {
// return StateLedger that contains new MutableTree copied from storageTree
}

```
