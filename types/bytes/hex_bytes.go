package bytes

import (
	"bytes"
	"encoding/hex"
	"fmt"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"strings"
)

// HexBytes enables HEX-encoding for json/encoding.
type HexBytes tmbytes.HexBytes

// Marshal needed for protobuf compatibility
func (hb HexBytes) Marshal() ([]byte, error) {
	return hb, nil
}

// Unmarshal needed for protobuf compatibility
func (hb *HexBytes) Unmarshal(data []byte) error {
	*hb = data
	return nil
}

// This is the point of Bytes.
func (hb HexBytes) MarshalJSON() ([]byte, error) {
	s := strings.ToUpper(hex.EncodeToString(hb))
	jbz := make([]byte, len(s)+2)
	jbz[0] = '"'
	copy(jbz[1:], s)
	jbz[len(jbz)-1] = '"'
	return jbz, nil
}

// This is the point of Bytes.
func (hb *HexBytes) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid hex string: %s", data)
	}

	// escape double quote
	str := strings.TrimPrefix(string(data[1:len(data)-1]), "0x")
	bz2, err := hex.DecodeString(str)
	if err != nil {
		return err
	}
	*hb = bz2
	return nil
}

// Bytes fulfills various interfaces in light-web3, etc...
func (hb HexBytes) Bytes() []byte {
	return hb
}

func (hb HexBytes) Compare(o HexBytes) int {
	return Compare(hb, o)
}

func Compare(h1, h2 HexBytes) int {
	return bytes.Compare(h1, h2)
}

func (hb HexBytes) Array20() [20]byte {
	var ret [20]byte
	n := len(ret)
	if len(hb) < n {
		n = len(hb)
	}
	copy(ret[:], hb[:n])
	return ret
}

func (hb HexBytes) Array32() [32]byte {
	var ret [32]byte
	n := len(ret)
	if len(hb) < n {
		n = len(hb)
	}
	copy(ret[:], hb[:n])
	return ret
}

func (hb HexBytes) String() string {
	return strings.ToUpper(hex.EncodeToString(hb))
}

// Format writes either address of 0th element in a slice in base 16 notation,
// with leading 0x (%p), or casts HexBytes to bytes and writes as hexadecimal
// string to s.
func (hb HexBytes) Format(s fmt.State, verb rune) {
	switch verb {
	case 'p':
		s.Write([]byte(fmt.Sprintf("%p", hb)))
	default:
		s.Write([]byte(fmt.Sprintf("%X", []byte(hb))))
	}
}
