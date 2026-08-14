package ledger

import (
	"fmt"
	"sync"

	"github.com/beatoz/beatoz-go/ledger/common"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	v2 "github.com/beatoz/beatoz-go/ledger/v2"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/xerrors"
	tmlog "github.com/tendermint/tendermint/libs/log"
)

type LedgerKey = common.LedgerKey
type ILedgerItem = common.ILedgerItem
type FuncNewItemFor = common.FuncNewItemFor
type FuncIterate = common.FuncIterate
type IImitable = common.IImitable

type LedgerVersion = common.LedgerVersion

const (
	LedgerV1 LedgerVersion = common.LedgerV1
	LedgerV2 LedgerVersion = common.LedgerV2
)

type IStateLedger interface {
	// Version returns the persisted IAVL tree version used as the committed state height.
	Version() int64

	Get(LedgerKey, bool) (ILedgerItem, xerrors.XError)
	Iterate(FuncIterate, bool) xerrors.XError
	Seek([]byte, bool, FuncIterate, bool) xerrors.XError
	Set(LedgerKey, ILedgerItem, bool) xerrors.XError
	Del(LedgerKey, bool) xerrors.XError

	CreateCache(bool) xerrors.XError
	WriteCache(bool) xerrors.XError
	ClearCache(bool) xerrors.XError

	Commit() ([]byte, int64, xerrors.XError)
	Close() xerrors.XError
	ImitableLedgerAt(int64) (IImitable, xerrors.XError)
}

func LedgerVersionAt(chainID string, height int64) LedgerVersion {
	if types.IsBTIP45(chainID, height) {
		return LedgerV2
	}
	return LedgerV1
}

// StateLedgerManager selects and forwards to the active v1 or v2 StateLedger.
// It also handles an explicitly requested one-time migration from v1 to v2.
type StateLedgerManager struct {
	active        IStateLedger
	activeVersion LedgerVersion

	name       string
	dbDir      string
	cacheSize  int
	newItemFor FuncNewItemFor
	logger     tmlog.Logger
	mtx        sync.RWMutex
}

func NewStateLedgerManager(name, dbDir string, cacheSize int, newItemFor FuncNewItemFor, logger tmlog.Logger) (*StateLedgerManager, xerrors.XError) {
	manager := &StateLedgerManager{
		name:       name,
		dbDir:      dbDir,
		cacheSize:  cacheSize,
		newItemFor: newItemFor,
		logger:     logger.With("ledger", "StateLedgerManager"),
	}

	active, xerr := manager.openV1()
	if xerr != nil {
		return nil, xerr
	}

	manager.active = active
	manager.activeVersion = LedgerV1
	return manager, nil
}

// UpgradeLedgerVersion migrates to the target ledger version.
// It is a no-op if the active version already matches.
func (manager *StateLedgerManager) UpgradeLedgerVersion(target LedgerVersion) xerrors.XError {
	manager.mtx.Lock()
	defer manager.mtx.Unlock()

	if _, xerr := manager.activeLedger(); xerr != nil {
		return xerr
	}
	if target == manager.activeVersion {
		return nil
	}

	if target == LedgerV2 && manager.activeVersion == LedgerV1 {
		return manager.migrateToV2()
	}

	return xerrors.NewOrdinary(fmt.Sprintf("unsupported ledger version upgrade: v%d -> v%d", manager.activeVersion, target))
}

func (manager *StateLedgerManager) ActiveLedgerVersion() LedgerVersion {
	manager.mtx.RLock()
	defer manager.mtx.RUnlock()
	return manager.activeVersion
}

// activeLedger returns the selected implementation. The caller must hold manager.mtx.
func (manager *StateLedgerManager) activeLedger() (IStateLedger, xerrors.XError) {
	if manager.active == nil {
		return nil, xerrors.NewOrdinary("state ledger manager is closed")
	}
	return manager.active, nil
}

// IStateLedger forwarding keeps migration and Close mutually exclusive with each operation.

func (manager *StateLedgerManager) Version() int64 {
	manager.mtx.RLock()
	defer manager.mtx.RUnlock()
	if manager.active == nil {
		return 0
	}
	return manager.active.Version()
}

func (manager *StateLedgerManager) Get(key LedgerKey, exec bool) (ILedgerItem, xerrors.XError) {
	manager.mtx.RLock()
	defer manager.mtx.RUnlock()
	active, xerr := manager.activeLedger()
	if xerr != nil {
		return nil, xerr
	}
	return active.Get(key, exec)
}

func (manager *StateLedgerManager) Iterate(cb FuncIterate, exec bool) xerrors.XError {
	manager.mtx.RLock()
	defer manager.mtx.RUnlock()
	active, xerr := manager.activeLedger()
	if xerr != nil {
		return xerr
	}
	return active.Iterate(cb, exec)
}

func (manager *StateLedgerManager) Seek(prefix []byte, ascending bool, cb FuncIterate, exec bool) xerrors.XError {
	manager.mtx.RLock()
	defer manager.mtx.RUnlock()
	active, xerr := manager.activeLedger()
	if xerr != nil {
		return xerr
	}
	return active.Seek(prefix, ascending, cb, exec)
}

func (manager *StateLedgerManager) Set(key LedgerKey, item ILedgerItem, exec bool) xerrors.XError {
	manager.mtx.RLock()
	defer manager.mtx.RUnlock()
	active, xerr := manager.activeLedger()
	if xerr != nil {
		return xerr
	}
	return active.Set(key, item, exec)
}

func (manager *StateLedgerManager) Del(key LedgerKey, exec bool) xerrors.XError {
	manager.mtx.RLock()
	defer manager.mtx.RUnlock()
	active, xerr := manager.activeLedger()
	if xerr != nil {
		return xerr
	}
	return active.Del(key, exec)
}

func (manager *StateLedgerManager) CreateCache(exec bool) xerrors.XError {
	manager.mtx.RLock()
	defer manager.mtx.RUnlock()
	active, xerr := manager.activeLedger()
	if xerr != nil {
		return xerr
	}
	if manager.activeVersion == LedgerV1 {
		return nil
	}
	return active.CreateCache(exec)
}

func (manager *StateLedgerManager) WriteCache(exec bool) xerrors.XError {
	manager.mtx.RLock()
	defer manager.mtx.RUnlock()
	active, xerr := manager.activeLedger()
	if xerr != nil {
		return xerr
	}
	if manager.activeVersion == LedgerV1 {
		return nil
	}
	return active.WriteCache(exec)
}

func (manager *StateLedgerManager) ClearCache(exec bool) xerrors.XError {
	manager.mtx.RLock()
	defer manager.mtx.RUnlock()
	active, xerr := manager.activeLedger()
	if xerr != nil {
		return xerr
	}
	if manager.activeVersion == LedgerV1 {
		return nil
	}
	return active.ClearCache(exec)
}

func (manager *StateLedgerManager) Commit() ([]byte, int64, xerrors.XError) {
	manager.mtx.RLock()
	defer manager.mtx.RUnlock()
	active, xerr := manager.activeLedger()
	if xerr != nil {
		return nil, 0, xerr
	}
	return active.Commit()
}

func (manager *StateLedgerManager) Close() xerrors.XError {
	manager.mtx.Lock()
	defer manager.mtx.Unlock()
	if manager.active == nil {
		return nil
	}
	if xerr := manager.active.Close(); xerr != nil {
		return xerr
	}
	manager.active = nil
	manager.activeVersion = common.LedgerNone
	return nil
}

func (manager *StateLedgerManager) ImitableLedgerAt(height int64) (IImitable, xerrors.XError) {
	manager.mtx.RLock()
	defer manager.mtx.RUnlock()
	active, xerr := manager.activeLedger()
	if xerr != nil {
		return nil, xerr
	}
	return active.ImitableLedgerAt(height)
}

// openV1 opens the v1 StateLedger.
func (manager *StateLedgerManager) openV1() (IStateLedger, xerrors.XError) {
	return v1.NewStateLedger(manager.name, manager.dbDir, manager.cacheSize, manager.newItemFor, manager.logger)
}

// openV2 opens the v2 StateLedger.
func (manager *StateLedgerManager) openV2() (IStateLedger, xerrors.XError) {
	return v2.NewStateLedger(manager.name, manager.dbDir, manager.cacheSize, manager.newItemFor, manager.logger)
}

// migrateToV2 closes the active v1 ledger and opens v2 from the same DB.
// Caller must hold manager.mtx write lock.
func (manager *StateLedgerManager) migrateToV2() xerrors.XError {
	if xerr := manager.active.Close(); xerr != nil {
		return xerr
	}
	manager.active = nil
	manager.activeVersion = common.LedgerNone

	newLedger, xerr := manager.openV2()
	if xerr != nil {
		return xerr
	}

	manager.active = newLedger
	manager.activeVersion = LedgerV2
	return nil
}

var _ IStateLedger = (*v1.StateLedger)(nil)
var _ IStateLedger = (*v2.StateLedger)(nil)
var _ IStateLedger = (*StateLedgerManager)(nil)
