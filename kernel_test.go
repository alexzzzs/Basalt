package basalt

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestMultiply(t *testing.T) {
	chunk := createTestChunk()

	kernel := Multiply{Column: "x", Scalar: 2.0}
	resultCols, err := kernel.Execute(chunk, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(resultCols) != 1 {
		t.Fatalf("Expected 1 result column, got %d", len(resultCols))
	}

	col := resultCols[0]
	if col.Type != Float64 {
		t.Errorf("Expected Float64 type, got %v", col.Type)
	}

	// Check values: 1.0*2=2.0, 2.0*2=4.0, 3.0*2=6.0
	expected := []float64{2.0, 4.0, 6.0}
	for i, exp := range expected {
		offset := i * 8
		bits := binary.LittleEndian.Uint64(col.Data[offset:])
		val := math.Float64frombits(bits)
		if val != exp {
			t.Errorf("Value[%d] expected %v, got %v", i, exp, val)
		}
	}
}

func TestMultiplyWithMask(t *testing.T) {
	chunk := createTestChunk()
	// Mask: true, false, true
	mask := Mask{true, false, true}

	kernel := Multiply{Column: "x", Scalar: 2.0}
	resultCols, err := kernel.Execute(chunk, mask)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	col := resultCols[0]

	// Expected: 1.0*2=2.0, 2.0 (no change), 3.0*2=6.0
	expected := []float64{2.0, 2.0, 6.0}
	for i, exp := range expected {
		offset := i * 8
		bits := binary.LittleEndian.Uint64(col.Data[offset:])
		val := math.Float64frombits(bits)
		if val != exp {
			t.Errorf("Value[%d] expected %v, got %v", i, exp, val)
		}
	}
}

func TestSum(t *testing.T) {
	chunk := createTestChunk()

	kernel := Sum{Column: "x"}
	resultCols, err := kernel.Execute(chunk, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(resultCols) != 1 {
		t.Fatalf("Expected 1 result column, got %d", len(resultCols))
	}

	col := resultCols[0]
	if len(col.Data) != 8 {
		t.Errorf("Expected 8 bytes, got %d", len(col.Data))
	}

	bits := binary.LittleEndian.Uint64(col.Data)
	total := math.Float64frombits(bits)
	expected := 1.0 + 2.0 + 3.0 // 6.0
	if total != expected {
		t.Errorf("Expected sum %v, got %v", expected, total)
	}
}

func TestSumWithMask(t *testing.T) {
	chunk := createTestChunk()
	// Mask: true, false, true
	mask := Mask{true, false, true}

	kernel := Sum{Column: "x"}
	resultCols, err := kernel.Execute(chunk, mask)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	col := resultCols[0]
	bits := binary.LittleEndian.Uint64(col.Data)
	total := math.Float64frombits(bits)
	expected := 1.0 + 3.0 // 4.0
	if total != expected {
		t.Errorf("Expected sum %v, got %v", expected, total)
	}
}

func TestCount(t *testing.T) {
	chunk := createTestChunk()

	kernel := Count{}
	resultCols, err := kernel.Execute(chunk, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(resultCols) != 1 {
		t.Fatalf("Expected 1 result column, got %d", len(resultCols))
	}

	col := resultCols[0]
	if col.Type != Int64 {
		t.Errorf("Expected Int64 type, got %v", col.Type)
	}

	bits := binary.LittleEndian.Uint64(col.Data)
	count := int64(bits)
	expected := int64(3)
	if count != expected {
		t.Errorf("Expected count %v, got %v", expected, count)
	}
}

func TestCountWithMask(t *testing.T) {
	chunk := createTestChunk()
	// Mask: true, false, true
	mask := Mask{true, false, true}

	kernel := Count{}
	resultCols, err := kernel.Execute(chunk, mask)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	col := resultCols[0]
	bits := binary.LittleEndian.Uint64(col.Data)
	count := int64(bits)
	expected := int64(2) // Only masked rows
	if count != expected {
		t.Errorf("Expected count %v, got %v", expected, count)
	}
}

func TestCountColumn(t *testing.T) {
	chunk := createTestChunk()

	kernel := Count{Column: "s"} // Count string column
	resultCols, err := kernel.Execute(chunk, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	col := resultCols[0]
	bits := binary.LittleEndian.Uint64(col.Data)
	count := int64(bits)
	expected := int64(3) // All strings are non-empty
	if count != expected {
		t.Errorf("Expected count %v, got %v", expected, count)
	}
}

// Test kernel validation
func TestKernelValidation(t *testing.T) {
	chunk := createTestChunk()

	// Valid kernel - float64 column with float64 scalar
	kernel := Multiply{Column: "x", Scalar: 2.0}
	_, err := kernel.Execute(chunk, nil)
	if err != nil {
		t.Errorf("Expected valid kernel execution, got error: %v", err)
	}

	// Invalid kernel - float64 column with int64 scalar (should work for multiply)
	kernel2 := Multiply{Column: "x", Scalar: int64(2)}
	_, err = kernel2.Execute(chunk, nil)
	if err != nil {
		t.Errorf("Expected valid kernel execution with int64 scalar, got error: %v", err)
	}

	// Invalid kernel - wrong scalar type for column
	kernel3 := Multiply{Column: "x", Scalar: "not a number"}
	_, err = kernel3.Execute(chunk, nil)
	if err == nil {
		t.Errorf("Expected error for incompatible scalar type")
	}

	// Non-existent column
	kernel4 := Multiply{Column: "nonexistent", Scalar: 2.0}
	_, err = kernel4.Execute(chunk, nil)
	if err == nil {
		t.Errorf("Expected error for non-existent column")
	}

	// Division by zero
	kernel5 := Divide{Column: "x", Scalar: 0.0}
	_, err = kernel5.Execute(chunk, nil)
	if err == nil {
		t.Errorf("Expected error for division by zero")
	}
}

func TestVariance(t *testing.T) {
	chunk := createTestChunk()

	kernel := Variance{Column: "x"}
	resultCols, err := kernel.Execute(chunk, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(resultCols) != 1 {
		t.Fatalf("Expected 1 result column, got %d", len(resultCols))
	}

	col := resultCols[0]
	if len(col.Data) != 8 {
		t.Errorf("Expected 8 bytes, got %d", len(col.Data))
	}

	bits := binary.LittleEndian.Uint64(col.Data)
	variance := math.Float64frombits(bits)
	// For values [1.0, 2.0, 3.0], mean = 2.0, variance = ((1-2)^2 + (2-2)^2 + (3-2)^2) / 3 = (1 + 0 + 1) / 3 = 0.666...
	expected := 2.0 / 3.0 // approximately 0.666...
	if math.Abs(variance-expected) > 0.001 {
		t.Errorf("Expected variance %v, got %v", expected, variance)
	}
}

func TestStdDev(t *testing.T) {
	chunk := createTestChunk()

	kernel := StdDev{Column: "x"}
	resultCols, err := kernel.Execute(chunk, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	col := resultCols[0]
	bits := binary.LittleEndian.Uint64(col.Data)
	stddev := math.Float64frombits(bits)
	// StdDev is sqrt of variance, so sqrt(2/3) ≈ 0.816...
	expected := math.Sqrt(2.0 / 3.0)
	if math.Abs(stddev-expected) > 0.001 {
		t.Errorf("Expected stddev %v, got %v", expected, stddev)
	}
}

func TestMedian(t *testing.T) {
	chunk := createTestChunk()

	kernel := Median{Column: "x"}
	resultCols, err := kernel.Execute(chunk, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	col := resultCols[0]
	bits := binary.LittleEndian.Uint64(col.Data)
	median := math.Float64frombits(bits)
	// For [1.0, 2.0, 3.0], sorted median is 2.0
	expected := 2.0
	if median != expected {
		t.Errorf("Expected median %v, got %v", expected, median)
	}
}

func TestValidateKernelScalar(t *testing.T) {
	// Valid validations
	if err := ValidateKernelScalar(1.5, []ColumnType{Float64}); err != nil {
		t.Errorf("Expected valid float64 scalar validation, got error: %v", err)
	}
	if err := ValidateKernelScalar(int64(10), []ColumnType{Int64}); err != nil {
		t.Errorf("Expected valid int64 scalar validation, got error: %v", err)
	}
	if err := ValidateKernelScalar(true, []ColumnType{Bool}); err != nil {
		t.Errorf("Expected valid bool scalar validation, got error: %v", err)
	}
	if err := ValidateKernelScalar("test", []ColumnType{String}); err != nil {
		t.Errorf("Expected valid string scalar validation, got error: %v", err)
	}

	// Multiple allowed types
	if err := ValidateKernelScalar(1.5, []ColumnType{Float64, Int64}); err != nil {
		t.Errorf("Expected valid multi-type validation, got error: %v", err)
	}

	// Invalid validations
	if err := ValidateKernelScalar("not a float", []ColumnType{Float64}); err == nil {
		t.Errorf("Expected error for float64 type mismatch")
	}
	if err := ValidateKernelScalar("not a int", []ColumnType{Int64}); err == nil {
		t.Errorf("Expected error for int64 type mismatch")
	}
	if err := ValidateKernelScalar("not a bool", []ColumnType{Bool}); err == nil {
		t.Errorf("Expected error for bool type mismatch")
	}
	if err := ValidateKernelScalar(123, []ColumnType{String}); err == nil {
		t.Errorf("Expected error for string type mismatch")
	}
}

func TestAddColumns(t *testing.T) {
	chunk := createTestChunk()

	kernel := AddColumns{Left: "x", Right: "y"}
	resultCols, err := kernel.Execute(chunk, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(resultCols) != 1 {
		t.Fatalf("Expected 1 result column, got %d", len(resultCols))
	}

	col := resultCols[0]
	if col.Type != Float64 {
		t.Errorf("Expected Float64 type, got %v", col.Type)
	}

	// Check values: 1.0+10=11.0, 2.0+20=22.0, 3.0+30=33.0
	expected := []float64{11.0, 22.0, 33.0}
	for i, exp := range expected {
		offset := i * 8
		bits := binary.LittleEndian.Uint64(col.Data[offset:])
		val := math.Float64frombits(bits)
		if val != exp {
			t.Errorf("Value[%d] expected %v, got %v", i, exp, val)
		}
	}
}

func TestSubtractColumns(t *testing.T) {
	chunk := createTestChunk()

	kernel := SubtractColumns{Left: "y", Right: "x"}
	resultCols, err := kernel.Execute(chunk, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	col := resultCols[0]
	if col.Type != Float64 {
		t.Errorf("Expected Float64 type, got %v", col.Type)
	}

	// Check values: 10-1=9.0, 20-2=18.0, 30-3=27.0
	expected := []float64{9.0, 18.0, 27.0}
	for i, exp := range expected {
		offset := i * 8
		bits := binary.LittleEndian.Uint64(col.Data[offset:])
		val := math.Float64frombits(bits)
		if val != exp {
			t.Errorf("Value[%d] expected %v, got %v", i, exp, val)
		}
	}
}

func TestMultiplyColumns(t *testing.T) {
	chunk := createTestChunk()

	kernel := MultiplyColumns{Left: "x", Right: "y"}
	resultCols, err := kernel.Execute(chunk, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	col := resultCols[0]
	if col.Type != Float64 {
		t.Errorf("Expected Float64 type, got %v", col.Type)
	}

	// Check values: 1.0*10=10.0, 2.0*20=40.0, 3.0*30=90.0
	expected := []float64{10.0, 40.0, 90.0}
	for i, exp := range expected {
		offset := i * 8
		bits := binary.LittleEndian.Uint64(col.Data[offset:])
		val := math.Float64frombits(bits)
		if val != exp {
			t.Errorf("Value[%d] expected %v, got %v", i, exp, val)
		}
	}
}

func TestDivideColumns(t *testing.T) {
	chunk := createTestChunk()

	kernel := DivideColumns{Left: "y", Right: "x"}
	resultCols, err := kernel.Execute(chunk, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	col := resultCols[0]
	if col.Type != Float64 {
		t.Errorf("Expected Float64 type, got %v", col.Type)
	}

	// Check values: 10/1=10.0, 20/2=10.0, 30/3=10.0
	expected := []float64{10.0, 10.0, 10.0}
	for i, exp := range expected {
		offset := i * 8
		bits := binary.LittleEndian.Uint64(col.Data[offset:])
		val := math.Float64frombits(bits)
		if val != exp {
			t.Errorf("Value[%d] expected %v, got %v", i, exp, val)
		}
	}
}

func TestPowerColumns(t *testing.T) {
	chunk := createTestChunk()

	kernel := PowerColumns{Left: "x", Right: "y"}
	resultCols, err := kernel.Execute(chunk, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	col := resultCols[0]
	if col.Type != Float64 {
		t.Errorf("Expected Float64 type, got %v", col.Type)
	}

	// Check values: 1^10=1.0, 2^20=1.048576e+06, 3^30=huge number
	expected := []float64{1.0, 1048576.0, 205891132094649.0}
	for i, exp := range expected {
		offset := i * 8
		bits := binary.LittleEndian.Uint64(col.Data[offset:])
		val := math.Float64frombits(bits)
		if math.Abs(val-exp) > 0.001 {
			t.Errorf("Value[%d] expected %v, got %v", i, exp, val)
		}
	}
}

func TestAddColumnsInt64(t *testing.T) {
	chunk := createTestChunk()

	kernel := AddColumns{Left: "y", Right: "y"}
	resultCols, err := kernel.Execute(chunk, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	col := resultCols[0]
	if col.Type != Int64 {
		t.Errorf("Expected Int64 type, got %v", col.Type)
	}

	// Check values: 10+10=20, 20+20=40, 30+30=60
	expected := []int64{20, 40, 60}
	for i, exp := range expected {
		offset := i * 8
		bits := binary.LittleEndian.Uint64(col.Data[offset:])
		val := int64(bits)
		if val != exp {
			t.Errorf("Value[%d] expected %v, got %v", i, exp, val)
		}
	}
}

func TestDivideColumnsWithMask(t *testing.T) {
	chunk := createTestChunk()
	// Mask: true, false, true
	mask := Mask{true, false, true}

	kernel := DivideColumns{Left: "y", Right: "x"}
	resultCols, err := kernel.Execute(chunk, mask)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	col := resultCols[0]

	// Expected: 10/1=10.0, 20 (no change), 30/3=10.0
	expected := []float64{10.0, 20.0, 10.0}
	for i, exp := range expected {
		offset := i * 8
		bits := binary.LittleEndian.Uint64(col.Data[offset:])
		val := math.Float64frombits(bits)
		if val != exp {
			t.Errorf("Value[%d] expected %v, got %v", i, exp, val)
		}
	}
}

func TestColumnArithmeticErrors(t *testing.T) {
	chunk := createTestChunk()

	// Non-existent left column
	kernel1 := AddColumns{Left: "nonexistent", Right: "x"}
	_, err := kernel1.Execute(chunk, nil)
	if err == nil {
		t.Errorf("Expected error for non-existent left column")
	}

	// Non-existent right column
	kernel2 := AddColumns{Left: "x", Right: "nonexistent"}
	_, err = kernel2.Execute(chunk, nil)
	if err == nil {
		t.Errorf("Expected error for non-existent right column")
	}

	// Type mismatch
	kernel3 := AddColumns{Left: "x", Right: "s"} // float64 + string
	_, err = kernel3.Execute(chunk, nil)
	if err == nil {
		t.Errorf("Expected error for type mismatch")
	}

	// Division by zero
	kernel4 := DivideColumns{Left: "x", Right: "z"} // dividing by boolean column with zeros
	_, err = kernel4.Execute(chunk, nil)
	if err == nil {
		t.Errorf("Expected error for division by zero")
	}

	// Power operation with Int64 (should now work)
	kernel5 := PowerColumns{Left: "y", Right: "y"} // int64 ^ int64
	_, err = kernel5.Execute(chunk, nil)
	if err != nil {
		t.Errorf("Expected success for power operation with Int64, got error: %v", err)
	}
}
