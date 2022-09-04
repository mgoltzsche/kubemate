package passwordgen

import (
	"crypto/rand"
	"encoding/binary"
)

type cryptoRandSource struct{}

func (_ cryptoRandSource) Int63() int64 {
	var b [8]byte
	rand.Read(b[:])
	// mask off sign bit to ensure positive number
	return int64(binary.LittleEndian.Uint64(b[:]) & (1<<63 - 1))
}

func (_ cryptoRandSource) Seed(_ int64) {}
