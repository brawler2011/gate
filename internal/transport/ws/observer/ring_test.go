package observer_test

import (
	"reflect"
	"testing"

	"github.com/gate149/core/internal/transport/ws/observer"
)

func TestRingBuffer_Push(t *testing.T) {
	rb := observer.NewRingBuffer[int](3)

	// Test: Successful push
	if err := rb.Push(10, 1); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test: Zero sequence
	if err := rb.Push(20, 0); err == nil {
		t.Error("expected error for sequence 0, got nil")
	}

	// Test: Monotonicity violation (duplicate sequence)
	if err := rb.Push(20, 1); err == nil {
		t.Error("expected error for duplicate sequence, got nil")
	}

	if rb.MaxSeq() != 1 {
		t.Errorf("expected MaxSeq 1, got %d", rb.MaxSeq())
	}
}

func TestRingBuffer_GetRange(t *testing.T) {
	rb := observer.NewRingBuffer[string](3)
	rb.Push("one", 1)
	rb.Push("two", 2)
	rb.Push("three", 3)

	tests := []struct {
		name    string
		since   uint64
		to      uint64
		want    []string
		wantErr bool
	}{
		{"FullRange", 0, 3, []string{"one", "two", "three"}, false},
		{"PartialRange", 1, 3, []string{"two", "three"}, false},
		{"OpenEnd", 1, 0, []string{"two", "three"}, false},
		{"SingleItem", 1, 2, []string{"two"}, false},
		{"InvalidRange", 3, 1, nil, true}, // since >= to
		{"BeyondMax", 5, 0, nil, false},   // since >= maxSeq
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rb.GetRange(tt.since, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) && !(len(got) == 0 && len(tt.want) == 0) {
				t.Errorf("GetRange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRingBuffer_HistoryLost(t *testing.T) {
	// Buffer with size 2
	rb := observer.NewRingBuffer[int](2)
	rb.Push(1, 1)
	rb.Push(2, 2)
	rb.Push(3, 3) // Element with seq=1 should be overwritten

	if rb.MinSeq() != 2 {
		t.Errorf("expected MinSeq 2, got %d", rb.MinSeq())
	}

	// Try to get overwritten element
	_, err := rb.GetRange(0, 3)
	if err == nil {
		t.Error("expected history lost error, got nil")
	}

	// Get remaining elements
	got, err := rb.GetRange(1, 3)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	want := []int{2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRingBuffer_Empty(t *testing.T) {
	rb := observer.NewRingBuffer[int](5)
	_, err := rb.GetRange(0, 1)
	if err == nil {
		t.Error("expected error for empty buffer")
	}
}
