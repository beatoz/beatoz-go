package types

import (
	"encoding/binary"
	"github.com/tendermint/tendermint/libs/json"
	tmdb "github.com/tendermint/tm-db"
	"sync"
)

const (
	keyChainID      = "ci"
	keyBlockHeight  = "bh"
	keyBlockContext = "bc"
	keyBlockAppHash = "ah"
	keyRewardHash   = "rh"
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

func (stdb *MetaDB) ChainID() string {
	v := stdb.get(keyChainID)
	if v == nil {
		return ""
	}
	return string(v)
}

func (stdb *MetaDB) PutChainID(chainId string) error {
	return stdb.put(keyChainID, []byte(chainId))
}

func (stdb *MetaDB) LastBlockHeight() int64 {
	v := stdb.get(keyBlockHeight)
	if v == nil {
		return 0
	}
	return int64(binary.BigEndian.Uint64(v))
}

func (stdb *MetaDB) PutLastBlockHeight(bh int64) error {
	v := make([]byte, 8)
	binary.BigEndian.PutUint64(v, uint64(bh))
	return stdb.put(keyBlockHeight, v)
}

func (stdb *MetaDB) LastBlockAppHash() []byte {
	return stdb.get(keyBlockAppHash)
}

func (stdb *MetaDB) PutLastBlockAppHash(v []byte) error {
	return stdb.put(keyBlockAppHash, v)
}

func (stdb *MetaDB) LastRewardHash() []byte {
	return stdb.get(keyRewardHash)
}

func (stdb *MetaDB) PutLastRewardHash(v []byte) error {
	return stdb.put(keyRewardHash, v)
}

func (stdb *MetaDB) LastBlockContext() *BlockContext {
	bz := stdb.get(keyBlockContext)
	if bz == nil {
		return nil
	}
	ret := &BlockContext{}
	if err := json.Unmarshal(bz, ret); err != nil {
		return nil
	}
	return ret
}

func (stdb *MetaDB) PutLastBlockContext(ctx *BlockContext) error {
	bz, err := json.Marshal(ctx)
	if err != nil {
		return err
	}
	return stdb.put(keyBlockContext, bz)
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
