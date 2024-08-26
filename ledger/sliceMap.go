package ledger

import (
	"sync"
)

type slicedMap[T ILedgerItem] struct {
	_slice []T
	_map   map[string]T
	mtx    sync.RWMutex
}

//func NewSlicedMap[T ILedgerItem]() *slicedMap[T] {
//	return &slicedMap[T]{
//		_map: make(map[string]T),
//	}
//}
//
//func (m *slicedMap[T]) Add(value T) error {
//	m.mtx.Lock()
//	defer m.mtx.Unlock()
//
//	if _, ok := m._map[value.Key()]; ok {
//		return errors.New("already exist item")
//	} else {
//		m._slice = append(m._slice, value)
//		m._map[key] = value
//	}
//	return nil
//}
//
//func (m *slicedMap[T]) Find(key string) T {
//	m.mtx.RLock()
//	defer m.mtx.RUnlock()
//
//	return m.find(key)
//}
//
//func (m *slicedMap[T]) find(key string) T {
//	if item, ok := m._map[key]; ok {
//		return item
//	}
//	return nil
//}
//
//func (m *slicedMap[T]) Del(key string) error {
//	m.mtx.Lock()
//	defer m.mtx.Unlock()
//
//	if item := m.find(key); item != nil {
//		delete(m._map, key)
//	}
//	return nil
//}
//
//func sliceDelete(key string, slice []any)
