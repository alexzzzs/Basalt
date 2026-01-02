package basalt

import (
	"encoding/binary"
	"math"
)

// NewTable creates a new table with the given schema and column names.
// Returns an error if the schema or column names are invalid.
func NewTable(schema []ColumnType, columnNames []string) (*Table, error) {
	if err := ValidateSchema(schema, columnNames); err != nil {
		return nil, err
	}
	return &Table{
		Chunks:      []*Chunk{},
		Schema:      schema,
		ColumnNames: columnNames,
	}, nil
}

// AppendBatch appends a batch of data to the table as a new chunk.
// The data map should contain column names as keys and slices of values as values.
// All columns must have the same number of values in their slices.
// Returns an error if data validation fails.
func (t *Table) AppendBatch(data map[string][]interface{}) error {
	if len(data) == 0 {
		return nil
	}

	// Validate data types and structure
	if err := ValidateDataTypes(t, data); err != nil {
		return err
	}

	// Determine batch size
	batchSize := len(data[t.getColumnNames()[0]])

	// Create new chunk
	chunk := &Chunk{
		Columns: make([]Column, len(t.Schema)),
		Len:     batchSize,
	}

	for i, colName := range t.getColumnNames() {
		colType := t.Schema[i]
		values := data[colName]

		if colType == String {
			column := Column{
				Type:    colType,
				Name:    colName,
				Strings: make([]string, batchSize),
			}
			for j, val := range values {
				column.Strings[j] = val.(string)
			}
			chunk.Columns[i] = column
		} else {
			column := Column{
				Type: colType,
				Name: colName,
				Data: make([]byte, batchSize*t.getTypeSize(colType)),
			}

			for j, val := range values {
				offset := j * t.getTypeSize(colType)
				switch colType {
				case Float64:
					f64 := val.(float64)
					binary.LittleEndian.PutUint64(column.Data[offset:], math.Float64bits(f64))
				case Int64:
					i64 := val.(int64)
					binary.LittleEndian.PutUint64(column.Data[offset:], uint64(i64))
				case Bool:
					b := val.(bool)
					if b {
						column.Data[offset] = 1
					} else {
						column.Data[offset] = 0
					}
				}
			}
			chunk.Columns[i] = column
		}
	}

	t.Chunks = append(t.Chunks, chunk)
	return nil
}

// getColumnNames returns column names in order
func (t *Table) getColumnNames() []string {
	return t.ColumnNames
}

func (t *Table) getTypeSize(colType ColumnType) int {
	switch colType {
	case Float64, Int64:
		return 8
	case Bool:
		return 1
	default:
		return 0
	}
}

// Writer provides streaming data ingestion for tables with buffering.
// It accumulates rows in memory and automatically flushes to the table when the buffer is full.
type Writer struct {
	table   *Table
	buffer  map[string][]interface{}
	bufSize int
}

// NewWriter creates a new streaming writer for the table with the specified buffer size.
// The buffer size determines how many rows are accumulated before auto-flushing.
func (t *Table) NewWriter(bufSize int) *Writer {
	return &Writer{
		table:   t,
		buffer:  make(map[string][]interface{}),
		bufSize: bufSize,
	}
}

// Append adds a single row to the buffer.
// Returns an error if the number of values doesn't match the table schema or types are invalid.
// Automatically flushes when the buffer reaches bufSize.
func (w *Writer) Append(values ...interface{}) error {
	if len(values) != len(w.table.Schema) {
		return ErrSchemaMismatch
	}

	// Validate each value type
	colNames := w.table.getColumnNames()
	for i, val := range values {
		colType := w.table.Schema[i]
		if err := ValidateValueType(val, colType); err != nil {
			return err
		}
		colName := colNames[i]
		w.buffer[colName] = append(w.buffer[colName], val)
	}

	// Auto-flush if buffer full
	if len(w.buffer[colNames[0]]) >= w.bufSize {
		return w.Flush()
	}
	return nil
}

// Flush writes all buffered data to the table as a new chunk and clears the buffer.
// Safe to call on an empty buffer. Returns an error if data validation fails.
func (w *Writer) Flush() error {
	if len(w.buffer) > 0 {
		err := w.table.AppendBatch(w.buffer)
		if err != nil {
			return err
		}
		w.buffer = make(map[string][]interface{})
	}
	return nil
}
