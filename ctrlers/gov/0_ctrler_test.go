package gov

import (
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	mockacct "github.com/beatoz/beatoz-go/ctrlers/mocks/acct"
	mockstake "github.com/beatoz/beatoz-go/ctrlers/mocks/stake"
	"github.com/beatoz/beatoz-go/ctrlers/stake"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
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
	acctMock    *mockacct.AcctHandlerMock
	stakeHelper *mockstake.StakeHandlerMock
	govParams0  = ctrlertypes.DefaultGovParams()
	govParams1  = ctrlertypes.Test1GovParams()
	govParams3  = ctrlertypes.Test3GovParams()
	defMinGas   = govParams0.MinTrxGas()
	defGasPrice = govParams0.GasPrice()
)

func init() {
	config.DBPath = filepath.Join(os.TempDir(), "gov-ctrler-test")
	_ = os.RemoveAll(config.DBPath)
	_ = os.MkdirAll(config.DBPath, 0700)

	var err error
	if govCtrler, err = NewGovCtrler(config, tmlog.NewNopLogger()); err != nil {
		panic(err)
	}
	govCtrler.GovParams = *govParams0

	rand.Seed(time.Now().UnixNano())

	acctMock = mockacct.NewAcctHandlerMock(1000)
	acctMock.Iterate(func(idx int, w *web3.Wallet) bool {
		w.GetAccount().SetBalance(uint256.NewInt(100_000))
		return true
	})

	var delegatees []*stake.Delegatee
	for i := 0; i < 14; i++ {
		w := acctMock.GetWallet(i)
		d := &stake.Delegatee{Addr: w.Address(), TotalPower: rand.Int63n(1_000_000)}
		delegatees = append(delegatees, d)
	}

	stakeHelper = mockstake.NewStakeHandlerMock(
		5, // 5 delegatees is only validator.
		delegatees,
	)
	sort.Sort(stake.PowerOrderDelegatees(stakeHelper.Delegatees))

}

func TestMain(m *testing.M) {
	os.MkdirAll(config.DBPath, 0700)

	exitCode := m.Run()

	os.RemoveAll(config.DBPath)

	os.Exit(exitCode)
}

func signTrx(tx *ctrlertypes.Trx, signerAddr types.Address, chainId string) error {
	_, _, err := acctMock.FindWallet(signerAddr).SignTrxRLP(tx, chainId)
	return err
}
