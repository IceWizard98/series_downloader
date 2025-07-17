package bloomfilter

import (
	"math"
	"sync"

	"github.com/cespare/xxhash/v2"
)

var (
	instance *bloomFilter
	once     sync.Once
)

type bloomFilter struct {
	hashingFunctions uint8
	Filter           []uint64
	slots            uint32
	mutex            sync.RWMutex
}

func GetInstance(size uint32) *bloomFilter {
	once.Do(func() {
		falsePositiveRate    := 0.05
		optimalBits          := math.Round(-float64(size) * math.Log(falsePositiveRate) / math.Pow(math.Log(2), 2))
		slots                := uint32(math.Round(optimalBits / 64.0))
		optimalHashFunctions := uint8(math.Round(optimalBits / float64(size) * math.Log(2)))
		
		if optimalHashFunctions == 0 {
			optimalHashFunctions = 1
		}
		
		instance = &bloomFilter{
			hashingFunctions : optimalHashFunctions,
			slots            : slots,
			Filter           : make([]uint64, slots),
		}
	})
	return instance
}

func (b *bloomFilter) getHashes(value []byte) []uint32 {
	hashes := make([]uint32, b.hashingFunctions)
	hash   := xxhash.Sum64(value)
	h1     := uint32(hash & 0xFFFFFFFF)
	h2     := uint32((hash >> 32) & 0xFFFFFFFF)
	
	for i := uint8(0); i < b.hashingFunctions; i++ {
		generated := h1 + h2*uint32(i)
		hashes[i] = generated % 64
	}
	return hashes
}

func (b *bloomFilter) Add(value []byte) {
	hashes := b.getHashes(value)
	
	b.mutex.Lock()
	defer b.mutex.Unlock()
	
	for _, hash := range hashes {
		slot := hash % b.slots
		b.Filter[slot] |= 1 << hash
	}
}

func (b *bloomFilter) Contains(value []byte) bool {
	hashes := b.getHashes(value)
	
	b.mutex.RLock() 
	defer b.mutex.RUnlock()
	
	for _, hash := range hashes {
		slot := hash % b.slots
		if (b.Filter[slot] & (1 << hash)) == 0 {
			return false
		}
	}
	return true
}
