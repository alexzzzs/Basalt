// Package basalt provides a columnar in-memory data processing engine
// for efficient numerical computations on tabular data.
package basalt

import "errors"

// ColumnType represents the data type of a column in a table.
type ColumnType int

const (
	// Float64 represents 64-bit floating point numbers.
	Float64 ColumnType = iota
	// Int64 represents 64-bit signed integers.
	Int64
	// Bool represents boolean values.
	Bool
	// String represents UTF-8 encoded strings.
	String
)

// Common errors
var (
	ErrColumnNotFound         = errors.New("column not found")
	ErrInvalidColumnType      = errors.New("invalid column type")
	ErrInvalidOperation       = errors.New("invalid operation")
	ErrPredicateNil           = errors.New("predicate cannot be nil")
	ErrInvalidKernel          = errors.New("invalid kernel")
	ErrUnsupportedType        = errors.New("unsupported column type")
	ErrInvalidValueType       = errors.New("invalid value type for comparison")
	ErrSchemaMismatch         = errors.New("schema and column names length mismatch")
	ErrDuplicateColumnName    = errors.New("duplicate column name")
	ErrEmptyColumnName        = errors.New("empty column name")
	ErrInvalidColumnTypeValue = errors.New("invalid column type value")
	ErrDataTypeMismatch       = errors.New("data type does not match column schema")
	ErrValueSliceLength       = errors.New("value slices have inconsistent lengths")
	ErrMissingColumnData      = errors.New("missing data for required column")
	ErrInvalidStringValue     = errors.New("invalid string value")
	ErrInvalidNumericValue    = errors.New("invalid numeric value")
	ErrInvalidBoolValue       = errors.New("invalid boolean value")
)

// Column represents a single column of data with type-specific storage.
// For numeric and boolean types, data is stored in binary format in the Data field.
// For string types, data is stored in the Strings field.
type Column struct {
	Data    []byte   // packed values for numeric/bool types
	Strings []string // string values (used when Type == String)
	Type    ColumnType
	Name    string
}

// Chunk represents a fixed-size batch of rows containing multiple columns.
// All columns in a chunk have the same number of rows.
type Chunk struct {
	Columns []Column
	Len     int // number of rows
}

// Table represents a collection of chunks with a defined schema.
// Tables support efficient columnar operations and streaming data ingestion.
type Table struct {
	Chunks      []*Chunk
	Schema      []ColumnType // column types in order
	ColumnNames []string     // column names in order
}

// Mask represents conditional selection per row, where true indicates
// the row should be included in the operation.
type Mask []bool

// Predicate represents a composable condition that can be evaluated
// against a chunk to produce a boolean mask.
type Predicate interface {
	Evaluate(chunk *Chunk) (Mask, error)
}

// Kernel represents a computation kernel that performs transformations
// on column data, optionally using a mask for conditional operations.
type Kernel interface {
	Execute(chunk *Chunk, mask Mask) ([]Column, error)
}

// Operator represents a transformation operation that can be applied
// to a chunk. Returns Chunk for data transformations, Mask for filtering,
// or scalar values for reductions.
type Operator interface {
	Apply(chunk *Chunk) (interface{}, error)
}

// Plan represents a directed acyclic graph (DAG) of operators that
// define a complete data processing pipeline.
type Plan struct {
	Ops []Operator
}

// Execute runs the plan on the given chunk, applying all operators in sequence.
// Returns the final result which may be a scalar value, chunk, or other data type.
func (p *Plan) Execute(chunk *Chunk) (interface{}, error) {
	var mask Mask
	currentChunk := chunk

	for _, op := range p.Ops {
		switch o := op.(type) {
		case Filter:
			result, err := o.Apply(currentChunk)
			if err != nil {
				return nil, err
			}
			if m, ok := result.(Mask); ok {
				mask = m
			}
		case MapIf:
			result, err := o.Apply(currentChunk)
			if err != nil {
				return nil, err
			}
			if c, ok := result.(*Chunk); ok {
				currentChunk = c
			}
		case Map:
			result, err := o.Apply(currentChunk)
			if err != nil {
				return nil, err
			}
			if c, ok := result.(*Chunk); ok {
				currentChunk = c
			}
		case Sort:
			result, err := o.Apply(currentChunk)
			if err != nil {
				return nil, err
			}
			if c, ok := result.(*Chunk); ok {
				currentChunk = c
			}
		case Reduce:
			if mask != nil {
				o.Mask = mask
			}
			return o.Apply(currentChunk)
		case ReduceIf:
			if mask != nil {
				o.Mask = mask
			}
			return o.Apply(currentChunk)
			// case Branch: // commented out
		default:
			return nil, ErrInvalidOperation
		}
	}

	// If we reach here, the plan didn't end with a reduction.
	// Return the final chunk, applying any accumulated mask if present.
	if mask != nil {
		// Apply mask to current chunk by filtering rows
		filteredChunk := &Chunk{
			Columns: make([]Column, len(currentChunk.Columns)),
			Len:     0, // will count filtered rows
		}

		// Count how many rows pass the mask
		for _, masked := range mask {
			if masked {
				filteredChunk.Len++
			}
		}

		// Create filtered columns
		for i, col := range currentChunk.Columns {
			filteredChunk.Columns[i] = Column{
				Name: col.Name,
				Type: col.Type,
			}

			if col.Type == String {
				filteredChunk.Columns[i].Strings = make([]string, 0, filteredChunk.Len)
			} else {
				size := getTypeSize(col.Type)
				filteredChunk.Columns[i].Data = make([]byte, filteredChunk.Len*size)
			}

			// Copy masked rows
			destIdx := 0
			for srcIdx, masked := range mask {
				if masked {
					if col.Type == String {
						filteredChunk.Columns[i].Strings = append(filteredChunk.Columns[i].Strings, col.Strings[srcIdx])
					} else {
						size := getTypeSize(col.Type)
						copy(filteredChunk.Columns[i].Data[destIdx*size:(destIdx+1)*size],
							col.Data[srcIdx*size:(srcIdx+1)*size])
					}
					destIdx++
				}
			}
		}
		return filteredChunk, nil
	}

	return currentChunk, nil
}

// Engine represents the execution engine that processes data processing plans.
// It manages worker pools for concurrent chunk processing.
type Engine struct {
	// worker pool configuration
	numWorkers int
}

// Result represents the output of a plan execution.
// The Data field contains scalar values, chunks, or other results depending on the plan.
type Result struct {
	Data interface{} // result data (scalar, chunk, etc.)
}

// ValidateSchema validates that a schema and column names are properly formed
func ValidateSchema(schema []ColumnType, columnNames []string) error {
	if len(schema) != len(columnNames) {
		return ErrSchemaMismatch
	}

	// Check for valid column types and non-empty, unique names
	nameSet := make(map[string]bool)
	for i, colType := range schema {
		name := columnNames[i]

		// Check column type is valid
		if colType < Float64 || colType > String {
			return ErrInvalidColumnTypeValue
		}

		// Check name is not empty
		if name == "" {
			return ErrEmptyColumnName
		}

		// Check name is unique
		if nameSet[name] {
			return ErrDuplicateColumnName
		}
		nameSet[name] = true
	}

	return nil
}

// ValidateDataTypes validates that provided data matches the table schema
func ValidateDataTypes(table *Table, data map[string][]interface{}) error {
	if len(data) == 0 {
		return nil
	}

	// Check all required columns are present
	columnNameSet := make(map[string]bool)
	for _, name := range table.ColumnNames {
		columnNameSet[name] = true
	}

	for colName := range data {
		if !columnNameSet[colName] {
			return ErrColumnNotFound
		}
	}

	// Check all columns have data
	for _, colName := range table.ColumnNames {
		if _, exists := data[colName]; !exists {
			return ErrMissingColumnData
		}
	}

	// Determine batch size and validate all slices have same length
	var batchSize int = -1
	for _, values := range data {
		if batchSize == -1 {
			batchSize = len(values)
		} else if len(values) != batchSize {
			return ErrValueSliceLength
		}
	}

	// Validate each value's type matches schema
	for i, colName := range table.ColumnNames {
		expectedType := table.Schema[i]
		values := data[colName]

		for _, val := range values {
			if err := ValidateValueType(val, expectedType); err != nil {
				return err
			}
		}
	}

	return nil
}

// ValidateValueType validates that a single value matches the expected column type
func ValidateValueType(value interface{}, expectedType ColumnType) error {
	switch expectedType {
	case Float64:
		if _, ok := value.(float64); !ok {
			return ErrInvalidNumericValue
		}
	case Int64:
		if _, ok := value.(int64); !ok {
			return ErrInvalidNumericValue
		}
	case Bool:
		if _, ok := value.(bool); !ok {
			return ErrInvalidBoolValue
		}
	case String:
		if _, ok := value.(string); !ok {
			return ErrInvalidStringValue
		}
	default:
		return ErrInvalidColumnType
	}
	return nil
}

// ValidateColumnExists checks if a column exists in the table
func ValidateColumnExists(table *Table, column string) error {
	for _, colName := range table.ColumnNames {
		if colName == column {
			return nil
		}
	}
	return ErrColumnNotFound
}

// ValidatePredicateValue validates that a predicate comparison value is compatible with the column type
func ValidatePredicateValue(columnType ColumnType, value interface{}) error {
	switch columnType {
	case Float64:
		if _, ok := value.(float64); !ok {
			return ErrInvalidValueType
		}
	case Int64:
		if _, ok := value.(int64); !ok {
			return ErrInvalidValueType
		}
	case Bool:
		if _, ok := value.(bool); !ok {
			return ErrInvalidValueType
		}
	case String:
		if _, ok := value.(string); !ok {
			return ErrInvalidValueType
		}
	default:
		return ErrUnsupportedType
	}
	return nil
}

// ValidateKernelScalar validates scalar parameters for kernels
// Allows numeric types to be compatible (int64 can be used with float64 columns and vice versa)
func ValidateKernelScalar(scalar interface{}, allowedTypes []ColumnType) error {
	for _, allowedType := range allowedTypes {
		switch allowedType {
		case Float64:
			if _, ok := scalar.(float64); ok {
				return nil
			}
			if _, ok := scalar.(int64); ok {
				return nil
			}
		case Int64:
			if _, ok := scalar.(int64); ok {
				return nil
			}
			if _, ok := scalar.(float64); ok {
				return nil
			}
		case Bool:
			if _, ok := scalar.(bool); ok {
				return nil
			}
		case String:
			if _, ok := scalar.(string); ok {
				return nil
			}
		}
	}
	return ErrInvalidValueType
}
