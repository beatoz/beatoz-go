package v2

import (
	"bytes"
	"slices"
)

type writeOp uint8

const (
	opSet writeOp = iota + 1
	opDel
)

type writeEntry struct {
	key  []byte
	item ILedgerItem
	op   writeOp
}

type writeSet struct {
	entries map[string]*writeEntry
}

func newWriteSet() *writeSet {
	return &writeSet{
		entries: make(map[string]*writeEntry),
	}
}

func (ws *writeSet) lookup(key []byte) (*writeEntry, bool) {
	entry, ok := ws.entries[string(key)]
	return entry, ok
}

func (ws *writeSet) set(key []byte, item ILedgerItem) {
	ws.entries[string(key)] = &writeEntry{
		key:  bytes.Clone(key),
		item: item,
		op:   opSet,
	}
}

func (ws *writeSet) del(key []byte) {
	ws.entries[string(key)] = &writeEntry{
		key: bytes.Clone(key),
		op:  opDel,
	}
}

func (ws *writeSet) orderedEntries() []*writeEntry {
	entries := make([]*writeEntry, 0, len(ws.entries))
	for _, entry := range ws.entries {
		entries = append(entries, entry)
	}
	slices.SortFunc(entries, func(a, b *writeEntry) int {
		return bytes.Compare(a.key, b.key)
	})
	return entries
}

func (ws *writeSet) keysWithPrefix(prefix []byte) [][]byte {
	keys := make([][]byte, 0, len(ws.entries))
	for _, entry := range ws.entries {
		if bytes.HasPrefix(entry.key, prefix) {
			keys = append(keys, bytes.Clone(entry.key))
		}
	}
	return keys
}
