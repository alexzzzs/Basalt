package main

import (
	"fmt"

	"github.com/alexzzzs/Basalt"
)

func main() {
	// Create table
	tbl, err := basalt.NewTable([]basalt.ColumnType{basalt.Float64, basalt.Float64, basalt.Float64}, []string{"x", "y", "z"})
	if err != nil {
		fmt.Printf("Error creating table: %v\n", err)
		return
	}

	// Ingest data
	data := map[string][]interface{}{
		"x": {1.0, -1.0, 2.0},
		"y": {3.0, 4.0, 6.0},
		"z": {10.0, 20.0, 30.0},
	}
	if err := tbl.AppendBatch(data); err != nil {
		fmt.Printf("Error appending batch: %v\n", err)
		return
	}

	// Create engine
	engine := basalt.NewEngine(4)

	// Create plan
	result, err := engine.From(tbl).
		Filter(basalt.Greater{Column: "x", Value: 0.0}).
		MapIf(basalt.Greater{Column: "y", Value: 5.0}, basalt.Multiply{Column: "z", Scalar: 2.0}).
		Reduce(basalt.Sum{Column: "z"}).
		Run()

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Result: %v\n", result.Data)
}
