package gov

import (
	cfg "github.com/beatoz/beatoz-go/cmd/config"
	mockacct "github.com/beatoz/beatoz-go/ctrlers/mocks/acct"
	govmock "github.com/beatoz/beatoz-go/ctrlers/mocks/gov"
	mockvpower "github.com/beatoz/beatoz-go/ctrlers/mocks/vpower"
	ctrlertypes "github.com/beatoz/beatoz-go/ctrlers/types"
	"github.com/beatoz/beatoz-go/types"
	"github.com/beatoz/beatoz-sdk-go/web3"
	"github.com/holiman/uint256"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	config      = cfg.DefaultConfig()
	govCtrler   *GovCtrler
	acctMock    *mockacct.AcctHandlerMock
	vpowMock    *mockvpower.VPowerHandlerMock //*mockstake.StakeHandlerMock
	govParams0  = ctrlertypes.DefaultGovParams()
	govParams1  = govmock.ForTest1GovParams()
	govParams3  = govmock.ForTest3GovParams()
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

	var dWals []*web3.Wallet
	for i := 0; i < 14; i++ {
		w := acctMock.GetWallet(i)
		//d := &stake.Delegatee{Addr: w.Address(), TotalPower: rand.Int63n(1_000_000)}
		dWals = append(dWals, w)
	}

	vpowMock = mockvpower.NewVPowerHandlerMock(dWals, 5)
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
