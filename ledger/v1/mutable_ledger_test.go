package v1

import (
	"encoding/binary"
	"fmt"
	"github.com/beatoz/beatoz-go/types/xerrors"
	"strconv"
	"strings"
)

type Item struct {
	key  int
	data string
}

func newItem(key int, data string) *Item {
	return &Item{
		key:  key,
		data: data,
	}
}

func (i *Item) Key() []byte {
	bs := make([]byte, 4)
	binary.BigEndian.PutUint32(bs, uint32(i.key))
	return bs
}

func (i *Item) Encode() ([]byte, xerrors.XError) {
	return []byte(fmt.Sprintf("key:%v,data:%v", i.key, i.data)), nil
}

func (i *Item) Decode(k, v []byte) xerrors.XError {
	toks := strings.Split(string(v), ",")
	key, _ := strings.CutPrefix(toks[0], "key:")
	data, _ := strings.CutPrefix(toks[1], "data:")

	var err error
	if i.key, err = strconv.Atoi(key); err != nil {
		return xerrors.From(err)
	}
	i.data = data
	return nil
}

var _ ILedgerItem = (*Item)(nil)
