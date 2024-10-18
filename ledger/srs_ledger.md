## Usages and features

type | gettable | settable | committable | at height | usage
-|:--------:|:--------:|:-----------:|:---------:|-
ImitableLedger        |    O     |    X     |      X      |     O     | query
MemLedger |    O     |    O     |      X      |     O     | checkTx, evmCall(?)
MutableLedger |    O     |    O     |      O      |     X     | deliverTx


```go
type IGettable interface {
	Get(...)
	Iterate(...)
}

type ISettable interface {
	Set(...)
	Del(...)
    Snapshot(...)
    RevertToSnapshot(...)
}

type ICommittable interface {
	Commit(...)
}

type IImitable interface {
    IGettable
	ISettable
}

type IMutable interface {
    IImitable
	ICommittable
}

type ILedger interface {
    IMutable
    ImitableLedgerAt(...)
}

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

func (ledger *StateLedger) ImitableLedgerAt(...) {
	// return StateLedger that contains storageTree.GetImmutable()
}

func (ledger *StateLedger) MemLedgerAt(...) {
// return StateLedger that contains new MutableTree copied from storageTree
}

```
