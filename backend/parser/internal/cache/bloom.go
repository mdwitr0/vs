package cache

import (
	"sync"

	"github.com/bits-and-blooms/bloom/v3"
)

type URLBloomFilter struct {
	filter *bloom.BloomFilter
	mu     sync.RWMutex
}

// NewURLBloomFilter creates a new Bloom filter for URLs
// expectedItems: expected number of URLs (e.g., 1_000_000)
// fpRate: false positive rate (e.g., 0.001 = 0.1%)
func NewURLBloomFilter(expectedItems uint, fpRate float64) *URLBloomFilter {
	return &URLBloomFilter{
		filter: bloom.NewWithEstimates(expectedItems, fpRate),
	}
}

func (b *URLBloomFilter) Add(url string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.filter.AddString(url)
}

func (b *URLBloomFilter) MayContain(url string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.filter.TestString(url)
}

func (b *URLBloomFilter) LoadBatch(urls []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, url := range urls {
		b.filter.AddString(url)
	}
}

func (b *URLBloomFilter) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.filter.ClearAll()
}

func (b *URLBloomFilter) Count() uint32 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.filter.ApproximatedSize()
}
