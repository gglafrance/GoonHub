package fingerprint

import (
	"encoding/gob"
	"fmt"
	"hash"
	"hash/fnv"
	"math"
	"os"
	"sync"
)

// BloomFilter is a thread-safe bloom filter with disk persistence.
type BloomFilter struct {
	bits    []uint64
	size    uint64
	hashFns int
	mu      sync.RWMutex
}

// NewBloomFilter creates a bloom filter sized for the expected number of items
// and desired false positive rate.
func NewBloomFilter(expectedItems int, falsePositiveRate float64) *BloomFilter {
	if expectedItems <= 0 {
		expectedItems = 1000
	}
	if falsePositiveRate <= 0 || falsePositiveRate >= 1 {
		falsePositiveRate = 0.01
	}

	// Optimal size: m = -n*ln(p) / (ln2)^2
	n := float64(expectedItems)
	p := falsePositiveRate
	m := -n * math.Log(p) / (math.Ln2 * math.Ln2)
	size := uint64(math.Ceil(m))

	// Optimal hash functions: k = (m/n) * ln2
	k := int(math.Ceil((float64(size) / n) * math.Ln2))
	if k < 1 {
		k = 1
	}

	words := (size + 63) / 64
	return &BloomFilter{
		bits:    make([]uint64, words),
		size:    size,
		hashFns: k,
	}
}

func (bf *BloomFilter) getHashes(value uint64) []uint64 {
	positions := make([]uint64, bf.hashFns)

	var h hash.Hash64
	h = fnv.New64a()

	// Double hashing: h(i) = h1 + i*h2
	buf := make([]byte, 8)
	buf[0] = byte(value)
	buf[1] = byte(value >> 8)
	buf[2] = byte(value >> 16)
	buf[3] = byte(value >> 24)
	buf[4] = byte(value >> 32)
	buf[5] = byte(value >> 40)
	buf[6] = byte(value >> 48)
	buf[7] = byte(value >> 56)

	h.Reset()
	h.Write(buf)
	h1 := h.Sum64()

	// Use reversed byte order for an independent second hash
	h.Reset()
	for i := 7; i >= 0; i-- {
		h.Write(buf[i : i+1])
	}
	h2 := h.Sum64()

	for i := 0; i < bf.hashFns; i++ {
		positions[i] = (h1 + uint64(i)*h2) % bf.size
	}

	return positions
}

// Add adds a hash value to the bloom filter.
func (bf *BloomFilter) Add(value uint64) {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	for _, pos := range bf.getHashes(value) {
		word := pos / 64
		bit := pos % 64
		bf.bits[word] |= 1 << bit
	}
}

// MayContain returns true if the value might be in the set (false positives possible).
func (bf *BloomFilter) MayContain(value uint64) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	for _, pos := range bf.getHashes(value) {
		word := pos / 64
		bit := pos % 64
		if bf.bits[word]&(1<<bit) == 0 {
			return false
		}
	}
	return true
}

// bloomFilterData is the serialization format for the bloom filter.
type bloomFilterData struct {
	Bits    []uint64
	Size    uint64
	HashFns int
}

// SaveToFile persists the bloom filter to disk using gob encoding.
func (bf *BloomFilter) SaveToFile(path string) error {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create bloom filter file: %w", err)
	}
	defer f.Close()

	data := bloomFilterData{
		Bits:    bf.bits,
		Size:    bf.size,
		HashFns: bf.hashFns,
	}

	enc := gob.NewEncoder(f)
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("failed to encode bloom filter: %w", err)
	}

	return nil
}

// LoadFromFile loads a bloom filter from disk.
func LoadFromFile(path string) (*BloomFilter, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open bloom filter file: %w", err)
	}
	defer f.Close()

	var data bloomFilterData
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode bloom filter: %w", err)
	}

	return &BloomFilter{
		bits:    data.Bits,
		size:    data.Size,
		hashFns: data.HashFns,
	}, nil
}
