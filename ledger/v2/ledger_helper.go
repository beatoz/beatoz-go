package v2

import (
	"bytes"
	"sort"

	corestore "cosmossdk.io/core/store"
	"github.com/beatoz/beatoz-go/types/xerrors"
)

func collectKeys(iter corestore.Iterator, prefix []byte, ascending bool, keySet map[string][]byte) xerrors.XError {
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if bytes.HasPrefix(key, prefix) {
			keySet[string(key)] = bytes.Clone(key)
		} else if ascending {
			break
		}
	}
	if err := iter.Error(); err != nil {
		_ = iter.Close()
		return xerrors.From(err)
	}
	if err := iter.Close(); err != nil {
		return xerrors.From(err)
	}
	return nil
}

func sortKeys(keySet map[string][]byte, ascending bool) [][]byte {
	keys := make([][]byte, 0, len(keySet))
	for _, key := range keySet {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		ret := bytes.Compare(keys[i], keys[j])
		if ascending {
			return ret < 0
		}
		return ret > 0
	})
	return keys
}
