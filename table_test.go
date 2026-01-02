package basalt

import (
	"testing"
)

func TestNewTable(t *testing.T) {
	schema := []ColumnType{Float64, Int64, Bool}
	columnNames := []string{"x", "y", "z"}
	table, err := NewTable(schema, columnNames)
	if err != nil {
		t.Fatalf("Unexpected error creating table: %v", err)
	}

	if len(table.Schema) != 3 {
		t.Errorf("Expected schema length 3, got %d", len(table.Schema))
	}

	if table.Schema[0] != Float64 {
		t.Errorf("Expected first column Float64, got %v", table.Schema[0])
	}

	if len(table.Chunks) != 0 {
		t.Errorf("Expected 0 chunks initially, got %d", len(table.Chunks))
	}
}

func TestAppendBatch(t *testing.T) {
	table, err := NewTable([]ColumnType{Float64, Int64, Bool}, []string{"x", "y", "z"})
	if err != nil {
		t.Fatalf("Unexpected error creating table: %v", err)
	}

	data := map[string][]interface{}{
		"x": {1.5, 2.5},
		"y": {int64(10), int64(20)},
		"z": {true, false},
	}

	if err := table.AppendBatch(data); err != nil {
		t.Fatalf("Unexpected error appending batch: %v", err)
	}

	if len(table.Chunks) != 1 {
		t.Errorf("Expected 1 chunk, got %d", len(table.Chunks))
	}

	chunk := table.Chunks[0]
	if chunk.Len != 2 {
		t.Errorf("Expected chunk length 2, got %d", chunk.Len)
	}

	if len(chunk.Columns) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(chunk.Columns))
	}

	// Verify column types
	for i, col := range chunk.Columns {
		if col.Type != table.Schema[i] {
			t.Errorf("Expected column type %v, got %v", table.Schema[i], col.Type)
		}
	}
}

func TestWriter(t *testing.T) {
	table, err := NewTable([]ColumnType{Float64, Int64, Bool}, []string{"x", "y", "z"})
	if err != nil {
		t.Fatalf("Unexpected error creating table: %v", err)
	}
	writer := table.NewWriter(2)

	// Append rows
	if err := writer.Append(1.0, int64(100), true); err != nil {
		t.Fatalf("Unexpected error appending row: %v", err)
	}
	if err := writer.Append(2.0, int64(200), false); err != nil {
		t.Fatalf("Unexpected error appending row: %v", err)
	}

	// Should auto-flush since bufSize=2
	if len(table.Chunks) != 1 {
		t.Errorf("Expected 1 chunk after auto-flush, got %d", len(table.Chunks))
	}

	// Append another row
	if err := writer.Append(3.0, int64(300), true); err != nil {
		t.Fatalf("Unexpected error appending row: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("Unexpected error flushing: %v", err)
	}

	if len(table.Chunks) != 2 {
		t.Errorf("Expected 2 chunks after flush, got %d", len(table.Chunks))
	}
}

func TestWriterFlush(t *testing.T) {
	table, err := NewTable([]ColumnType{Float64, Int64, Bool}, []string{"x", "y", "z"})
	if err != nil {
		t.Fatalf("Unexpected error creating table: %v", err)
	}
	writer := table.NewWriter(10)

	if err := writer.Append(1.0, int64(100), true); err != nil {
		t.Fatalf("Unexpected error appending row: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("Unexpected error flushing: %v", err)
	}

	if len(table.Chunks) != 1 {
		t.Errorf("Expected 1 chunk after flush, got %d", len(table.Chunks))
	}

	// Flush empty buffer
	if err := writer.Flush(); err != nil {
		t.Fatalf("Unexpected error flushing empty buffer: %v", err)
	}
	if len(table.Chunks) != 1 {
		t.Errorf("Expected still 1 chunk after empty flush, got %d", len(table.Chunks))
	}
}

// Test validation functions
func TestValidateSchema(t *testing.T) {
	// Valid schema
	schema := []ColumnType{Float64, Int64, Bool}
	names := []string{"x", "y", "z"}
	if err := ValidateSchema(schema, names); err != nil {
		t.Errorf("Expected valid schema, got error: %v", err)
	}

	// Mismatched lengths
	if err := ValidateSchema([]ColumnType{Float64}, []string{"x", "y"}); err == nil {
		t.Errorf("Expected error for mismatched lengths")
	}

	// Duplicate names
	if err := ValidateSchema([]ColumnType{Float64, Int64}, []string{"x", "x"}); err == nil {
		t.Errorf("Expected error for duplicate column names")
	}

	// Empty name
	if err := ValidateSchema([]ColumnType{Float64}, []string{""}); err == nil {
		t.Errorf("Expected error for empty column name")
	}

	// Invalid column type
	if err := ValidateSchema([]ColumnType{10}, []string{"x"}); err == nil {
		t.Errorf("Expected error for invalid column type")
	}
}

func TestNewTableValidation(t *testing.T) {
	// Test invalid schema
	_, err := NewTable([]ColumnType{Float64}, []string{"x", "y"})
	if err == nil {
		t.Errorf("Expected error for mismatched schema lengths")
	}

	_, err = NewTable([]ColumnType{10}, []string{"x"})
	if err == nil {
		t.Errorf("Expected error for invalid column type")
	}
}

func TestAppendBatchValidation(t *testing.T) {
	table, _ := NewTable([]ColumnType{Float64, Int64}, []string{"x", "y"})

	// Valid data
	data := map[string][]interface{}{
		"x": {1.0, 2.0},
		"y": {int64(10), int64(20)},
	}
	if err := table.AppendBatch(data); err != nil {
		t.Errorf("Expected valid batch append, got error: %v", err)
	}

	// Missing column
	badData := map[string][]interface{}{
		"x": {1.0},
		// missing "y"
	}
	if err := table.AppendBatch(badData); err == nil {
		t.Errorf("Expected error for missing column")
	}

	// Wrong type
	badData2 := map[string][]interface{}{
		"x": {"not a float"}, // should be float64
		"y": {int64(10)},
	}
	if err := table.AppendBatch(badData2); err == nil {
		t.Errorf("Expected error for wrong data type")
	}

	// Inconsistent slice lengths
	badData3 := map[string][]interface{}{
		"x": {1.0, 2.0},
		"y": {int64(10)}, // different length
	}
	if err := table.AppendBatch(badData3); err == nil {
		t.Errorf("Expected error for inconsistent slice lengths")
	}
}

func TestWriterValidation(t *testing.T) {
	table, _ := NewTable([]ColumnType{Float64, Int64}, []string{"x", "y"})
	writer := table.NewWriter(10)

	// Valid append
	if err := writer.Append(1.0, int64(10)); err != nil {
		t.Errorf("Expected valid append, got error: %v", err)
	}

	// Wrong number of values
	if err := writer.Append(1.0); err == nil {
		t.Errorf("Expected error for wrong number of values")
	}

	// Wrong type
	if err := writer.Append("not a float", int64(10)); err == nil {
		t.Errorf("Expected error for wrong value type")
	}
}
