package v1

type kvPair[T any] struct {
	key []byte
	val T
}

type revisionList[T any] struct {
	revs []*kvPair[T]
}

func newSnapshotList[T any]() *revisionList[T] {
	return &revisionList[T]{
		revs: make([]*kvPair[T], 0),
	}
}
func (revlist *revisionList[T]) set(key []byte, val T) {
	revlist.revs = append(revlist.revs, &kvPair[T]{
		key: key,
		val: val,
	})
}

func (revlist revisionList[T]) snapshot() int {
	return len(revlist.revs)
}

func (revlist *revisionList[T]) revert(snap int) {
	revlist.revs = revlist.revs[:snap]
}

func (revlist revisionList[T]) reset() {
	revlist.revs = revlist.revs[:0]
}

func (revlist revisionList[T]) iterate(cb func(idx int, kv *kvPair[T]) bool) {
	for i, kv := range revlist.revs {
		if cb(i, kv) == false {
			break
		}
	}
}
