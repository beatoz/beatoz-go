package ledger

import (
	"encoding/hex"
	"fmt"
	v0 "github.com/beatoz/beatoz-go/ledger/v0"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	"os"
	"strings"
	"testing"
)

var (
	testItemsV0 []*TestItemV0
	testItemsV1 []*TestItemV1
	dbDirV0     string
	dbDirV1     string
)

func init() {
	fmt.Println("Running init()...")
	for i := 0; i < 100_000; i++ {
		testItemsV0 = append(testItemsV0, newTestItemV0(bytes.RandHexString(512)))
	}
	for i := 0; i < 100_000; i++ {
		testItemsV1 = append(testItemsV1, newTestItemV1(bytes.RandHexString(512)))
	}
}

func Benchmark_Set_V0(b *testing.B) {
	dbDir, err := os.MkdirTemp("", "ledger_performance_test_setget_v0")
	require.NoError(b, err)
	ledger, err := v0.NewFinalityLedger("ledgerV0", dbDir, 100_000, emptyTestItemV0)
	require.NoError(b, err)

	dbDirV0 = dbDir

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		require.NoError(b, ledger.SetFinality(testItemsV0[i%len(testItemsV0)]))
	}
	b.StopTimer()

	_, _, err = ledger.Commit()
	require.NoError(b, err)
	require.NoError(b, ledger.Close())
}

func Benchmark_Set_V1(b *testing.B) {
	dbDir, err := os.MkdirTemp("", "ledger_performance_test_setget_v1")
	require.NoError(b, err)
	ledger, err := v1.NewMutableLedger("ledgerV1", dbDir, 100_000, emptyTestItemV1, log.NewNopLogger())
	require.NoError(b, err)

	dbDirV1 = dbDir

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it := testItemsV1[i%len(testItemsV1)]
		require.NoError(b, ledger.Set(it.Key(), it))
	}
	b.StopTimer()

	_, _, err = ledger.Commit()
	require.NoError(b, err)
	require.NoError(b, ledger.Close())
}

func Benchmark_Set_V1_Mem(b *testing.B) {
	_ledger, err := v1.NewMutableLedger("ledgerV1", dbDirV1, 100_000, emptyTestItemV1, log.NewNopLogger())
	require.NoError(b, err)

	ledger, err := v1.NewMemLedgerAt(_ledger.Version(), _ledger, log.NewNopLogger())
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		it := testItemsV1[i%len(testItemsV1)]
		require.NoError(b, ledger.Set(it.Key(), it))
	}
	b.StopTimer()

	require.NoError(b, err)
	require.NoError(b, _ledger.Close())
}

// Benchmark_Get_V0
// To run this benchmark, Benchmark_Set_V0 must have already been run.
func Benchmark_Get_V0(b *testing.B) {
	ledger, err := v0.NewFinalityLedger("ledgerV0", dbDirV0, 100_000, emptyTestItemV0)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ledger.GetFinality(testItemsV0[i%len(testItemsV0)].Key())
		require.NoError(b, err, fmt.Sprintf("tyr to read item[%d], key:%x", i%len(testItemsV0), testItemsV0[i%len(testItemsV0)].Key()))
	}

	b.StopTimer()
	require.NoError(b, ledger.Close())
}

// Benchmark_Get_V1
// To run this benchmark, Benchmark_Set_V1 must have already been run.
func Benchmark_Get_V1(b *testing.B) {
	ledger, err := v1.NewMutableLedger("ledgerV1", dbDirV1, 100_000, emptyTestItemV1, log.NewNopLogger())
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ledger.Get(testItemsV1[i%len(testItemsV1)].Key())
		require.NoError(b, err, fmt.Sprintf("tyr to read items[%d], key:%x", i%len(testItemsV1), testItemsV1[i%len(testItemsV1)].Key()))
	}

	b.StopTimer()
	require.NoError(b, ledger.Close())
}

func Benchmark_Get_V1_Mem(b *testing.B) {
	_ledger, err := v1.NewMutableLedger("ledgerV1", dbDirV1, 100_000, emptyTestItemV1, log.NewNopLogger())
	require.NoError(b, err)

	ledger, err := v1.NewMemLedgerAt(_ledger.Version(), _ledger, log.NewNopLogger())
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ledger.Get(testItemsV1[i%len(testItemsV1)].Key())
		require.NoError(b, err, fmt.Sprintf("tyr to read items[%d], key:%x", i%len(testItemsV1), testItemsV1[i%len(testItemsV1)].Key()))
	}

	b.StopTimer()
	require.NoError(b, _ledger.Close())
}

func Benchmark_Commit_V0(b *testing.B) {
	dbDir, err := os.MkdirTemp("", "ledger_performance_test_commit_v0_*")
	require.NoError(b, err)
	ledger, err := v0.NewFinalityLedger("ledgerV0", dbDir, 100_000, emptyTestItemV0)
	require.NoError(b, err)

	totalTxs := int64(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// set test data
		for j := 0; j < 20_000; j++ {
			item := newTestItemV0(bytes.RandHexString(512))
			require.NoError(b, ledger.SetFinality(item))
			totalTxs++
		}

		b.StartTimer()
		_, _, err = ledger.Commit()
		require.NoError(b, err)
	}
	b.StopTimer()
	require.NoError(b, ledger.Close())
}

func Benchmark_Commit_V1(b *testing.B) {
	dbDir, err := os.MkdirTemp("", "ledger_performance_test_commit_v1_*")
	require.NoError(b, err)
	ledger, err := v1.NewMutableLedger("ledgerV1", dbDir, 100_000, emptyTestItemV1, log.NewNopLogger())
	require.NoError(b, err)

	totalTxs := int64(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// set test data
		for j := 0; j < 20_000; j++ {
			item := newTestItemV1(bytes.RandHexString(512))
			require.NoError(b, ledger.Set(item.Key(), item))
			totalTxs++
		}

		b.StartTimer()
		_, _, err = ledger.Commit()
		require.NoError(b, err)
	}
	b.StopTimer()
	require.NoError(b, ledger.Close())
}

type TestItemV0 struct {
	key  []byte
	data string
}

func newTestItemV0(data string) *TestItemV0 {
	return &TestItemV0{
		key:  bytes.RandBytes(32),
		data: data,
	}
}
func emptyTestItemV0() *TestItemV0 {
	return &TestItemV0{}
}

func (i *TestItemV0) Key() v0.LedgerKey {
	var bs [32]byte
	copy(bs[:], i.key)
	return bs
}

func (i *TestItemV0) Encode() ([]byte, xerrors.XError) {
	return []byte(fmt.Sprintf("key:%x,data:%v", i.key, i.data)), nil
}

func (i *TestItemV0) Decode(bz []byte) xerrors.XError {
	toks := strings.Split(string(bz), ",")
	key, _ := strings.CutPrefix(toks[0], "key:")
	data, _ := strings.CutPrefix(toks[1], "data:")

	var err error
	if i.key, err = hex.DecodeString(key); err != nil {
		return xerrors.From(err)
	}
	i.data = data
	return nil
}

type TestItemV1 struct {
	*TestItemV0
}

func newTestItemV1(data string) *TestItemV1 {
	return &TestItemV1{
		TestItemV0: newTestItemV0(data),
	}
}

func emptyTestItemV1() v1.ILedgerItem {
	return &TestItemV1{
		TestItemV0: &TestItemV0{},
	}
}

func (i *TestItemV1) Key() v1.LedgerKey {
	return i.key
}
