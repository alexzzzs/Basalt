package basalt

import (
	"encoding/binary"
	"math"
	"testing"
)

// Helper to create a test chunk
func createTestChunk() *Chunk {
	// x: 1.0, 2.0, 3.0
	// y: 10, 20, 30
	// z: true, false, true
	// s: "apple", "banana", "cherry"
	chunk := &Chunk{
		Len: 3,
		Columns: []Column{
			{Name: "x", Type: Float64, Data: make([]byte, 24)},
			{Name: "y", Type: Int64, Data: make([]byte, 24)},
			{Name: "z", Type: Bool, Data: make([]byte, 3)},
			{Name: "s", Type: String, Strings: []string{"apple", "banana", "cherry"}},
		},
	}

	// Pack data
	binary.LittleEndian.PutUint64(chunk.Columns[0].Data[0:], math.Float64bits(1.0))
	binary.LittleEndian.PutUint64(chunk.Columns[0].Data[8:], math.Float64bits(2.0))
	binary.LittleEndian.PutUint64(chunk.Columns[0].Data[16:], math.Float64bits(3.0))

	binary.LittleEndian.PutUint64(chunk.Columns[1].Data[0:], uint64(10))
	binary.LittleEndian.PutUint64(chunk.Columns[1].Data[8:], uint64(20))
	binary.LittleEndian.PutUint64(chunk.Columns[1].Data[16:], uint64(30))

	chunk.Columns[2].Data[0] = 1
	chunk.Columns[2].Data[1] = 0
	chunk.Columns[2].Data[2] = 1

	return chunk
}

func TestGreater(t *testing.T) {
	chunk := createTestChunk()

	pred := Greater{Column: "x", Value: 1.5}
	mask, err := pred.Evaluate(chunk)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := Mask{false, true, true}
	if len(mask) != len(expected) {
		t.Fatalf("Expected mask length %d, got %d", len(expected), len(mask))
	}
	for i, exp := range expected {
		if mask[i] != exp {
			t.Errorf("Mask[%d] expected %v, got %v", i, exp, mask[i])
		}
	}
}

func TestLess(t *testing.T) {
	chunk := createTestChunk()

	pred := Less{Column: "y", Value: int64(25)}
	mask, err := pred.Evaluate(chunk)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := Mask{true, true, false}
	if len(mask) != len(expected) {
		t.Fatalf("Expected mask length %d, got %d", len(expected), len(mask))
	}
	for i, exp := range expected {
		if mask[i] != exp {
			t.Errorf("Mask[%d] expected %v, got %v", i, exp, mask[i])
		}
	}
}

func TestAnd(t *testing.T) {
	chunk := createTestChunk()

	left := Greater{Column: "x", Value: 1.5}
	right := Less{Column: "y", Value: int64(25)}
	pred := And{Left: left, Right: right}

	mask, err := pred.Evaluate(chunk)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := Mask{false, true, false}
	if len(mask) != len(expected) {
		t.Fatalf("Expected mask length %d, got %d", len(expected), len(mask))
	}
	for i, exp := range expected {
		if mask[i] != exp {
			t.Errorf("Mask[%d] expected %v, got %v", i, exp, mask[i])
		}
	}
}

func TestOr(t *testing.T) {
	chunk := createTestChunk()

	left := Greater{Column: "x", Value: 2.5}
	right := Less{Column: "y", Value: int64(15)}
	pred := Or{Left: left, Right: right}

	mask, err := pred.Evaluate(chunk)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := Mask{true, false, true}
	if len(mask) != len(expected) {
		t.Fatalf("Expected mask length %d, got %d", len(expected), len(mask))
	}
	for i, exp := range expected {
		if mask[i] != exp {
			t.Errorf("Mask[%d] expected %v, got %v", i, exp, mask[i])
		}
	}
}

func TestEquals(t *testing.T) {
	chunk := createTestChunk()

	pred := Equals{Column: "x", Value: 2.0}
	mask, err := pred.Evaluate(chunk)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := Mask{false, true, false}
	if len(mask) != len(expected) {
		t.Fatalf("Expected mask length %d, got %d", len(expected), len(mask))
	}
	for i, exp := range expected {
		if mask[i] != exp {
			t.Errorf("Mask[%d] expected %v, got %v", i, exp, mask[i])
		}
	}
}

func TestEqualsBool(t *testing.T) {
	chunk := createTestChunk()

	pred := Equals{Column: "z", Value: true}
	mask, err := pred.Evaluate(chunk)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := Mask{true, false, true}
	if len(mask) != len(expected) {
		t.Fatalf("Expected mask length %d, got %d", len(expected), len(mask))
	}
	for i, exp := range expected {
		if mask[i] != exp {
			t.Errorf("Mask[%d] expected %v, got %v", i, exp, mask[i])
		}
	}
}

func TestNot(t *testing.T) {
	chunk := createTestChunk()

	inner := Greater{Column: "x", Value: 1.5}
	pred := Not{Predicate: inner}

	mask, err := pred.Evaluate(chunk)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := Mask{true, false, false} // Negation of {false, true, true}
	if len(mask) != len(expected) {
		t.Fatalf("Expected mask length %d, got %d", len(expected), len(mask))
	}
	for i, exp := range expected {
		if mask[i] != exp {
			t.Errorf("Mask[%d] expected %v, got %v", i, exp, mask[i])
		}
	}
}

func TestGreaterString(t *testing.T) {
	chunk := createTestChunk()

	pred := Greater{Column: "s", Value: "banana"}
	mask, err := pred.Evaluate(chunk)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := Mask{false, false, true} // "cherry" > "banana", others are not
	if len(mask) != len(expected) {
		t.Fatalf("Expected mask length %d, got %d", len(expected), len(mask))
	}
	for i, exp := range expected {
		if mask[i] != exp {
			t.Errorf("Mask[%d] expected %v, got %v", i, exp, mask[i])
		}
	}
}

func TestLessString(t *testing.T) {
	chunk := createTestChunk()

	pred := Less{Column: "s", Value: "banana"}
	mask, err := pred.Evaluate(chunk)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := Mask{true, false, false} // "apple" < "banana", others are not
	if len(mask) != len(expected) {
		t.Fatalf("Expected mask length %d, got %d", len(expected), len(mask))
	}
	for i, exp := range expected {
		if mask[i] != exp {
			t.Errorf("Mask[%d] expected %v, got %v", i, exp, mask[i])
		}
	}
}

func TestEqualsString(t *testing.T) {
	chunk := createTestChunk()

	pred := Equals{Column: "s", Value: "banana"}
	mask, err := pred.Evaluate(chunk)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := Mask{false, true, false}
	if len(mask) != len(expected) {
		t.Fatalf("Expected mask length %d, got %d", len(expected), len(mask))
	}
	for i, exp := range expected {
		if mask[i] != exp {
			t.Errorf("Mask[%d] expected %v, got %v", i, exp, mask[i])
		}
	}
}

// Test predicate validation
func TestPredicateValidation(t *testing.T) {
	chunk := createTestChunk()

	// Valid predicate - float64 column with float64 value
	pred := Greater{Column: "x", Value: 1.5}
	_, err := pred.Evaluate(chunk)
	if err != nil {
		t.Errorf("Expected valid predicate, got error: %v", err)
	}

	// Invalid predicate - float64 column with string value
	badPred := Greater{Column: "x", Value: "not a number"}
	_, err = badPred.Evaluate(chunk)
	if err == nil {
		t.Errorf("Expected error for type mismatch in predicate")
	}

	// Non-existent column
	badPred2 := Greater{Column: "nonexistent", Value: 1.5}
	_, err = badPred2.Evaluate(chunk)
	if err == nil {
		t.Errorf("Expected error for non-existent column")
	}
}

func TestValidatePredicateValue(t *testing.T) {
	// Valid validations
	if err := ValidatePredicateValue(Float64, 1.5); err != nil {
		t.Errorf("Expected valid float64 validation, got error: %v", err)
	}
	if err := ValidatePredicateValue(Int64, int64(10)); err != nil {
		t.Errorf("Expected valid int64 validation, got error: %v", err)
	}
	if err := ValidatePredicateValue(Bool, true); err != nil {
		t.Errorf("Expected valid bool validation, got error: %v", err)
	}
	if err := ValidatePredicateValue(String, "test"); err != nil {
		t.Errorf("Expected valid string validation, got error: %v", err)
	}

	// Invalid validations
	if err := ValidatePredicateValue(Float64, "not a float"); err == nil {
		t.Errorf("Expected error for float64 type mismatch")
	}
	if err := ValidatePredicateValue(Int64, 10.5); err == nil {
		t.Errorf("Expected error for int64 type mismatch")
	}
	if err := ValidatePredicateValue(Bool, "not a bool"); err == nil {
		t.Errorf("Expected error for bool type mismatch")
	}
	if err := ValidatePredicateValue(String, 123); err == nil {
		t.Errorf("Expected error for string type mismatch")
	}
}
