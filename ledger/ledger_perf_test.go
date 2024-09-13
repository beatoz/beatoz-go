package ledger

import (
	"encoding/binary"
	"fmt"
	v0 "github.com/beatoz/beatoz-go/ledger/v0"
	v1 "github.com/beatoz/beatoz-go/ledger/v1"
	"github.com/beatoz/beatoz-go/types/bytes"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	"os"
	"strconv"
	"strings"
	"testing"
)

var (
	testItemV0s      []*TestItemV0
	testItemV1s      []*TestItemV1
	cntWirttenItemV0 = 0
	cntWirttenItemV1 = 0
)

func init() {
	for i := 0; i < 100_000; i++ {
		testItemV0s = append(testItemV0s, newTestItemV0(i, bytes.RandHexString(512)))
	}
	for i := 0; i < 100_000; i++ {
		testItemV1s = append(testItemV1s, newTestItemV1(i, bytes.RandHexString(512)))
	}
}

func Benchmark_Set_V0(b *testing.B) {
	dbDir, err := os.MkdirTemp("", "ledger_performance_test_set_v0")
	require.NoError(b, err)
	ledger, err := v0.NewFinalityLedger("ledgerV0", dbDir, 100_000, emptyTestItemV0)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		require.NoError(b, ledger.SetFinality(testItemV0s[i%len(testItemV0s)]))
		cntWirttenItemV0 = i + 1
	}
	b.StopTimer()

	//_, _, err = ledger.Commit()
	//require.NoError(b, err)
	require.NoError(b, ledger.Close())
}

func Benchmark_Set_V1(b *testing.B) {
	dbDir, err := os.MkdirTemp("", "ledger_performance_test_set_v1")
	require.NoError(b, err)
	ledger, err := v1.NewMutableLedger("ledgerV1", dbDir, 100_000, emptyTestItemV1, log.NewNopLogger())
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		require.NoError(b, ledger.Set(testItemV1s[i%len(testItemV1s)]))
		cntWirttenItemV1 = i + 1
	}
	b.StopTimer()

	//_, _, err = ledger.Commit()
	//require.NoError(b, err)
	require.NoError(b, ledger.Close())
}

func Benchmark_Get_V0(b *testing.B) {
	dbDir, err := os.MkdirTemp("", "ledger_performance_test_get_v0")
	require.NoError(b, err)
	ledger, err := v0.NewFinalityLedger("ledgerV0", dbDir, 100_000, emptyTestItemV0)
	require.NoError(b, err)

	cntSavedItems := min(cntWirttenItemV0, len(testItemV0s))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ledger.GetFinality(testItemV0s[i%cntSavedItems].Key())
		require.NoError(b, err, fmt.Sprintf("tyr to read %d/%d, key:%x", i%cntSavedItems, cntSavedItems, testItemV0s[i%cntSavedItems].Key()))
	}

	b.StopTimer()
	require.NoError(b, ledger.Close())
}

func Benchmark_Get_V1(b *testing.B) {
	dbDir, err := os.MkdirTemp("", "ledger_performance_test_get_v1")
	require.NoError(b, err)
	ledger, err := v1.NewMutableLedger("ledgerV1", dbDir, 100_000, emptyTestItemV1, log.NewNopLogger())
	require.NoError(b, err)

	cntSavedItems := min(cntWirttenItemV1, len(testItemV0s))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ledger.Get(testItemV1s[i%cntSavedItems].Key())
		require.NoError(b, err, fmt.Sprintf("tyr to read %d/%d, key:%x", i%cntSavedItems, cntSavedItems, testItemV1s[i%cntSavedItems].Key()))
	}

	b.StopTimer()
	require.NoError(b, ledger.Close())
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
		for j := 0; j < 10_000; j++ {
			item := newTestItemV0(j, bytes.RandHexString(512))
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
		for j := 0; j < 10_000; j++ {
			item := newTestItemV1(j, bytes.RandHexString(512))
			require.NoError(b, ledger.Set(item))
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
	key  int
	data string
}

func newTestItemV0(key int, data string) *TestItemV0 {
	return &TestItemV0{
		key:  key,
		data: data,
	}
}
func emptyTestItemV0() *TestItemV0 {
	return &TestItemV0{}
}

func (i *TestItemV0) Key() v0.LedgerKey {
	var bs [32]byte
	binary.BigEndian.PutUint32(bs[:], uint32(i.key))
	return bs
}

func (i *TestItemV0) Encode() ([]byte, xerrors.XError) {
	return []byte(fmt.Sprintf("key:%v,data:%v", i.key, i.data)), nil
}

func (i *TestItemV0) Decode(bz []byte) xerrors.XError {
	toks := strings.Split(string(bz), ",")
	key, _ := strings.CutPrefix(toks[0], "key:")
	data, _ := strings.CutPrefix(toks[1], "data:")

	var err error
	if i.key, err = strconv.Atoi(key); err != nil {
		return xerrors.From(err)
	}
	i.data = data
	return nil
}

type TestItemV1 struct {
	*TestItemV0
}

func newTestItemV1(key int, data string) *TestItemV1 {
	return &TestItemV1{
		TestItemV0: newTestItemV0(key, data),
	}
}

func emptyTestItemV1() v1.ILedgerItem {
	return &TestItemV1{
		TestItemV0: &TestItemV0{},
	}
}

func (i *TestItemV1) Key() v1.LedgerKey {
	bs := make([]byte, 32)
	binary.BigEndian.PutUint32(bs, uint32(i.key))
	return bs
}