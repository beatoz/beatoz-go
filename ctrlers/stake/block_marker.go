package stake

import (
	"errors"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"sync"
)

type BlockMarker struct {
	BlockHeights []int64 `json:"blockHeights"`
	mtx          sync.RWMutex
}

func (bm *BlockMarker) Mark(height int64) xerrors.XError {
	bm.mtx.Lock()
	defer bm.mtx.Unlock()

	lastIdx := len(bm.BlockHeights) - 1
	if lastIdx >= 0 && bm.BlockHeights[lastIdx] >= height {
		return xerrors.From(errors.New("height must bigger than last marked height"))
	}

	bm.BlockHeights = append(bm.BlockHeights, height)
	return nil
}

// CountInWindow() returns the number of marked height in [startHeight, endHeight].
// If `rewin` is true, all heights before `h0` is removed after counting.
func (bm *BlockMarker) CountInWindow(h0, h1 int64, rewin bool) int {
	if h0 > h1 {
		return 0
	}

	bm.mtx.RLock()
	defer bm.mtx.RUnlock()

	count := 0
	preIdx := -1
	for i, h := range bm.BlockHeights {
		if h < h0 {
			preIdx = i
		}
		if h >= h0 && h <= h1 {
			count++
		}
		if h >= h1 {
			break
		}
	}

	if rewin && preIdx > 0 {
		bm.BlockHeights = bm.BlockHeights[preIdx+1:]
	}

	return count
}
