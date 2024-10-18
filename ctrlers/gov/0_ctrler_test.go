package gov

import (
	"bytes"
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	"github.com/beatoz/beatoz-go/ctrlers/stake"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/libs/web3"
	"github.com/beatoz/beatoz-go/types"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

var (
	config      = cfg.DefaultConfig()
	govCtrler   *GovCtrler
	stakeHelper *stakeHandlerMock
	acctHelper  *acctHelperMock
	govParams0  = ctrlertypes.DefaultGovParams()
	govParams1  = ctrlertypes.Test1GovParams()
	govParams2  = ctrlertypes.Test2GovParams()
	govParams3  = ctrlertypes.Test3GovParams()
	govParams4  = ctrlertypes.Test4GovParams()
	govParams5  = ctrlertypes.Test5GovParams()
	defMinGas   = govParams0.MinTrxGas()
	defGasPrice = govParams0.GasPrice()

	wallets []*web3.Wallet
)

func init() {
	config.DBPath = filepath.Join(os.TempDir(), "gov-ctrler-test")
	os.RemoveAll(config.DBPath)
	os.MkdirAll(config.DBPath, 0700)

	var err error
	if govCtrler, err = NewGovCtrler(config, tmlog.NewNopLogger()); err != nil {
		panic(err)
	}
	govCtrler.GovParams = *govParams0

	rand.Seed(time.Now().UnixNano())

	var delegatees []*stake.Delegatee
	for i := 0; i < 14; i++ {
		w := web3.NewWallet(nil)
		wallets = append(wallets, w)

		d := &stake.Delegatee{Addr: w.Address(), TotalPower: rand.Int63n(1000000)}
		delegatees = append(delegatees, d)
	}

	stakeHelper = &stakeHandlerMock{
		valCnt:     5, // 5 delegatees is only validator.
		delegatees: delegatees,
	}
	sort.Sort(stake.PowerOrderDelegatees(stakeHelper.delegatees))

	acctHelper = &acctHelperMock{
		acctMap: make(map[ctrlertypes.AcctKey]*ctrlertypes.Account),
	}
}

func TestMain(m *testing.M) {
	os.MkdirAll(config.DBPath, 0700)

	exitCode := m.Run()

	os.RemoveAll(config.DBPath)

	os.Exit(exitCode)
}

func findWallet(address types.Address) *web3.Wallet {
	for _, w := range wallets {
		if bytes.Compare(w.Address(), address) == 0 {
			return w
		}
	}
	return nil
}
func signTrx(tx *ctrlertypes.Trx, signerAddr types.Address, chainId string) error {
	_, _, err := findWallet(signerAddr).SignTrxRLP(tx, chainId)
	return err
}
