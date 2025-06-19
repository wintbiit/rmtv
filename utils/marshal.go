package utils

import (
	"encoding/binary"
)

func MarshalInt(n int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(n))
	return b
}

func UnmarshalInt(b []byte) int {
	if len(b) < 8 {
		return 0
	}
	return int(binary.BigEndian.Uint64(b))
}

func MarshalInt64(n int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(n))
	return b
}

func UnmarshalInt64(b []byte) int64 {
	if len(b) < 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(b))
}
