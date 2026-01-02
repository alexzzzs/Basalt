package basalt

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// CSVReader provides functionality to read CSV files into tables
type CSVReader struct {
	Delimiter rune
	HasHeader bool
}

// NewCSVReader creates a new CSV reader with default settings
func NewCSVReader() *CSVReader {
	return &CSVReader{
		Delimiter: ',',
		HasHeader: true,
	}
}

// ReadTable reads a CSV from an io.Reader and returns a Table
func (r *CSVReader) ReadTable(reader io.Reader, schema []ColumnType, columnNames []string) (*Table, error) {
	csvReader := csv.NewReader(reader)
	csvReader.Comma = r.Delimiter
	csvReader.TrimLeadingSpace = true

	table, err := NewTable(schema, columnNames)
	if err != nil {
		return nil, err
	}

	// Skip header if present
	if r.HasHeader {
		_, err := csvReader.Read()
		if err != nil {
			return nil, fmt.Errorf("failed to read header: %w", err)
		}
	}

	// Read data rows
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV record: %w", err)
		}

		if len(record) != len(schema) {
			return nil, fmt.Errorf("CSV record has %d fields, expected %d", len(record), len(schema))
		}

		// Convert record to interface{} slice
		row := make([]interface{}, len(record))
		for i, field := range record {
			switch schema[i] {
			case Float64:
				val, err := strconv.ParseFloat(strings.TrimSpace(field), 64)
				if err != nil {
					return nil, fmt.Errorf("failed to parse float64 at column %d: %w", i, err)
				}
				row[i] = val
			case Int64:
				val, err := strconv.ParseInt(strings.TrimSpace(field), 10, 64)
				if err != nil {
					return nil, fmt.Errorf("failed to parse int64 at column %d: %w", i, err)
				}
				row[i] = val
			case Bool:
				val, err := strconv.ParseBool(strings.TrimSpace(field))
				if err != nil {
					return nil, fmt.Errorf("failed to parse bool at column %d: %w", i, err)
				}
				row[i] = val
			case String:
				row[i] = strings.TrimSpace(field)
			default:
				return nil, fmt.Errorf("unsupported column type: %v", schema[i])
			}
		}

		// Create data map for AppendBatch
		data := make(map[string][]interface{})
		for i, colName := range columnNames {
			data[colName] = []interface{}{row[i]}
		}

		if err := table.AppendBatch(data); err != nil {
			return nil, fmt.Errorf("failed to append batch: %w", err)
		}
	}

	return table, nil
}

// CSVWriter provides functionality to write tables to CSV files
type CSVWriter struct {
	Delimiter   rune
	WriteHeader bool
}

// NewCSVWriter creates a new CSV writer with default settings
func NewCSVWriter() *CSVWriter {
	return &CSVWriter{
		Delimiter:   ',',
		WriteHeader: true,
	}
}

// WriteTable writes a table to an io.Writer in CSV format
func (w *CSVWriter) WriteTable(writer io.Writer, table *Table) error {
	csvWriter := csv.NewWriter(writer)
	csvWriter.Comma = w.Delimiter
	defer csvWriter.Flush()

	// Write header if requested
	if w.WriteHeader {
		if err := csvWriter.Write(table.ColumnNames); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	// Write data rows from all chunks
	for _, chunk := range table.Chunks {
		for row := 0; row < chunk.Len; row++ {
			record := make([]string, len(chunk.Columns))

			for colIdx, col := range chunk.Columns {
				val, err := readColumnValue(&col, row)
				if err != nil {
					return fmt.Errorf("failed to read column value at row %d, col %d: %w", row, colIdx, err)
				}

				switch v := val.(type) {
				case float64:
					record[colIdx] = strconv.FormatFloat(v, 'f', -1, 64)
				case int64:
					record[colIdx] = strconv.FormatInt(v, 10)
				case bool:
					record[colIdx] = strconv.FormatBool(v)
				case string:
					record[colIdx] = v
				default:
					return fmt.Errorf("unsupported value type: %T", val)
				}
			}

			if err := csvWriter.Write(record); err != nil {
				return fmt.Errorf("failed to write record: %w", err)
			}
		}
	}

	return nil
}
