package v1

type kvPair struct {
	key []byte
	val []byte
}

type snapshotList struct {
	revisions []*kvPair
}

func NewSnapshotList() *snapshotList {
	return &snapshotList{
		revisions: make([]*kvPair, 0),
	}
}
func (uph *snapshotList) set(key, val []byte) {
	uph.revisions = append(uph.revisions, &kvPair{
		key: key,
		val: val,
	})
}

func (uph snapshotList) snapshot() int {
	return len(uph.revisions)
}

func (uph *snapshotList) revert(snap int) {
	uph.revisions = uph.revisions[:snap]
}

func (uph snapshotList) reset() {
	uph.revisions = uph.revisions[:0]
}

func (uph snapshotList) iterate(cb func(idx int, kv *kvPair) bool) {
	for i, kv := range uph.revisions {
		if cb(i, kv) == false {
			break
		}
	}
}
