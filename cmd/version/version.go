package version

import (
	"fmt"
	"github.com/tendermint/tendermint/version"
	"strconv"
)

const (
	FMT_VERSTR = "%v.%v.%v-%x@%s"
)

var (
	majorVer  uint64 = 1
	minorVer  uint64 = 3
	patchVer  uint64 = 19
	commitVer uint64 = 0

	// it is changed using ldflags.
	//  ex) -ldflags "... -X 'github.com/beatoz/beatoz-go/cmd/version.GitCommit=$(LVER)'"
	GitCommit string

	MASK_MAJOR_VER  = uint64(0xFF00000000000000)
	MASK_MINOR_VER  = uint64(0x00FF000000000000)
	MASK_PATCH_VER  = uint64(0x0000FFFF00000000)
	MASK_COMMIT_VER = uint64(0x00000000FFFFFFFF)
)

func init() {
	if GitCommit != "" {
		commitVer, _ = strconv.ParseUint(GitCommit, 16, 64)
	}
}

func String() string {
	return fmt.Sprintf(FMT_VERSTR, majorVer, minorVer, patchVer, commitVer, version.TMCoreSemVer)
}

func Uint64(masks ...uint64) uint64 {
	mask := uint64(0)
	if len(masks) == 0 {
		mask = MASK_MAJOR_VER | MASK_MINOR_VER | MASK_PATCH_VER | MASK_COMMIT_VER
	} else {
		for _, m := range masks {
			mask |= m
		}
	}
	return ((majorVer << 56) + (minorVer << 48) + (patchVer << 32) + commitVer) & (mask)
}

func Uint64Formated(masks ...uint64) uint64 {
	mask := uint64(0)
	if len(masks) == 0 {
		mask = MASK_MAJOR_VER | MASK_MINOR_VER | MASK_PATCH_VER | MASK_COMMIT_VER
	} else {
		for _, m := range masks {
			mask |= m
		}
	}

	retVer := uint64(0)
	if mask&MASK_MAJOR_VER != 0 {
		retVer += majorVer * 1_000_000
	}
	if mask&MASK_MINOR_VER != 0 {
		retVer += minorVer * 1_000
	}
	if mask&MASK_PATCH_VER != 0 {
		retVer += patchVer
	}
	return retVer
}

func Parse(c uint64) string {
	return fmt.Sprintf(FMT_VERSTR,
		((c >> 56) & 0xFF),
		((c >> 48) & 0xFF),
		((c >> 32) & 0xFFFF),
		(c & 0xFFFFFFFF),
		version.TMCoreSemVer)
}

func Major() uint64 {
	return majorVer
}

func Minor() uint64 {
	return minorVer
}

func Patch() uint64 {
	return patchVer
}

func CommitHash() uint64 {
	return commitVer
}
