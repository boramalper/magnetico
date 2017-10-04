package mainline

import (
	"net"
	"go.uber.org/zap"
	"crypto/sha1"
	"math/bits"
	"math"
)

const (
	k uint32 = 2
	m uint32 = 256 * 8
)

type BloomFilter struct {
	filter [m/8]byte
}

func (bf *BloomFilter) InsertIP(ip net.IP) {
	if !(len(ip) == net.IPv4len || len(ip) == net.IPv6len) {
		zap.S().Panicf("Attempted to insert an invalid IP to the bloom filter!  %d", len(ip))
	}

	hash := sha1.Sum(ip)

	var index1, index2 uint32
	index1 = uint32(hash[0]) | uint32(hash[1]) << 8
	index2 = uint32(hash[2]) | uint32(hash[3]) << 8

	// truncate index to m (11 bits required)
	index1 %= m
	index2 %= m

	// set bits at index1 and index2
	bf.filter[index1 / 8] |= 0x01 << (index1 % 8)
	bf.filter[index2 / 8] |= 0x01 << (index2 % 8)
}

func (bf *BloomFilter) Estimate() float64 {
	// TODO: make it faster?
	var nZeroes uint32 = 0
	for _, b := range bf.filter {
		nZeroes += 8 - uint32(bits.OnesCount8(uint8(b)))
	}

	var c uint32
	if m - 1 < nZeroes {
		c = m - 1
	} else {
		c = nZeroes
	}
	return math.Log(float64(c) / float64(m)) / (float64(k) * math.Log(1 - 1/float64(m)))
}

func (bf *BloomFilter) Filter() (filterCopy [m/8]byte) {
	copy(filterCopy[:], bf.filter[:])
	return filterCopy
}
