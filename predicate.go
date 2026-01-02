package basalt

import (
	"encoding/binary"
	"math"
)

// Greater predicate compares column values for greater than relationship
type Greater struct {
	Column string      // column name to compare
	Value  interface{} // value to compare against
}

func (p Greater) Evaluate(chunk *Chunk) (Mask, error) {
	return evaluateComparison(chunk, p.Column, p.Value, func(a, b interface{}) bool {
		return compareValues(a, b) > 0 // greater than
	})
}

// Less predicate compares column values for less than relationship
type Less struct {
	Column string      // column name to compare
	Value  interface{} // value to compare against
}

func (p Less) Evaluate(chunk *Chunk) (Mask, error) {
	return evaluateComparison(chunk, p.Column, p.Value, func(a, b interface{}) bool {
		return compareValues(a, b) < 0 // less than
	})
}

// And predicate combines two predicates with logical AND
type And struct {
	Left, Right Predicate // predicates to combine
}

func (p And) Evaluate(chunk *Chunk) (Mask, error) {
	leftMask, err := p.Left.Evaluate(chunk)
	if err != nil {
		return nil, err
	}
	rightMask, err := p.Right.Evaluate(chunk)
	if err != nil {
		return nil, err
	}
	result := make(Mask, len(leftMask))
	for i := range result {
		result[i] = leftMask[i] && rightMask[i]
	}
	return result, nil
}

// Or predicate combines two predicates with logical OR
type Or struct {
	Left, Right Predicate // predicates to combine
}

func (p Or) Evaluate(chunk *Chunk) (Mask, error) {
	leftMask, err := p.Left.Evaluate(chunk)
	if err != nil {
		return nil, err
	}
	rightMask, err := p.Right.Evaluate(chunk)
	if err != nil {
		return nil, err
	}
	result := make(Mask, len(leftMask))
	for i := range result {
		result[i] = leftMask[i] || rightMask[i]
	}
	return result, nil
}

// Equals predicate compares column values for equality
type Equals struct {
	Column string      // column name to compare
	Value  interface{} // value to compare against
}

func (p Equals) Evaluate(chunk *Chunk) (Mask, error) {
	return evaluateComparison(chunk, p.Column, p.Value, func(a, b interface{}) bool {
		return compareValues(a, b) == 0 // equal
	})
}

// Not predicate negates the result of another predicate
type Not struct {
	Predicate Predicate // predicate to negate
}

func (p Not) Evaluate(chunk *Chunk) (Mask, error) {
	innerMask, err := p.Predicate.Evaluate(chunk)
	if err != nil {
		return nil, err
	}
	result := make(Mask, len(innerMask))
	for i := range result {
		result[i] = !innerMask[i]
	}
	return result, nil
}

// readColumnValue safely reads a value from a column at the given index
func readColumnValue(col *Column, index int) (interface{}, error) {
	if index < 0 {
		return nil, ErrInvalidOperation
	}

	switch col.Type {
	case Float64:
		if index >= len(col.Data)/8 {
			return nil, ErrInvalidOperation
		}
		offset := index * 8
		if offset+8 > len(col.Data) {
			return nil, ErrInvalidOperation
		}
		bits := binary.LittleEndian.Uint64(col.Data[offset:])
		return math.Float64frombits(bits), nil
	case Int64:
		if index >= len(col.Data)/8 {
			return nil, ErrInvalidOperation
		}
		offset := index * 8
		if offset+8 > len(col.Data) {
			return nil, ErrInvalidOperation
		}
		bits := binary.LittleEndian.Uint64(col.Data[offset:])
		return int64(bits), nil
	case Bool:
		if index >= len(col.Data) {
			return nil, ErrInvalidOperation
		}
		return col.Data[index] != 0, nil
	case String:
		if index >= len(col.Strings) {
			return nil, ErrInvalidOperation
		}
		return col.Strings[index], nil
	default:
		return nil, ErrUnsupportedType
	}
}

func findColumn(chunk *Chunk, name string) *Column {
	for i := range chunk.Columns {
		if chunk.Columns[i].Name == name {
			return &chunk.Columns[i]
		}
	}
	return nil
}

// evaluateComparison provides a generic comparison evaluation framework
func evaluateComparison(chunk *Chunk, columnName string, compareValue interface{}, compareFunc func(a, b interface{}) bool) (Mask, error) {
	col := findColumn(chunk, columnName)
	if col == nil {
		return nil, ErrColumnNotFound
	}

	// Validate that the comparison value type matches the column type
	if err := ValidatePredicateValue(col.Type, compareValue); err != nil {
		return nil, err
	}

	mask := make(Mask, chunk.Len)
	for i := 0; i < chunk.Len; i++ {
		val, err := readColumnValue(col, i)
		if err != nil {
			return nil, err
		}

		if compareFunc(val, compareValue) {
			mask[i] = true
		}
	}
	return mask, nil
}

// compareValues compares two values and returns -1, 0, or 1 for less, equal, greater
func compareValues(a, b interface{}) int {
	switch va := a.(type) {
	case float64:
		if vb, ok := b.(float64); ok {
			if va < vb {
				return -1
			} else if va > vb {
				return 1
			}
			return 0
		}
	case int64:
		if vb, ok := b.(int64); ok {
			if va < vb {
				return -1
			} else if va > vb {
				return 1
			}
			return 0
		}
	case string:
		if vb, ok := b.(string); ok {
			if va < vb {
				return -1
			} else if va > vb {
				return 1
			}
			return 0
		}
	case bool:
		if vb, ok := b.(bool); ok {
			if va == vb {
				return 0
			}
			// For bool, we can consider true > false
			if va && !vb {
				return 1
			}
			return -1
		}
	}
	return 0 // default to equal if types don't match
}

func getTypeSize(colType ColumnType) int {
	switch colType {
	case Float64, Int64:
		return 8
	case Bool:
		return 1
	case String:
		return 0 // Variable length
	default:
		return 0
	}
}
