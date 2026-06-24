package v2

import (
	"bytes"
	"sort"
)

type writeOp uint8

const (
	opSet writeOp = iota + 1
	opDel
)

type writeEntry struct {
	key []byte
	val []byte
	op  writeOp
}

type writeSet struct {
	entries map[string]*writeEntry
}

func newWriteSet() *writeSet {
	return &writeSet{
		entries: make(map[string]*writeEntry),
	}
}

func (ws *writeSet) set(key, val []byte) {
	if ws == nil {
		return
	}
	ws.entries[string(key)] = &writeEntry{
		key: cloneBytes(key),
		val: cloneBytes(val),
		op:  opSet,
	}
}

func (ws *writeSet) del(key []byte) {
	if ws == nil {
		return
	}
	ws.entries[string(key)] = &writeEntry{
		key: cloneBytes(key),
		op:  opDel,
	}
}

func (ws *writeSet) get(key []byte) (*writeEntry, bool) {
	if ws == nil {
		return nil, false
	}
	entry, ok := ws.entries[string(key)]
	return entry, ok
}

func (ws *writeSet) reset() {
	if ws == nil {
		return
	}
	ws.entries = make(map[string]*writeEntry)
}

func (ws *writeSet) orderedEntries(ascending bool) []*writeEntry {
	if ws == nil || len(ws.entries) == 0 {
		return nil
	}

	ret := make([]*writeEntry, 0, len(ws.entries))
	for _, entry := range ws.entries {
		ret = append(ret, entry)
	}

	sort.Slice(ret, func(i, j int) bool {
		cmp := bytes.Compare(ret[i].key, ret[j].key)
		if ascending {
			return cmp < 0
		}
		return cmp > 0
	})
	return ret
}

func (ws *writeSet) keysWithPrefix(prefix []byte) [][]byte {
	if ws == nil || len(ws.entries) == 0 {
		return nil
	}

	keys := make([][]byte, 0, len(ws.entries))
	for _, entry := range ws.entries {
		if bytes.HasPrefix(entry.key, prefix) {
			keys = append(keys, cloneBytes(entry.key))
		}
	}

	sort.Slice(keys, func(i, j int) bool {
		return bytes.Compare(keys[i], keys[j]) < 0
	})
	return keys
}

func cloneBytes(src []byte) []byte {
	if src == nil {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}
