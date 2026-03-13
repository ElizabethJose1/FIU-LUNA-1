package main

import (
	"encoding/binary"
	"hash/crc32"
)

var MaxPacketSize = 8192

func ComputeCRC(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

func AppendCRC(data []byte) []byte {
	c := ComputeCRC(data)
	out := make([]byte, len(data)+4)
	copy(out, data)
	binary.BigEndian.PutUint32(out[len(data):], c)
	return out
}
