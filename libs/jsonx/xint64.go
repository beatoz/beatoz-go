package jsonx

import (
	jsoniter "github.com/json-iterator/go"
	"reflect"
	"strconv"
	"strings"
	"unsafe"
)

type xint64Extension struct {
	jsoniter.DummyExtension
}

func (e *xint64Extension) UpdateStructDescriptor(desc *jsoniter.StructDescriptor) {
	for i := range desc.Fields {
		binding := desc.Fields[i]
		if binding.Field.Type().Kind() == reflect.Int64 {
			jsonTags := binding.Field.Tag().Get("json")
			if jsonTags == "-" {
				continue
			}

			found := false
			if jsonTags != "" {
				parts := strings.Split(jsonTags, ",")
				for _, part := range parts {
					if part == "string" {
						found = true
						break
					}
				}
			}

			if !found {
				binding.Encoder = &int64Encoder{}
				binding.Decoder = &int64Decoder{}
			}
		} else if binding.Field.Type().Kind() == reflect.Uint64 {
			jsonTags := binding.Field.Tag().Get("json")
			if jsonTags == "-" {
				continue
			}

			found := false
			if jsonTags != "" {
				parts := strings.Split(jsonTags, ",")
				for _, part := range parts {
					if part == "string" {
						found = true
						break
					}
				}
			}

			if !found {
				binding.Encoder = &uint64Encoder{}
				binding.Decoder = &uint64Decoder{}
			}
		}
	}
}

type int64Encoder struct{}

func (enc *int64Encoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	encodeInt64(ptr, stream)
}
func (enc *int64Encoder) IsEmpty(ptr unsafe.Pointer) bool {
	return isEmptyInt64(ptr)
}

var _ jsoniter.ValEncoder = (*int64Encoder)(nil)

type int64Decoder struct{}

var _ jsoniter.ValDecoder = (*int64Decoder)(nil)

func (dec *int64Decoder) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	decodeInt64(ptr, iter)
}

//
// int64 operation functions

func encodeInt64(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	v := *(*int64)(ptr)
	stream.WriteString(strconv.FormatInt(v, 10))
}
func decodeInt64(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	switch iter.WhatIsNext() {
	case jsoniter.StringValue:
		s := iter.ReadString()
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			*(*int64)(ptr) = i
		} else {
			*(*int64)(ptr) = 0
		}
	case jsoniter.NumberValue:
		*(*int64)(ptr) = iter.ReadInt64()
	default:
		// 다른 타입이면 건너뛰기
		iter.Skip()
	}
}

func isEmptyInt64(ptr unsafe.Pointer) bool {
	return *(*int64)(ptr) == 0
}

// uint64
type uint64Encoder struct{}

func (enc *uint64Encoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	encodeUint64(ptr, stream)
}
func (enc *uint64Encoder) IsEmpty(ptr unsafe.Pointer) bool {
	return isEmptyInt64(ptr)
}

var _ jsoniter.ValEncoder = (*uint64Encoder)(nil)

type uint64Decoder struct{}

var _ jsoniter.ValDecoder = (*uint64Decoder)(nil)

func (dec *uint64Decoder) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	decodeUint64(ptr, iter)
}

// uint64 operation functions
func encodeUint64(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	v := *(*uint64)(ptr)
	stream.WriteString(strconv.FormatUint(v, 10))
}
func decodeUint64(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	switch iter.WhatIsNext() {
	case jsoniter.StringValue:
		s := iter.ReadString()
		if i, err := strconv.ParseUint(s, 10, 64); err == nil {
			*(*uint64)(ptr) = i
		} else {
			*(*uint64)(ptr) = 0
		}
	case jsoniter.NumberValue:
		*(*uint64)(ptr) = iter.ReadUint64()
	default:
		// 다른 타입이면 건너뛰기
		iter.Skip()
	}
}

func isEmptyUint64(ptr unsafe.Pointer) bool {
	return *(*uint64)(ptr) == 0
}
