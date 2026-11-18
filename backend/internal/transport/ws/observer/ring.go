package observer

import (
	"errors"
	"math"
	"sync"
)

var (
	ErrBufferEmpty  = errors.New("ring buffer is empty")
	ErrInvalidRange = errors.New("requested range is invalid")
	ErrHistoryLost  = errors.New("requested history is lost")
)

type RingBufferItem[T any] struct {
	Value T
	Seq   uint64
}

type RingBuffer[T any] struct {
	buffer []RingBufferItem[T]
	size   uint64
	minSeq uint64
	maxSeq uint64
	mu     sync.RWMutex
}

func NewRingBuffer[T any](size uint64) *RingBuffer[T] {
	if size == 0 {
		panic("ring buffer size cannot be zero")
	}
	return &RingBuffer[T]{
		buffer: make([]RingBufferItem[T], size),
		size:   size,
	}
}

func (b *RingBuffer[T]) index(seq uint64) int {
	idx := (seq - 1) % b.size
	if idx > uint64(math.MaxInt) {
		return math.MaxInt
	}
	return int(idx)
}

// Push requires strictly increasing sequence numbers.
func (b *RingBuffer[T]) Push(item T, seq uint64) error {
	if seq == 0 {
		return errors.New("sequence number cannot be zero")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.maxSeq == 0 {
		b.minSeq = seq
		b.maxSeq = seq
		b.buffer[b.index(seq)] = RingBufferItem[T]{Value: item, Seq: seq}
		return nil
	}

	if seq <= b.maxSeq {
		return errors.New("event sequence is out of order")
	}

	b.maxSeq = seq

	if b.maxSeq-b.minSeq >= b.size {
		b.minSeq = b.maxSeq - b.size + 1
	}

	b.buffer[b.index(seq)] = RingBufferItem[T]{Value: item, Seq: seq}
	return nil
}

// Ranges exclude since and include to; zero to means maxSeq. Evicted starts return ErrHistoryLost.
func (b *RingBuffer[T]) GetRange(since, to uint64) ([]T, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.maxSeq == 0 {
		return nil, ErrBufferEmpty
	}

	if since >= b.maxSeq {
		return nil, nil
	}

	if to != 0 && since >= to {
		return nil, ErrInvalidRange
	}

	start := since + 1

	if start < b.minSeq {
		return nil, ErrHistoryLost
	}

	end := b.maxSeq
	if to != 0 && to < end {
		end = to
	}

	if end < start {
		return nil, nil
	}

	cap := min(end-start+1, b.size)

	result := make([]T, 0, cap)

	for s := start; s <= end; s++ {
		it := b.buffer[b.index(s)]
		if it.Seq == s {
			result = append(result, it.Value)
		}
	}

	return result, nil
}

func (b *RingBuffer[T]) MinSeq() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.minSeq
}

func (b *RingBuffer[T]) MaxSeq() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.maxSeq
}
