package ledger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/beatoz/beatoz-go/ledger/common"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	v2 "github.com/beatoz/beatoz-go/ledger/v2"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	tmlog "github.com/tendermint/tendermint/libs/log"
)

type managerTestItem struct {
	key   []byte
	value []byte
}

func newManagerTestItem(key, value string) *managerTestItem {
	return &managerTestItem{key: []byte(key), value: []byte(value)}
}

func (item *managerTestItem) Encode() ([]byte, xerrors.XError) {
	return append([]byte(nil), item.value...), nil
}

func (item *managerTestItem) Decode(key, value []byte) xerrors.XError {
	item.key = append(item.key[:0], key...)
	item.value = append(item.value[:0], value...)
	return nil
}

type managerStateLedgerStub struct {
	versionFn          func() int64
	getFn              func(LedgerKey, bool) (ILedgerItem, xerrors.XError)
	iterateFn          func(FuncIterate, bool) xerrors.XError
	seekFn             func([]byte, bool, FuncIterate, bool) xerrors.XError
	setFn              func(LedgerKey, ILedgerItem, bool) xerrors.XError
	delFn              func(LedgerKey, bool) xerrors.XError
	createCacheFn      func(bool) xerrors.XError
	writeCacheFn       func(bool) xerrors.XError
	clearCacheFn       func(bool) xerrors.XError
	commitFn           func() ([]byte, int64, xerrors.XError)
	closeFn            func() xerrors.XError
	imitableLedgerAtFn func(int64) (IImitable, xerrors.XError)
}

func (stub *managerStateLedgerStub) Version() int64 {
	if stub.versionFn == nil {
		return 0
	}
	return stub.versionFn()
}

func (stub *managerStateLedgerStub) Get(key LedgerKey, exec bool) (ILedgerItem, xerrors.XError) {
	if stub.getFn == nil {
		return nil, nil
	}
	return stub.getFn(key, exec)
}

func (stub *managerStateLedgerStub) Iterate(cb FuncIterate, exec bool) xerrors.XError {
	if stub.iterateFn == nil {
		return nil
	}
	return stub.iterateFn(cb, exec)
}

func (stub *managerStateLedgerStub) Seek(prefix []byte, ascending bool, cb FuncIterate, exec bool) xerrors.XError {
	if stub.seekFn == nil {
		return nil
	}
	return stub.seekFn(prefix, ascending, cb, exec)
}

func (stub *managerStateLedgerStub) Set(key LedgerKey, item ILedgerItem, exec bool) xerrors.XError {
	if stub.setFn == nil {
		return nil
	}
	return stub.setFn(key, item, exec)
}

func (stub *managerStateLedgerStub) Del(key LedgerKey, exec bool) xerrors.XError {
	if stub.delFn == nil {
		return nil
	}
	return stub.delFn(key, exec)
}

func (stub *managerStateLedgerStub) CreateCache(exec bool) xerrors.XError {
	if stub.createCacheFn == nil {
		return nil
	}
	return stub.createCacheFn(exec)
}

func (stub *managerStateLedgerStub) WriteCache(exec bool) xerrors.XError {
	if stub.writeCacheFn == nil {
		return nil
	}
	return stub.writeCacheFn(exec)
}

func (stub *managerStateLedgerStub) ClearCache(exec bool) xerrors.XError {
	if stub.clearCacheFn == nil {
		return nil
	}
	return stub.clearCacheFn(exec)
}

func (stub *managerStateLedgerStub) Commit() ([]byte, int64, xerrors.XError) {
	if stub.commitFn == nil {
		return nil, 0, nil
	}
	return stub.commitFn()
}

func (stub *managerStateLedgerStub) Close() xerrors.XError {
	if stub.closeFn == nil {
		return nil
	}
	return stub.closeFn()
}

func (stub *managerStateLedgerStub) ImitableLedgerAt(height int64) (IImitable, xerrors.XError) {
	if stub.imitableLedgerAtFn == nil {
		return nil, nil
	}
	return stub.imitableLedgerAtFn(height)
}

type managerImitableLedgerStub struct{}

func (stub *managerImitableLedgerStub) Get(LedgerKey) (ILedgerItem, xerrors.XError) {
	return nil, nil
}

func (stub *managerImitableLedgerStub) Iterate(FuncIterate) xerrors.XError {
	return nil
}

func (stub *managerImitableLedgerStub) Seek([]byte, bool, FuncIterate) xerrors.XError {
	return nil
}

func (stub *managerImitableLedgerStub) Set(LedgerKey, ILedgerItem) xerrors.XError {
	return nil
}

func (stub *managerImitableLedgerStub) Del(LedgerKey) xerrors.XError {
	return nil
}

func newManagerTestLedger(t *testing.T, dbDir string) *StateLedgerManager {
	t.Helper()
	manager, xerr := NewStateLedgerManager(
		"manager",
		dbDir,
		100,
		func(LedgerKey) ILedgerItem { return &managerTestItem{} },
		tmlog.NewNopLogger(),
	)
	require.NoError(t, xerr)
	t.Cleanup(func() { require.NoError(t, manager.Close()) })
	return manager
}

func newUnopenedManagerForTest(dbDir string) *StateLedgerManager {
	return &StateLedgerManager{
		name:       "manager",
		dbDir:      dbDir,
		cacheSize:  100,
		newItemFor: func(LedgerKey) ILedgerItem { return &managerTestItem{} },
		logger:     tmlog.NewNopLogger(),
	}
}

func TestLedgerVersionAt(t *testing.T) {
	testCases := []struct {
		chainID string
		height  int64
		want    LedgerVersion
	}{
		{chainID: "0xbea701", height: 0, want: LedgerV1},
		{chainID: "0xbea701", height: 499_999, want: LedgerV1},
		{chainID: "0xbea701", height: 500_000, want: LedgerV2},
		{chainID: "0xbea700", height: 0, want: LedgerV2},
		{chainID: "unregistered", height: 0, want: LedgerV2},
	}

	for _, testCase := range testCases {
		got := LedgerVersionAt(testCase.chainID, testCase.height)
		require.Equal(t, testCase.want, got, "chainID=%s height=%d", testCase.chainID, testCase.height)
	}
}

func TestNewStateLedgerManager(t *testing.T) {
	dbDir := t.TempDir()
	manager := newManagerTestLedger(t, dbDir)

	require.IsType(t, &v1.StateLedger{}, manager.active)
	require.Equal(t, LedgerV1, manager.ActiveLedgerVersion())
	require.Equal(t, "manager", manager.name)
	require.Equal(t, dbDir, manager.dbDir)
	require.Equal(t, 100, manager.cacheSize)
	require.NotNil(t, manager.newItemFor)
	require.NotNil(t, manager.logger)
}

func TestNewStateLedgerManager_OpenError(t *testing.T) {
	badDBDir := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(badDBDir, []byte("file"), 0o600))

	manager, xerr := NewStateLedgerManager(
		"manager",
		badDBDir,
		100,
		func(LedgerKey) ILedgerItem { return &managerTestItem{} },
		tmlog.NewNopLogger(),
	)

	require.Nil(t, manager)
	require.Error(t, xerr)
}

func TestStateLedgerManager_UpgradeLedgerVersion_SameVersion(t *testing.T) {
	closeCalled := false
	stub := &managerStateLedgerStub{
		closeFn: func() xerrors.XError {
			closeCalled = true
			return nil
		},
	}
	manager := &StateLedgerManager{active: stub, activeVersion: LedgerV1}

	xerr := manager.UpgradeLedgerVersion(LedgerV1)

	require.NoError(t, xerr)
	require.False(t, closeCalled)
	require.Same(t, stub, manager.active)
	require.Equal(t, LedgerV1, manager.ActiveLedgerVersion())
}

func TestStateLedgerManager_UpgradeLedgerVersion_V2(t *testing.T) {
	manager := newManagerTestLedger(t, t.TempDir())

	xerr := manager.UpgradeLedgerVersion(LedgerV2)

	require.NoError(t, xerr)
	require.IsType(t, &v2.StateLedger{}, manager.active)
	require.Equal(t, LedgerV2, manager.ActiveLedgerVersion())
}

func TestStateLedgerManager_UpgradeLedgerVersion_Unsupported(t *testing.T) {
	stub := &managerStateLedgerStub{}
	manager := &StateLedgerManager{active: stub, activeVersion: LedgerV2}

	xerr := manager.UpgradeLedgerVersion(LedgerV1)

	require.ErrorContains(t, xerr, "unsupported ledger version upgrade: v2 -> v1")
	require.Same(t, stub, manager.active)
	require.Equal(t, LedgerV2, manager.ActiveLedgerVersion())
}

func TestStateLedgerManager_UpgradeLedgerVersion_InvalidTarget(t *testing.T) {
	stub := &managerStateLedgerStub{}
	manager := &StateLedgerManager{active: stub, activeVersion: LedgerV1}

	xerr := manager.UpgradeLedgerVersion(common.LedgerNone)

	require.ErrorContains(t, xerr, "unsupported ledger version upgrade: v1 -> v0")
	require.Same(t, stub, manager.active)
	require.Equal(t, LedgerV1, manager.ActiveLedgerVersion())
}

func TestStateLedgerManager_UpgradeLedgerVersion_Closed(t *testing.T) {
	manager := &StateLedgerManager{activeVersion: common.LedgerNone}

	xerr := manager.UpgradeLedgerVersion(LedgerV2)

	require.ErrorContains(t, xerr, "state ledger manager is closed")
}

func TestStateLedgerManager_ActiveLedgerVersion(t *testing.T) {
	manager := &StateLedgerManager{activeVersion: LedgerV2}

	require.Equal(t, LedgerV2, manager.ActiveLedgerVersion())
}

func TestStateLedgerManager_activeLedger(t *testing.T) {
	stub := &managerStateLedgerStub{}
	manager := &StateLedgerManager{active: stub, activeVersion: LedgerV1}

	active, xerr := manager.activeLedger()

	require.NoError(t, xerr)
	require.Same(t, stub, active)
}

func TestStateLedgerManager_activeLedger_Closed(t *testing.T) {
	manager := &StateLedgerManager{activeVersion: common.LedgerNone}

	active, xerr := manager.activeLedger()

	require.Nil(t, active)
	require.ErrorContains(t, xerr, "state ledger manager is closed")
}

func TestStateLedgerManager_Version(t *testing.T) {
	called := 0
	stub := &managerStateLedgerStub{
		versionFn: func() int64 {
			called++
			return 17
		},
	}
	manager := &StateLedgerManager{active: stub, activeVersion: LedgerV2}

	version := manager.Version()

	require.EqualValues(t, 17, version)
	require.Equal(t, 1, called)
}

func TestStateLedgerManager_Version_Closed(t *testing.T) {
	manager := &StateLedgerManager{activeVersion: common.LedgerNone}

	require.Zero(t, manager.Version())
}

func TestStateLedgerManager_Get(t *testing.T) {
	wantKey := LedgerKey("key")
	wantItem := newManagerTestItem("key", "value")
	wantErr := xerrors.NewOrdinary("get error")
	var gotKey LedgerKey
	var gotExec bool
	stub := &managerStateLedgerStub{
		getFn: func(key LedgerKey, exec bool) (ILedgerItem, xerrors.XError) {
			gotKey = key
			gotExec = exec
			return wantItem, wantErr
		},
	}
	manager := &StateLedgerManager{active: stub, activeVersion: LedgerV2}

	item, xerr := manager.Get(wantKey, true)

	require.Equal(t, wantKey, gotKey)
	require.True(t, gotExec)
	require.Same(t, wantItem, item)
	require.Equal(t, wantErr, xerr)
}

func TestStateLedgerManager_Iterate(t *testing.T) {
	wantKey := LedgerKey("key")
	wantItem := newManagerTestItem("key", "value")
	wantErr := xerrors.NewOrdinary("iterate callback error")
	var gotExec bool
	stub := &managerStateLedgerStub{
		iterateFn: func(cb FuncIterate, exec bool) xerrors.XError {
			gotExec = exec
			return cb(wantKey, wantItem)
		},
	}
	manager := &StateLedgerManager{active: stub, activeVersion: LedgerV2}
	callbackCalled := false

	xerr := manager.Iterate(func(key LedgerKey, item ILedgerItem) xerrors.XError {
		callbackCalled = true
		require.Equal(t, wantKey, key)
		require.Same(t, wantItem, item)
		return wantErr
	}, true)

	require.True(t, gotExec)
	require.True(t, callbackCalled)
	require.Equal(t, wantErr, xerr)
}

func TestStateLedgerManager_Seek(t *testing.T) {
	wantPrefix := []byte("prefix")
	wantKey := LedgerKey("prefix-key")
	wantItem := newManagerTestItem("prefix-key", "value")
	wantErr := xerrors.NewOrdinary("seek callback error")
	var gotPrefix []byte
	var gotAscending bool
	var gotExec bool
	stub := &managerStateLedgerStub{
		seekFn: func(prefix []byte, ascending bool, cb FuncIterate, exec bool) xerrors.XError {
			gotPrefix = prefix
			gotAscending = ascending
			gotExec = exec
			return cb(wantKey, wantItem)
		},
	}
	manager := &StateLedgerManager{active: stub, activeVersion: LedgerV2}
	callbackCalled := false

	xerr := manager.Seek(wantPrefix, false, func(key LedgerKey, item ILedgerItem) xerrors.XError {
		callbackCalled = true
		require.Equal(t, wantKey, key)
		require.Same(t, wantItem, item)
		return wantErr
	}, true)

	require.Equal(t, wantPrefix, gotPrefix)
	require.False(t, gotAscending)
	require.True(t, gotExec)
	require.True(t, callbackCalled)
	require.Equal(t, wantErr, xerr)
}

func TestStateLedgerManager_Set(t *testing.T) {
	wantKey := LedgerKey("key")
	wantItem := newManagerTestItem("key", "value")
	wantErr := xerrors.NewOrdinary("set error")
	var gotKey LedgerKey
	var gotItem ILedgerItem
	var gotExec bool
	stub := &managerStateLedgerStub{
		setFn: func(key LedgerKey, item ILedgerItem, exec bool) xerrors.XError {
			gotKey = key
			gotItem = item
			gotExec = exec
			return wantErr
		},
	}
	manager := &StateLedgerManager{active: stub, activeVersion: LedgerV2}

	xerr := manager.Set(wantKey, wantItem, true)

	require.Equal(t, wantKey, gotKey)
	require.Same(t, wantItem, gotItem)
	require.True(t, gotExec)
	require.Equal(t, wantErr, xerr)
}

func TestStateLedgerManager_Del(t *testing.T) {
	wantKey := LedgerKey("key")
	wantErr := xerrors.NewOrdinary("del error")
	var gotKey LedgerKey
	var gotExec bool
	stub := &managerStateLedgerStub{
		delFn: func(key LedgerKey, exec bool) xerrors.XError {
			gotKey = key
			gotExec = exec
			return wantErr
		},
	}
	manager := &StateLedgerManager{active: stub, activeVersion: LedgerV2}

	xerr := manager.Del(wantKey, true)

	require.Equal(t, wantKey, gotKey)
	require.True(t, gotExec)
	require.Equal(t, wantErr, xerr)
}

func TestStateLedgerManager_CreateCache(t *testing.T) {
	v1Called := false
	v1Stub := &managerStateLedgerStub{
		createCacheFn: func(bool) xerrors.XError {
			v1Called = true
			return xerrors.NewOrdinary("must not be called")
		},
	}
	v1Manager := &StateLedgerManager{active: v1Stub, activeVersion: LedgerV1}
	for _, exec := range []bool{false, true} {
		require.NoError(t, v1Manager.CreateCache(exec), "exec=%t", exec)
	}
	require.False(t, v1Called)

	wantErr := xerrors.NewOrdinary("create cache error")
	var gotExec []bool
	v2Stub := &managerStateLedgerStub{
		createCacheFn: func(exec bool) xerrors.XError {
			gotExec = append(gotExec, exec)
			return wantErr
		},
	}
	v2Manager := &StateLedgerManager{active: v2Stub, activeVersion: LedgerV2}
	for _, exec := range []bool{false, true} {
		xerr := v2Manager.CreateCache(exec)
		require.Equal(t, wantErr, xerr, "exec=%t", exec)
	}
	require.Equal(t, []bool{false, true}, gotExec)
}

func TestStateLedgerManager_WriteCache(t *testing.T) {
	v1Called := false
	v1Stub := &managerStateLedgerStub{
		writeCacheFn: func(bool) xerrors.XError {
			v1Called = true
			return xerrors.NewOrdinary("must not be called")
		},
	}
	v1Manager := &StateLedgerManager{active: v1Stub, activeVersion: LedgerV1}
	for _, exec := range []bool{false, true} {
		require.NoError(t, v1Manager.WriteCache(exec), "exec=%t", exec)
	}
	require.False(t, v1Called)

	wantErr := xerrors.NewOrdinary("write cache error")
	var gotExec []bool
	v2Stub := &managerStateLedgerStub{
		writeCacheFn: func(exec bool) xerrors.XError {
			gotExec = append(gotExec, exec)
			return wantErr
		},
	}
	v2Manager := &StateLedgerManager{active: v2Stub, activeVersion: LedgerV2}
	for _, exec := range []bool{false, true} {
		xerr := v2Manager.WriteCache(exec)
		require.Equal(t, wantErr, xerr, "exec=%t", exec)
	}
	require.Equal(t, []bool{false, true}, gotExec)
}

func TestStateLedgerManager_ClearCache(t *testing.T) {
	v1Called := false
	v1Stub := &managerStateLedgerStub{
		clearCacheFn: func(bool) xerrors.XError {
			v1Called = true
			return xerrors.NewOrdinary("must not be called")
		},
	}
	v1Manager := &StateLedgerManager{active: v1Stub, activeVersion: LedgerV1}
	for _, exec := range []bool{false, true} {
		require.NoError(t, v1Manager.ClearCache(exec), "exec=%t", exec)
	}
	require.False(t, v1Called)

	wantErr := xerrors.NewOrdinary("clear cache error")
	var gotExec []bool
	v2Stub := &managerStateLedgerStub{
		clearCacheFn: func(exec bool) xerrors.XError {
			gotExec = append(gotExec, exec)
			return wantErr
		},
	}
	v2Manager := &StateLedgerManager{active: v2Stub, activeVersion: LedgerV2}
	for _, exec := range []bool{false, true} {
		xerr := v2Manager.ClearCache(exec)
		require.Equal(t, wantErr, xerr, "exec=%t", exec)
	}
	require.Equal(t, []bool{false, true}, gotExec)
}

func TestStateLedgerManager_Commit(t *testing.T) {
	wantHash := []byte("hash")
	wantVersion := int64(23)
	wantErr := xerrors.NewOrdinary("commit error")
	called := 0
	stub := &managerStateLedgerStub{
		commitFn: func() ([]byte, int64, xerrors.XError) {
			called++
			return wantHash, wantVersion, wantErr
		},
	}
	manager := &StateLedgerManager{active: stub, activeVersion: LedgerV2}

	hash, version, xerr := manager.Commit()

	require.Equal(t, wantHash, hash)
	require.Equal(t, wantVersion, version)
	require.Equal(t, wantErr, xerr)
	require.Equal(t, 1, called)
}

func TestStateLedgerManager_Close(t *testing.T) {
	closeCalls := 0
	stub := &managerStateLedgerStub{
		closeFn: func() xerrors.XError {
			closeCalls++
			return nil
		},
	}
	manager := &StateLedgerManager{active: stub, activeVersion: LedgerV2}

	require.NoError(t, manager.Close())
	require.Nil(t, manager.active)
	require.Equal(t, common.LedgerNone, manager.ActiveLedgerVersion())
	require.Equal(t, 1, closeCalls)

	require.NoError(t, manager.Close())
	require.Equal(t, 1, closeCalls)
}

func TestStateLedgerManager_Close_Error(t *testing.T) {
	wantErr := xerrors.NewOrdinary("close error")
	stub := &managerStateLedgerStub{
		closeFn: func() xerrors.XError { return wantErr },
	}
	manager := &StateLedgerManager{active: stub, activeVersion: LedgerV2}

	xerr := manager.Close()

	require.Equal(t, wantErr, xerr)
	require.Same(t, stub, manager.active)
	require.Equal(t, LedgerV2, manager.ActiveLedgerVersion())
}

func TestStateLedgerManager_ImitableLedgerAt(t *testing.T) {
	wantHeight := int64(19)
	wantLedger := &managerImitableLedgerStub{}
	wantErr := xerrors.NewOrdinary("imitable ledger error")
	var gotHeight int64
	stub := &managerStateLedgerStub{
		imitableLedgerAtFn: func(height int64) (IImitable, xerrors.XError) {
			gotHeight = height
			return wantLedger, wantErr
		},
	}
	manager := &StateLedgerManager{active: stub, activeVersion: LedgerV2}

	imitable, xerr := manager.ImitableLedgerAt(wantHeight)

	require.Equal(t, wantHeight, gotHeight)
	require.Same(t, wantLedger, imitable)
	require.Equal(t, wantErr, xerr)
}

func TestStateLedgerManager_ClosedOperations(t *testing.T) {
	manager := &StateLedgerManager{activeVersion: common.LedgerNone}
	item := newManagerTestItem("key", "value")
	operations := []struct {
		name string
		call func() xerrors.XError
	}{
		{name: "Get", call: func() xerrors.XError { _, xerr := manager.Get(item.key, true); return xerr }},
		{name: "Iterate", call: func() xerrors.XError {
			return manager.Iterate(func(LedgerKey, ILedgerItem) xerrors.XError { return nil }, true)
		}},
		{name: "Seek", call: func() xerrors.XError {
			return manager.Seek(nil, true, func(LedgerKey, ILedgerItem) xerrors.XError { return nil }, true)
		}},
		{name: "Set", call: func() xerrors.XError { return manager.Set(item.key, item, true) }},
		{name: "Del", call: func() xerrors.XError { return manager.Del(item.key, true) }},
		{name: "CreateCache", call: func() xerrors.XError { return manager.CreateCache(true) }},
		{name: "WriteCache", call: func() xerrors.XError { return manager.WriteCache(true) }},
		{name: "ClearCache", call: func() xerrors.XError { return manager.ClearCache(true) }},
		{name: "Commit", call: func() xerrors.XError { _, _, xerr := manager.Commit(); return xerr }},
		{name: "ImitableLedgerAt", call: func() xerrors.XError { _, xerr := manager.ImitableLedgerAt(0); return xerr }},
	}

	for _, operation := range operations {
		xerr := operation.call()
		require.ErrorContains(t, xerr, "state ledger manager is closed", "operation=%s", operation.name)
	}
}

func TestStateLedgerManager_openV1(t *testing.T) {
	manager := newUnopenedManagerForTest(t.TempDir())

	active, xerr := manager.openV1()

	require.NoError(t, xerr)
	require.IsType(t, &v1.StateLedger{}, active)
	require.NoError(t, active.Close())
}

func TestStateLedgerManager_openV2(t *testing.T) {
	manager := newUnopenedManagerForTest(t.TempDir())

	active, xerr := manager.openV2()

	require.NoError(t, xerr)
	require.IsType(t, &v2.StateLedger{}, active)
	require.NoError(t, active.Close())
}

func TestStateLedgerManager_migrateToV2(t *testing.T) {
	manager := newManagerTestLedger(t, t.TempDir())
	item := newManagerTestItem("key", "migration-value")
	require.NoError(t, manager.Set(item.key, item, true))
	_, versionBefore, xerr := manager.Commit()
	require.NoError(t, xerr)

	manager.mtx.Lock()
	xerr = manager.migrateToV2()
	manager.mtx.Unlock()

	require.NoError(t, xerr)
	require.IsType(t, &v2.StateLedger{}, manager.active)
	require.Equal(t, LedgerV2, manager.ActiveLedgerVersion())
	require.Equal(t, versionBefore, manager.Version())
	got, xerr := manager.Get(item.key, false)
	require.NoError(t, xerr)
	require.Equal(t, item.key, got.(*managerTestItem).key)
	require.Equal(t, item.value, got.(*managerTestItem).value)
}

func TestStateLedgerManager_migrateToV2_CloseError(t *testing.T) {
	wantErr := xerrors.NewOrdinary("close error")
	stub := &managerStateLedgerStub{
		closeFn: func() xerrors.XError { return wantErr },
	}
	manager := &StateLedgerManager{active: stub, activeVersion: LedgerV1}

	manager.mtx.Lock()
	xerr := manager.migrateToV2()
	manager.mtx.Unlock()

	require.Equal(t, wantErr, xerr)
	require.Same(t, stub, manager.active)
	require.Equal(t, LedgerV1, manager.ActiveLedgerVersion())
}

func TestStateLedgerManager_migrateToV2_OpenError(t *testing.T) {
	badDBDir := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(badDBDir, []byte("file"), 0o600))
	closeCalls := 0
	stub := &managerStateLedgerStub{
		closeFn: func() xerrors.XError {
			closeCalls++
			return nil
		},
	}
	manager := newUnopenedManagerForTest(badDBDir)
	manager.active = stub
	manager.activeVersion = LedgerV1

	manager.mtx.Lock()
	xerr := manager.migrateToV2()
	manager.mtx.Unlock()

	require.Error(t, xerr)
	require.Equal(t, 1, closeCalls)
	require.Nil(t, manager.active)
	require.Equal(t, common.LedgerNone, manager.ActiveLedgerVersion())
}

var _ ILedgerItem = (*managerTestItem)(nil)
var _ IStateLedger = (*managerStateLedgerStub)(nil)
var _ IImitable = (*managerImitableLedgerStub)(nil)
