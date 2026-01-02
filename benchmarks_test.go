package basalt

import (
	"testing"
)

// BenchmarkSum benchmarks the Sum kernel performance
func BenchmarkSum(b *testing.B) {
	chunk := createTestChunk()

	kernel := Sum{Column: "x"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := kernel.Execute(chunk, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMultiply benchmarks the Multiply kernel performance
func BenchmarkMultiply(b *testing.B) {
	chunk := createTestChunk()

	kernel := Multiply{Column: "x", Scalar: 2.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := kernel.Execute(chunk, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFilter benchmarks predicate evaluation performance
func BenchmarkFilter(b *testing.B) {
	chunk := createTestChunk()

	pred := Greater{Column: "x", Value: 1.5}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pred.Evaluate(chunk)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEngineRun benchmarks full engine execution
func BenchmarkEngineRun(b *testing.B) {
	table, _ := NewTable([]ColumnType{Float64, Int64, Float64}, []string{"x", "y", "z"})
	data := map[string][]interface{}{
		"x": {1.0, 2.0, 3.0},
		"y": {int64(10), int64(20), int64(30)},
		"z": {10.0, 20.0, 30.0},
	}
	table.AppendBatch(data)

	engine := NewEngine(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.From(table).
			Filter(Greater{Column: "x", Value: 1.5}).
			MapIf(Greater{Column: "y", Value: int64(15)}, Multiply{Column: "z", Scalar: 2.0}).
			Reduce(Sum{Column: "z"}).
			Run()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkTableAppendBatch benchmarks table batch append performance
func BenchmarkTableAppendBatch(b *testing.B) {
	table, _ := NewTable([]ColumnType{Float64, Int64, Bool}, []string{"x", "y", "z"})

	data := map[string][]interface{}{
		"x": {1.5, 2.5},
		"y": {int64(10), int64(20)},
		"z": {true, false},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := table.AppendBatch(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSort benchmarks sorting performance
func BenchmarkSort(b *testing.B) {
	table, _ := NewTable([]ColumnType{Float64, Int64}, []string{"x", "y"})
	data := map[string][]interface{}{
		"x": {3.0, 1.0, 2.0},
		"y": {int64(30), int64(10), int64(20)},
	}
	table.AppendBatch(data)

	engine := NewEngine(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.From(table).
			Sort("x", true).
			Run()
		if err != nil {
			b.Fatal(err)
		}
	}
}
