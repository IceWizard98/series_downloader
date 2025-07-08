package bloomfilter

import (
	"sync"
	"github.com/cespare/xxhash/v2"
)

var (
	instance *bloomFilter
	once     sync.Once
)

type bloomFilter struct {
	k      uint32
	Filter uint32
	mutex  sync.RWMutex
}

func GetInstance() *bloomFilter {
	once.Do(func() {
		instance = &bloomFilter{
			k:      3,
			Filter: 0,
		}
	})
	return instance
}

func (b *bloomFilter) getHashes(value []byte) []uint32 {
	hashes := make([]uint32, b.k)
	hash   := xxhash.Sum64(value)
	h1     := uint32(hash & 0xFFFFFFFF)
	h2     := uint32((hash >> 32) & 0xFFFFFFFF)
	
	for i := uint32(0); i < b.k; i++ {
		generated := h1 + h2*i
		hashes[i] = generated % 32
	}
	return hashes
}

func (b *bloomFilter) Add(value []byte) {
	hashes := b.getHashes(value)
	
	b.mutex.Lock()
	defer b.mutex.Unlock()
	
	for _, hash := range hashes {
		b.Filter |= 1 << hash
	}
}

func (b *bloomFilter) Contains(value []byte) bool {
	hashes := b.getHashes(value)
	
	b.mutex.RLock() 
	defer b.mutex.RUnlock()
	
	for _, hash := range hashes {
		if (b.Filter & (1 << hash)) == 0 {
			return false
		}
	}
	return true
}
