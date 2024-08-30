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
	dbDir       string
	testItemV0s []*TestItemV0
	testItemV1s []*TestItemV1
)

func init() {
	if _dir, err := os.MkdirTemp("", "ledger_performance_test"); err != nil {
		panic(err)
	} else {
		dbDir = _dir
	}

	for i := 0; i < 1000000; i++ {
		testItemV0s = append(testItemV0s, newTestItemV0(i, bytes.RandHexString(512)))
	}
	for i := 0; i < 1000000; i++ {
		testItemV1s = append(testItemV1s, newTestItemV1(i, bytes.RandHexString(512)))
	}
}

func Benchmark_Encode_V0(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := testItemV0s[i%len(testItemV0s)].Encode()
		require.NoError(b, err)
	}
}

func Benchmark_Encode_V1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := testItemV0s[i%len(testItemV1s)].Encode()
		require.NoError(b, err)
	}
}

func Benchmark_Set_V0(b *testing.B) {
	b.StopTimer()
	ledger, err := v0.NewFinalityLedger("ledgerV0", dbDir, 1_000_000, emptyTestItemV0)
	require.NoError(b, err)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		require.NoError(b, ledger.SetFinality(testItemV0s[i%len(testItemV0s)]))
	}

	//_start := time.Now()
	_, _, err = ledger.Commit()
	//_elapsed := time.Since(_start)
	//fmt.Println("V0 elapsed time", _elapsed)
	require.NoError(b, err)

	b.StopTimer()
	require.NoError(b, ledger.Close())
}

func Benchmark_Set_V1(b *testing.B) {
	b.StopTimer()
	ledger, err := v1.NewLedger("ledgerV1", dbDir, 1_000_000, emptyTestItemV1, log.NewNopLogger())
	require.NoError(b, err)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		require.NoError(b, ledger.Set(testItemV1s[i%len(testItemV1s)]))
	}

	//_start := time.Now()
	_, _, err = ledger.Commit()
	//_elapsed := time.Since(_start)
	//fmt.Println("V1 elapsed time", _elapsed)
	require.NoError(b, err)

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
	bs := make([]byte, 4)
	binary.BigEndian.PutUint32(bs, uint32(i.key))
	return bs
}
