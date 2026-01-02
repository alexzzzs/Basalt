package basalt

import (
	"testing"
)

func TestEngineRun(t *testing.T) {
	// Create table
	table, err := NewTable([]ColumnType{Float64, Int64, Float64}, []string{"x", "y", "z"})
	if err != nil {
		t.Fatalf("Unexpected error creating table: %v", err)
	}
	data := map[string][]interface{}{
		"x": {1.0, 2.0, 3.0},
		"y": {int64(10), int64(20), int64(30)},
		"z": {10.0, 20.0, 30.0},
	}
	if err := table.AppendBatch(data); err != nil {
		t.Fatalf("Unexpected error appending batch: %v", err)
	}

	// Create engine
	engine := NewEngine(4)

	// Create plan: filter x > 1.5, multiply z by 2 if y > 15, sum z
	result, err := engine.From(table).
		Filter(Greater{Column: "x", Value: 1.5}).
		MapIf(Greater{Column: "y", Value: int64(15)}, Multiply{Column: "z", Scalar: 2.0}).
		Reduce(Sum{Column: "z"}).
		Run()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// After filter: rows 2 and 3 (x=2.0,3.0)
	// For row 2: y=20 > 15, so z=20 * 2 = 40
	// For row 3: y=30 > 15, so z=30 * 2 = 60
	// Sum: 40 + 60 = 100.0
	expected := 100.0
	if res, ok := result.Data.(float64); ok {
		if res != expected {
			t.Errorf("Expected result %v, got %v", expected, res)
		}
	} else {
		t.Errorf("Expected float64 result, got %T", result.Data)
	}
}

func TestEngineAggregations(t *testing.T) {
	table, err := NewTable([]ColumnType{Float64}, []string{"x"})
	if err != nil {
		t.Fatalf("Unexpected error creating table: %v", err)
	}
	data := map[string][]interface{}{
		"x": {1.0, 2.0, 3.0, 4.0, 5.0},
	}
	if err := table.AppendBatch(data); err != nil {
		t.Fatalf("Unexpected error appending batch: %v", err)
	}

	engine := NewEngine(1)

	// Test Min
	result, err := engine.From(table).
		Reduce(Min{Column: "x"}).
		Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expected := 1.0
	if res, ok := result.Data.(float64); ok {
		if res != expected {
			t.Errorf("Expected min result %v, got %v", expected, res)
		}
	} else {
		t.Errorf("Expected float64 result, got %T", result.Data)
	}

	// Test Max
	result, err = engine.From(table).
		Reduce(Max{Column: "x"}).
		Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expected = 5.0
	if res, ok := result.Data.(float64); ok {
		if res != expected {
			t.Errorf("Expected max result %v, got %v", expected, res)
		}
	} else {
		t.Errorf("Expected float64 result, got %T", result.Data)
	}

	// Test Average
	result, err = engine.From(table).
		Reduce(Average{Column: "x"}).
		Run()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expected = 3.0 // (1+2+3+4+5)/5 = 3.0
	if res, ok := result.Data.(float64); ok {
		if res != expected {
			t.Errorf("Expected average result %v, got %v", expected, res)
		}
	} else {
		t.Errorf("Expected float64 result, got %T", result.Data)
	}
}

func TestEngineSimpleFilter(t *testing.T) {
	table, err := NewTable([]ColumnType{Float64, Int64, Float64}, []string{"x", "y", "z"})
	if err != nil {
		t.Fatalf("Unexpected error creating table: %v", err)
	}
	data := map[string][]interface{}{
		"x": {1.0, 2.0, 3.0},
		"y": {int64(1), int64(2), int64(3)},
		"z": {1.0, 2.0, 3.0},
	}
	if err := table.AppendBatch(data); err != nil {
		t.Fatalf("Unexpected error appending batch: %v", err)
	}

	engine := NewEngine(1)

	// Filter x < 2.5, sum x
	result, err := engine.From(table).
		Filter(Less{Column: "x", Value: 2.5}).
		Reduce(Sum{Column: "x"}).
		Run()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Rows 1 and 2: 1.0 + 2.0 = 3.0
	expected := 3.0
	if res, ok := result.Data.(float64); ok {
		if res != expected {
			t.Errorf("Expected result %v, got %v", expected, res)
		}
	} else {
		t.Errorf("Expected float64 result, got %T", result.Data)
	}
}

func TestEngineInt64Arithmetic(t *testing.T) {
	table, err := NewTable([]ColumnType{Int64, Int64}, []string{"a", "b"})
	if err != nil {
		t.Fatalf("Unexpected error creating table: %v", err)
	}
	data := map[string][]interface{}{
		"a": {int64(10), int64(20), int64(30)},
		"b": {int64(2), int64(4), int64(6)},
	}
	if err := table.AppendBatch(data); err != nil {
		t.Fatalf("Unexpected error appending batch: %v", err)
	}

	engine := NewEngine(1)

	// Test Int64 multiplication: a * 2
	result, err := engine.From(table).
		Map(Multiply{Column: "a", Scalar: int64(2)}).
		Reduce(Sum{Column: "a"}).
		Run()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// a values after multiply: 20, 40, 60. Sum: 120.0
	expected := 120.0
	if res, ok := result.Data.(float64); ok {
		if res != expected {
			t.Errorf("Expected result %v, got %v", expected, res)
		}
	} else {
		t.Errorf("Expected float64 result, got %T", result.Data)
	}
}

func TestEngineSort(t *testing.T) {
	table, err := NewTable([]ColumnType{Float64, Int64}, []string{"x", "y"})
	if err != nil {
		t.Fatalf("Unexpected error creating table: %v", err)
	}
	data := map[string][]interface{}{
		"x": {3.0, 1.0, 2.0},
		"y": {int64(30), int64(10), int64(20)},
	}
	if err := table.AppendBatch(data); err != nil {
		t.Fatalf("Unexpected error appending batch: %v", err)
	}

	engine := NewEngine(1)

	// Sort by x ascending, then sum y
	result, err := engine.From(table).
		Sort("x", true).
		Reduce(Sum{Column: "y"}).
		Run()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// After sorting by x ascending: y values should be 10, 20, 30. Sum: 60.0
	expected := 60.0
	if res, ok := result.Data.(float64); ok {
		if res != expected {
			t.Errorf("Expected result %v, got %v", expected, res)
		}
	} else {
		t.Errorf("Expected float64 result, got %T", result.Data)
	}
}
