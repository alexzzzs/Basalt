
# Basalt

A lightweight, concurrent dataframe library for Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/alexzzzs/Basalt.svg)](https://pkg.go.dev/github.com/alexzzzs/Basalt)
[![Go Report Card](https://goreportcard.com/badge/github.com/alexzzzs/Basalt)](https://goreportcard.com/report/github.com/alexzzzs/Basalt)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Why?

I built Basalt because I needed a way to process tabular numerical data in Go without the overhead of full-blown data science toolkits or the fragility of raw `[][]string` parsing.

It's designed to be a middle ground: strictly typed and columnar (like a database), but with a fluent API that feels a bit like using LINQ or Pandas. It relies heavily on worker pools to keep operations fast across multiple cores.

## Key Features

*   **Columnar Storage:** Data is stored contiguously in memory, which is much friendlier to CPU caches than arrays of structs.
*   **Concurrent by Default:** Heavy lifting (maps, filters, aggregations) automatically utilizes worker pools.
*   **Zero-Reflection Hot Paths:** Designed to avoid reflection overhead during actual data processing.
*   **CSV Friendly:** Includes a robust reader/writer that handles type conversion for you.

## Installation

```bash
go get github.com/alexzzzs/Basalt
```

## Usage Example

Here is a real-world scenario: loading product data, filtering for high-value items, applying a discount, and calculating the new average.

```go
package main

import (
    "fmt"
    "github.com/alexzzzs/Basalt"
)

func main() {
    // 1. Define the schema
    // We use columnar arrays for better memory locality
    table, err := basalt.NewTable(
        []basalt.ColumnType{basalt.Float64, basalt.Float64},
        []string{"price", "inventory_count"},
    )
    if err != nil {
        panic(err)
    }

    // 2. Load some raw data
    // In a real app, this would likely come from the CSV reader
    data := map[string][]interface{}{
        "price":           {10.50, 150.00, 15.75, 300.00},
        "inventory_count": {100.0, 50.0, 75.0, 5.0},
    }

    if err := table.AppendBatch(data); err != nil {
        panic(err)
    }

    // 3. The Pipeline
    // Find items over $100, apply a 10% discount, and find the average price.
    result, err := basalt.NewEngine(4).From(table).
        Filter(basalt.Greater{Column: "price", Value: 100.0}).
        Map(basalt.Multiply{Column: "price", Scalar: 0.90}). // Apply 10% off
        Reduce(basalt.Average{Column: "price"}).
        Run()

    if err != nil {
        panic(err)
    }

    fmt.Printf("Average discounted price of premium items: $%.2f\n", result.Data)
    // Output: Average discounted price of premium items: $202.50
}
```

## Column-to-Column Operations

Basalt supports element-wise arithmetic operations between columns:

```go
// Calculate total value as price * quantity
result, err := basalt.NewEngine(4).From(table).
    Map(basalt.MultiplyColumns{Left: "price", Right: "quantity"}).
    Reduce(basalt.Sum{Column: "price_quantity"}).
    Run()

// Calculate profit margin as (revenue - cost) / revenue
result, err := basalt.NewEngine(4).From(table).
    Map(basalt.SubtractColumns{Left: "revenue", Right: "cost"}).
    Map(basalt.DivideColumns{Left: "revenue_cost", Right: "revenue"}).
    Reduce(basalt.Average{Column: "revenue_cost_div_revenue"}).
    Run()
```

Column-to-column operations support mixed numeric types (Float64 and Int64) and automatically promote to the appropriate result type.

## Supported Operations

The engine supports a focused set of primitives. Check the [GoDocs](https://pkg.go.dev/github.com/alexzzzs/Basalt) for the full API surface.

*   **Filters:** Standard comparisons (`Greater`, `Less`, `Equals`) plus logical composition (`And`, `Or`, `Not`).
*   **Transformations:**
  * Scalar math: `Multiply`, `Add`, `Subtract`, `Divide`, `Power`
  * Column-to-Column math: `AddColumns`, `SubtractColumns`, `MultiplyColumns`, `DivideColumns`, `PowerColumns`
*   **Stats:** `Sum`, `Average`, `Min`, `Max`, `Variance`, `StdDev`, `Median`, `Count`.

## Performance

Benchmarks run on an Intel i7-13620H.

The columnar design allows us to keep allocations very low. A simple sum operation generates only ~3 allocations regardless of table size.

| Operation | Time/Op | Allocs/Op |
| :--- | :--- | :--- |
| **Sum** | ~57ns | 3 |
| **Filter** | ~46ns | 4 |

## Contributing

PRs are welcome! If you're looking to help, we currently need more tests around edge-case CSV parsing.

1. Fork it
2. Create your feature branch (`git checkout -b feature/my-new-feature`)
3. Commit your changes (`git commit -am 'Add some feature'`)
4. Push to the branch (`git push origin feature/my-new-feature`)
5. Create a new Pull Request

## License

MIT
