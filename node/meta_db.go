package node

import (
	"encoding/binary"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	"github.com/holiman/uint256"
	tmdb "github.com/tendermint/tm-db"
	"sync"
)

const (
	keyBlockContext = "bc"
	keyTxn          = "xn"
	keyTxFee        = "xf"
)

type MetaDB struct {
	db tmdb.DB

	txn        uint64
	totalTxFee *uint256.Int

	mtx sync.RWMutex
}

func OpenMetaDB(name, dir string) (*MetaDB, error) {
	// The returned 'db' instance is safe in concurrent use.
	db, err := tmdb.NewDB(name, "goleveldb", dir)
	if err != nil {
		return nil, err
	}

	txn := uint64(0)
	if v, err := db.Get([]byte(keyTxn)); v != nil && err == nil {
		txn = binary.BigEndian.Uint64(v)
	}

	txFeeTotal := uint256.NewInt(0)
	if v, err := db.Get([]byte(keyTxFee)); v != nil && err == nil {
		_ = txFeeTotal.SetBytes(v)
	}

	return &MetaDB{
		db:         db,
		txn:        txn,
		totalTxFee: txFeeTotal,
	}, nil
}

func (stdb *MetaDB) Close() error {
	stdb.mtx.Lock()
	defer stdb.mtx.Unlock()

	//stdb.cache = map[string][]byte{}
	return stdb.db.Close()
}

func (stdb *MetaDB) LastBlockContext() *ctrlertypes.BlockContext {
	stdb.mtx.RLock()
	defer stdb.mtx.RUnlock()

	bz := stdb.get(keyBlockContext)
	if bz == nil {
		return nil
	}
	ret := &ctrlertypes.BlockContext{}
	if err := jsonx.Unmarshal(bz, ret); err != nil {
		return nil
	}
	return ret
}

func (stdb *MetaDB) PutLastBlockContext(ctx *ctrlertypes.BlockContext) error {
	stdb.mtx.Lock()
	defer stdb.mtx.Unlock()

	bz, err := jsonx.Marshal(ctx)
	if err != nil {
		return err
	}
	return stdb.put(keyBlockContext, bz)
}

func (stdb *MetaDB) Txn() uint64 {
	stdb.mtx.RLock()
	defer stdb.mtx.RUnlock()

	return stdb.txn
}

func (stdb *MetaDB) PutTxn(n uint64) error {
	stdb.mtx.Lock()
	defer stdb.mtx.Unlock()

	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, n)
	if err := stdb.put(keyTxn, bz); err != nil {
		return err
	}
	stdb.txn = n
	return nil
}

func (stdb *MetaDB) TotalTxFee() *uint256.Int {
	stdb.mtx.RLock()
	defer stdb.mtx.RUnlock()

	return stdb.totalTxFee.Clone()
}

func (stdb *MetaDB) PutTotalTxFee(f *uint256.Int) error {
	stdb.mtx.Lock()
	defer stdb.mtx.Unlock()

	if err := stdb.put(keyTxFee, f.Bytes()); err != nil {
		return err
	}
	stdb.totalTxFee = f.Clone()
	return nil
}

func (stdb *MetaDB) get(k string) []byte {
	if v, err := stdb.db.Get([]byte(k)); err == nil {
		return v
	}
	return nil
}

func (stdb *MetaDB) put(k string, v []byte) error {
	if err := stdb.db.SetSync([]byte(k), v); err != nil {
		return err
	}
	return nil
}
