# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-01-02

### Added

- Initial release of Basalt.
- Columnar storage for efficient memory access and CPU cache friendliness
- Concurrent execution engine with worker pools for multi-core
- Support for multiple data types: Float64, Int64, Bool, String
- Robust CSV reader and writer with automatic type conversion
- Filtering predicates: Greater, Less, Equals, And, Or, Not
- Scalar arithmetic transformations: Multiply, Add, Subtract, Divide, Power
- Statistical aggregations: Sum, Average, Min, Max, Variance, StdDev, Median, Count
- Sorting operations on columns (ascending/descending)
- API for building and executing query plans
- Chunk-based processing for handling large datasets
- Comprehensive test suite covering core functionality
- Performance benchmarks
- Example CLI application demonstrating usage
