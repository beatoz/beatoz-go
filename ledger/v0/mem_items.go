package v0

import "github.com/beatoz/beatoz-go/types/xerrors"

type memItems[T ILedgerItem] struct {
	gotItems     map[LedgerKey]T
	updatedItems map[LedgerKey]T
	removedKeys  []LedgerKey
}

func newMemItems[T ILedgerItem]() *memItems[T] {
	return &memItems[T]{
		gotItems:     make(map[LedgerKey]T),
		updatedItems: make(map[LedgerKey]T),
	}
}

func (m *memItems[T]) appendRemovedKey(key LedgerKey) {
	m.removedKeys = append(m.removedKeys, key)
}

func (m *memItems[T]) isRemovedKey(key LedgerKey) bool {
	for _, k := range m.removedKeys {
		if key == k {
			return true
		}
	}
	return false
}

//func (m *memItems[T]) set(item T) xerrors.XError {
//	m.gotItems[item.Key()] = item
//	m.updatedItems[item.Key()] = item
//	return nil
//}

func (m *memItems[T]) setGotItem(item T) {
	m.gotItems[item.Key()] = item
}

func (m *memItems[T]) setUpdatedItem(item T) {
	m.updatedItems[item.Key()] = item
}

func (m *memItems[T]) getGotItem(key LedgerKey) (T, bool) {
	item, ok := m.gotItems[key]
	return item, ok
}

func (m *memItems[T]) getUpdatedItem(key LedgerKey) (T, bool) {
	item, ok := m.updatedItems[key]
	return item, ok
}

func (m *memItems[T]) delGotItem(key LedgerKey) T {
	item, ok := m.gotItems[key]
	if ok {
		delete(m.gotItems, key)
	}
	return item
}

func (m *memItems[T]) delUpdatedItem(key LedgerKey) T {
	item, ok := m.updatedItems[key]
	if ok {
		delete(m.updatedItems, key)
	}
	return item
}

func (m *memItems[T]) delRemovedKey(key LedgerKey) {
	for i, key0 := range m.removedKeys {
		if key0 == key {
			m.removedKeys = append(m.removedKeys[:i], m.removedKeys[i+1:]...)
			return
		}
	}
}

func (m *memItems[T]) iterateGotItems(cb func(T) xerrors.XError) xerrors.XError {
	return iterateItems(m.gotItems, cb)
}

func (m *memItems[T]) iterateUpdatedItems(cb func(T) xerrors.XError) xerrors.XError {
	return iterateItems(m.updatedItems, cb)
}

func (m *memItems[T]) iterateRemovedKeys(cb func(LedgerKey) xerrors.XError) xerrors.XError {
	for _, key := range m.removedKeys {
		var vk LedgerKey
		copy(vk[:], key[:])
		if xerr := cb(key); xerr != nil {
			return xerr
		}
	}
	return nil
}

func (m *memItems[T]) reset() {
	m.gotItems = make(map[LedgerKey]T)
	m.updatedItems = make(map[LedgerKey]T)
	m.removedKeys = nil
}

func (m *memItems[T]) refresh() {
	for k, v := range m.updatedItems {
		m.gotItems[k] = v
	}
	m.updatedItems = make(map[LedgerKey]T)
	m.removedKeys = nil
}
