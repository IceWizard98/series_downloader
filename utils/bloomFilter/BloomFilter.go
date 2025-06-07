package bloomfilter

import (
	"github.com/cespare/xxhash/v2"
)

var instance *bloomFilter
type bloomFilter struct{
	k      uint32
	Filter uint32
}

func GetInstance() *bloomFilter {
	if instance == nil {
		instance = &bloomFilter{
			k:      3,
			Filter: 0,
		}
	}

	return instance
}

func (b *bloomFilter) getHashes(value []byte) []uint32 {
	hashes := make([]uint32, b.k)
	hash   := xxhash.Sum64(value)
	h1     := uint32(hash & 0xFFFFFFFF)
	h2     := uint32((hash >> 32) & 0xFFFFFFFF)

	for i := range b.k {
		generated := h1 + h2*i
		hashes[i]  = uint32(generated % 32)
	}

	return hashes
}

func (b *bloomFilter) Add(value []byte) {
	hashes := b.getHashes(value)
	for _, hash := range hashes {
		b.Filter |= 1 << hash
	}
}

func (b *bloomFilter) Contains(value []byte) bool {
	hashes := b.getHashes(value)
	for _, hash := range hashes {
		if (b.Filter & (1 << hash)) == 0 {
			return false
		}
	}
	return true
}
