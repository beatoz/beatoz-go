package version

import (
	"fmt"
	"github.com/tendermint/tendermint/version"
	"regexp"
	"strconv"
)

const (
	FMT_VERSTR      = "v%v.%v.%v-%x@%s"
	MASK_MAJOR_VER  = uint64(0xFF00000000000000)
	MASK_MINOR_VER  = uint64(0x00FF000000000000)
	MASK_PATCH_VER  = uint64(0x0000FFFF00000000)
	MASK_COMMIT_VER = uint64(0x00000000FFFFFFFF)
)

var (
	// it is changed using ldflags.
	//  ex) -ldflags "... -X 'github.com/beatoz/beatoz-go/cmd/version.GitCommit=$(XXX)'"
	Version   string
	GitCommit string

	majorVer  uint64 = 0
	minorVer  uint64 = 11
	patchVer  uint64 = 1
	commitVer uint64 = 0
)

func init() {
	parseVersions(Version, GitCommit)
}

func parseVersions(vers ...string) {
	versionStr := vers[0]
	gitCommit := vers[1]

	if versionStr == "" {
		return
	}

	re := regexp.MustCompile(`v(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(versionStr)
	if matches == nil {
		panic(fmt.Errorf("invalid version string: %v", versionStr))
	}
	majorVer, _ = strconv.ParseUint(matches[1], 10, 64)
	minorVer, _ = strconv.ParseUint(matches[2], 10, 64)
	patchVer, _ = strconv.ParseUint(matches[3], 10, 64)

	if gitCommit != "" {
		var err error
		commitVer, err = strconv.ParseUint(gitCommit, 16, 64)
		if err != nil {
			panic(fmt.Errorf("error: %v, invalid git commit: %v", err, gitCommit))
		}
	}
}

func String() string {
	return fmt.Sprintf(FMT_VERSTR, majorVer, minorVer, patchVer, commitVer, version.TMCoreSemVer)
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
