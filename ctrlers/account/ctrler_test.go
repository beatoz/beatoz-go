package account

import (
	"errors"
	"fmt"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

var (
	acctCtrlers []*AcctCtrler
	addrs       []types.Address
)

func initialize() error {
	acctCtrlers = nil
	for i := 0; i < 10; i++ {
		config := cfg.DefaultConfig()
		config.DBPath = filepath.Join(os.TempDir(), fmt.Sprintf("testnode-%d", i))
		_ = os.RemoveAll(config.DBDir())

		ctrler, err := NewAcctCtrler(config, tmlog.NewNopLogger())
		if err != nil {
			return err
		}
		acctCtrlers = append(acctCtrlers, ctrler)
	}

	addrs = nil
	for i := 0; i < 100; i++ {
		addr := bytes.RandBytes(types.AddrSize)
		for j := 0; j < len(acctCtrlers); j++ {
			acct := acctCtrlers[j].FindOrNewAccount(addr, true)
			_ = acct.AddBalance(uint256.NewInt(1000000000))
			_ = acctCtrlers[j].SetAccount(acct, true)
		}
		addrs = append(addrs, addr)
	}
	if _, _, err := commit(); err != nil {
		return err
	}

	return nil
}

func commit() ([]byte, int64, error) {
	var preAppHash, appHash []byte
	var preVer, ver int64
	var xerr xerrors.XError
	for j := 0; j < len(acctCtrlers); j++ {
		appHash, ver, xerr = acctCtrlers[j].Commit()
		if xerr != nil {
			return nil, -1, xerr
		}
		if preAppHash != nil && bytes.Compare(preAppHash, appHash) != 0 {
			return nil, -1, errors.New("appHash is not same")
		}
		if preVer != 0 && preVer != ver {
			return nil, -1, errors.New("version is not same")
		}
		preAppHash = appHash
		preVer = ver
	}
	return appHash, ver, nil
}

func simuRand(n int) error {
	for i := 0; i < n; i++ {
		r0, r1, r2 := rand.Intn(len(addrs)), rand.Intn(len(addrs)), rand.Intn(len(acctCtrlers))
		addr0, addr1, ctrler := addrs[r0], addrs[r1], acctCtrlers[r2]

		if err := ctrler.Transfer(addr0, addr1, uint256.NewInt(1), false); err != nil {
			return err
		}
	}
	return nil
}
func execRand(n int) error {
	for i := 0; i < n; i++ {
		r0, r1, r2 := rand.Intn(len(addrs)), rand.Intn(len(addrs)), rand.Intn(len(acctCtrlers))
		addr0, addr1, ctrler := addrs[r0], addrs[r1], acctCtrlers[r2]

		if err := ctrler.Transfer(addr0, addr1, uint256.NewInt(1), true); err != nil {
			return err
		}
	}
	return nil
}

func execSame(n int) error {
	for i := 0; i < n; i++ {
		r0, r1 := rand.Intn(len(addrs)), rand.Intn(len(addrs))
		addr0, addr1 := addrs[r0], addrs[r1]

		for j := 0; j < len(acctCtrlers); j++ {

			if err := acctCtrlers[j].Transfer(addr0, addr1, uint256.NewInt(1), true); err != nil {
				return err
			}
		}
	}
	return nil
}

func TestAcctCtrler_Commit(t *testing.T) {
	var preHash []byte
	var preVer int64

	require.NoError(t, initialize())
	for i := 0; i < 100; i++ {
		// simulation 의 경우 각 노드(acctCtrler) 이 서로 다른 값을 가져도 상관 없다.
		require.NoError(t, simuRand(100))

		h, v, e := commit()
		require.NoError(t, e)

		// simulation 의 경우 commit 되는 것이 없으므로 hash 변경도 없다.
		if preHash != nil {
			require.Equal(t, preHash, h)
		}
		preHash = h

		if preVer != 0 {
			require.Equal(t, preVer+1, v)
		}
		preVer = v
	}

	require.NoError(t, initialize())
	for i := 0; i < 100; i++ {
		require.NoError(t, execRand(100))

		_, _, e := commit()
		// execution 이 random 으로 실행 (execRand) 되면(각 노드(acctCtrler) 이 서로 다른 살행을 하면) 에러 발생.
		require.Error(t, e)
	}

	require.NoError(t, initialize())
	preHash = nil
	preVer = 0

	for i := 2; i < 100; i++ {
		require.NoError(t, execSame(100))

		h, v, e := commit()
		require.NoError(t, e)

		if preHash != nil {
			require.NotEqual(t, preHash, h)
		}
		preHash = h

		if preVer != 0 {
			require.Equal(t, preVer+1, v)
		}
		preVer = v
	}
}
