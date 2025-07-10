package types

import (
	"encoding/binary"
	"github.com/beatoz/beatoz-go/libs/jsonx"
	tmdb "github.com/tendermint/tm-db"
	"sync"
)

const (
	keyBlockContext = "bc"
	keyTxn          = "xn"
)

type MetaDB struct {
	db tmdb.DB

	mtx   sync.RWMutex
	cache map[string][]byte
}

func OpenMetaDB(name, dir string) (*MetaDB, error) {
	// The returned 'db' instance is safe in concurrent use.
	db, err := tmdb.NewDB(name, "goleveldb", dir)
	if err != nil {
		return nil, err
	}

	return &MetaDB{
		db:    db,
		cache: make(map[string][]byte),
	}, nil
}

func (stdb *MetaDB) Close() error {
	stdb.mtx.Lock()
	defer stdb.mtx.Unlock()

	stdb.cache = map[string][]byte{}
	return stdb.db.Close()
}

func (stdb *MetaDB) LastBlockContext() *BlockContext {
	bz := stdb.get(keyBlockContext)
	if bz == nil {
		return nil
	}
	ret := &BlockContext{}
	if err := jsonx.Unmarshal(bz, ret); err != nil {
		return nil
	}
	return ret
}

func (stdb *MetaDB) PutLastBlockContext(ctx *BlockContext) error {
	bz, err := jsonx.Marshal(ctx)
	if err != nil {
		return err
	}
	return stdb.put(keyBlockContext, bz)
}

func (stdb *MetaDB) Txn() uint64 {
	bz := stdb.get(keyTxn)
	if bz == nil {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

func (stdb *MetaDB) PutTxn(n uint64) error {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(n))
	return stdb.put(keyTxn, bz)
}

func (stdb *MetaDB) putCache(k string, v []byte) {
	stdb.mtx.Lock()
	defer stdb.mtx.Unlock()

	stdb.cache[k] = v
}

func (stdb *MetaDB) getCache(k string) []byte {
	stdb.mtx.RLock()
	defer stdb.mtx.RUnlock()

	v := stdb.cache[k]
	return v
}

func (stdb *MetaDB) get(k string) []byte {
	if v := stdb.getCache(k); v != nil {
		return v
	}

	if v, err := stdb.db.Get([]byte(k)); err == nil {
		stdb.putCache(k, v)
		return v
	}

	return nil
}

func (stdb *MetaDB) put(k string, v []byte) error {
	if err := stdb.db.SetSync([]byte(k), v); err != nil {
		return err
	}
	stdb.putCache(k, v)
	return nil
}
