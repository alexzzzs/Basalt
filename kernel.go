package basalt

import (
	"encoding/binary"
	"math"
)

// arithmeticOp defines the signature for arithmetic operations
type arithmeticOp func(float64, float64) float64

// applyArithmeticScalar applies a scalar arithmetic operation to a column
func applyArithmeticScalar(chunk *Chunk, columnName string, scalar interface{}, mask Mask, op arithmeticOp) ([]Column, error) {
	col := findColumn(chunk, columnName)
	if col == nil {
		return nil, ErrColumnNotFound
	}

	// Validate scalar type is compatible with column type
	if err := ValidateKernelScalar(scalar, []ColumnType{col.Type}); err != nil {
		return nil, err
	}

	result := Column{
		Name: col.Name,
		Type: col.Type,
		Data: make([]byte, len(col.Data)),
	}

	switch col.Type {
	case Float64:
		scalarF64, _ := scalar.(float64) // Already validated above
		for i := 0; i < chunk.Len; i++ {
			if i >= len(col.Data)/8 {
				return nil, ErrInvalidOperation
			}
			offset := i * 8
			if offset+8 > len(col.Data) {
				return nil, ErrInvalidOperation
			}
			bits := binary.LittleEndian.Uint64(col.Data[offset:])
			val := math.Float64frombits(bits)
			newVal := op(val, scalarF64)
			if mask != nil && !mask[i] {
				newVal = val // no change if not masked
			}
			if offset+8 > len(result.Data) {
				return nil, ErrInvalidOperation
			}
			binary.LittleEndian.PutUint64(result.Data[offset:], math.Float64bits(newVal))
		}
	case Int64:
		scalarI64, _ := scalar.(int64) // Already validated above
		for i := 0; i < chunk.Len; i++ {
			if i >= len(col.Data)/8 {
				return nil, ErrInvalidOperation
			}
			offset := i * 8
			if offset+8 > len(col.Data) {
				return nil, ErrInvalidOperation
			}
			bits := binary.LittleEndian.Uint64(col.Data[offset:])
			val := int64(bits)
			newVal := int64(op(float64(val), float64(scalarI64)))
			if mask != nil && !mask[i] {
				newVal = val // no change if not masked
			}
			if offset+8 > len(result.Data) {
				return nil, ErrInvalidOperation
			}
			binary.LittleEndian.PutUint64(result.Data[offset:], uint64(newVal))
		}
	default:
		return nil, ErrInvalidColumnType
	}

	return []Column{result}, nil
}

// writeColumnValue safely writes a value to a column at the given index
func writeColumnValue(col *Column, index int, value interface{}) error {
	if index < 0 {
		return ErrInvalidOperation
	}

	switch col.Type {
	case Float64:
		if index >= len(col.Data)/8 {
			return ErrInvalidOperation
		}
		offset := index * 8
		if offset+8 > len(col.Data) {
			return ErrInvalidOperation
		}
		if f64, ok := value.(float64); ok {
			binary.LittleEndian.PutUint64(col.Data[offset:], math.Float64bits(f64))
		} else {
			return ErrInvalidValueType
		}
	case Int64:
		if index >= len(col.Data)/8 {
			return ErrInvalidOperation
		}
		offset := index * 8
		if offset+8 > len(col.Data) {
			return ErrInvalidOperation
		}
		if i64, ok := value.(int64); ok {
			binary.LittleEndian.PutUint64(col.Data[offset:], uint64(i64))
		} else {
			return ErrInvalidValueType
		}
	case Bool:
		if index >= len(col.Data) {
			return ErrInvalidOperation
		}
		if b, ok := value.(bool); ok {
			if b {
				col.Data[index] = 1
			} else {
				col.Data[index] = 0
			}
		} else {
			return ErrInvalidValueType
		}
	case String:
		if index >= len(col.Strings) {
			return ErrInvalidOperation
		}
		if s, ok := value.(string); ok {
			col.Strings[index] = s
		} else {
			return ErrInvalidValueType
		}
	default:
		return ErrUnsupportedType
	}
	return nil
}

// Multiply kernel: multiplies a column by a scalar
type Multiply struct {
	Column string
	Scalar interface{} // can be float64 or int64
}

func (k Multiply) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	return applyArithmeticScalar(chunk, k.Column, k.Scalar, mask, func(a, b float64) float64 { return a * b })
}

// Sum kernel: sums a column
type Sum struct {
	Column string
}

func (k Sum) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	col := findColumn(chunk, k.Column)
	if col == nil {
		return nil, ErrColumnNotFound
	}

	var total float64
	switch col.Type {
	case Float64:
		for i := 0; i < chunk.Len; i++ {
			if mask == nil || mask[i] {
				if i >= len(col.Data)/8 {
					return nil, ErrInvalidOperation
				}
				offset := i * 8
				if offset+8 > len(col.Data) {
					return nil, ErrInvalidOperation
				}
				bits := binary.LittleEndian.Uint64(col.Data[offset:])
				val := math.Float64frombits(bits)
				total += val
			}
		}
	case Int64:
		for i := 0; i < chunk.Len; i++ {
			if mask == nil || mask[i] {
				if i >= len(col.Data)/8 {
					return nil, ErrInvalidOperation
				}
				offset := i * 8
				if offset+8 > len(col.Data) {
					return nil, ErrInvalidOperation
				}
				bits := binary.LittleEndian.Uint64(col.Data[offset:])
				val := int64(bits)
				total += float64(val)
			}
		}
	default:
		return nil, ErrInvalidColumnType
	}

	// Return as a single-value column
	result := Column{
		Name: "sum_" + col.Name,
		Type: Float64,
		Data: make([]byte, 8),
	}
	binary.LittleEndian.PutUint64(result.Data, math.Float64bits(total))
	return []Column{result}, nil
}

// Add kernel: adds a scalar to a column
type Add struct {
	Column string
	Scalar interface{} // can be float64 or int64
}

func (k Add) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	return applyArithmeticScalar(chunk, k.Column, k.Scalar, mask, func(a, b float64) float64 { return a + b })
}

// Subtract kernel: subtracts a scalar from a column
type Subtract struct {
	Column string
	Scalar interface{} // can be float64 or int64
}

func (k Subtract) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	return applyArithmeticScalar(chunk, k.Column, k.Scalar, mask, func(a, b float64) float64 { return a - b })
}

// Min kernel: finds minimum value in a column
type Min struct {
	Column string
}

func (k Min) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	col := findColumn(chunk, k.Column)
	if col == nil {
		return nil, ErrColumnNotFound
	}
	if col.Type != Float64 {
		return nil, ErrInvalidColumnType
	}

	var minVal float64 = math.Inf(1) // positive infinity
	count := 0
	for i := 0; i < chunk.Len; i++ {
		if mask == nil || mask[i] {
			if i >= len(col.Data)/8 {
				return nil, ErrInvalidOperation
			}
			offset := i * 8
			if offset+8 > len(col.Data) {
				return nil, ErrInvalidOperation
			}
			bits := binary.LittleEndian.Uint64(col.Data[offset:])
			val := math.Float64frombits(bits)
			if val < minVal {
				minVal = val
			}
			count++
		}
	}

	if count == 0 {
		minVal = 0
	}

	result := Column{
		Name: "min_" + col.Name,
		Type: Float64,
		Data: make([]byte, 8),
	}
	binary.LittleEndian.PutUint64(result.Data, math.Float64bits(minVal))
	return []Column{result}, nil
}

// Max kernel: finds maximum value in a column
type Max struct {
	Column string
}

func (k Max) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	col := findColumn(chunk, k.Column)
	if col == nil {
		return nil, ErrColumnNotFound
	}
	if col.Type != Float64 {
		return nil, ErrInvalidColumnType
	}

	var maxVal float64 = math.Inf(-1) // negative infinity
	count := 0
	for i := 0; i < chunk.Len; i++ {
		if mask == nil || mask[i] {
			if i >= len(col.Data)/8 {
				return nil, ErrInvalidOperation
			}
			offset := i * 8
			if offset+8 > len(col.Data) {
				return nil, ErrInvalidOperation
			}
			bits := binary.LittleEndian.Uint64(col.Data[offset:])
			val := math.Float64frombits(bits)
			if val > maxVal {
				maxVal = val
			}
			count++
		}
	}

	if count == 0 {
		maxVal = 0
	}

	result := Column{
		Name: "max_" + col.Name,
		Type: Float64,
		Data: make([]byte, 8),
	}
	binary.LittleEndian.PutUint64(result.Data, math.Float64bits(maxVal))
	return []Column{result}, nil
}

// Average kernel: computes average of a column
type Average struct {
	Column string
}

func (k Average) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	col := findColumn(chunk, k.Column)
	if col == nil {
		return nil, ErrColumnNotFound
	}
	if col.Type != Float64 {
		return nil, ErrInvalidColumnType
	}

	var total float64
	count := 0
	for i := 0; i < chunk.Len; i++ {
		if mask == nil || mask[i] {
			if i >= len(col.Data)/8 {
				return nil, ErrInvalidOperation
			}
			offset := i * 8
			if offset+8 > len(col.Data) {
				return nil, ErrInvalidOperation
			}
			bits := binary.LittleEndian.Uint64(col.Data[offset:])
			val := math.Float64frombits(bits)
			total += val
			count++
		}
	}

	var avg float64
	if count > 0 {
		avg = total / float64(count)
	}

	result := Column{
		Name: "avg_" + col.Name,
		Type: Float64,
		Data: make([]byte, 8),
	}
	binary.LittleEndian.PutUint64(result.Data, math.Float64bits(avg))
	return []Column{result}, nil
}

// Divide kernel: divides a column by a scalar
type Divide struct {
	Column string
	Scalar interface{} // can be float64 or int64
}

func (k Divide) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	col := findColumn(chunk, k.Column)
	if col == nil {
		return nil, ErrColumnNotFound
	}

	result := Column{
		Name: col.Name,
		Type: col.Type,
		Data: make([]byte, len(col.Data)),
	}

	// Validate scalar type and check for division by zero
	if err := ValidateKernelScalar(k.Scalar, []ColumnType{col.Type}); err != nil {
		return nil, err
	}

	switch col.Type {
	case Float64:
		scalar := k.Scalar.(float64)
		if scalar == 0 {
			return nil, ErrInvalidOperation
		}
		for i := 0; i < chunk.Len; i++ {
			if i >= len(col.Data)/8 {
				return nil, ErrInvalidOperation
			}
			offset := i * 8
			if offset+8 > len(col.Data) {
				return nil, ErrInvalidOperation
			}
			bits := binary.LittleEndian.Uint64(col.Data[offset:])
			val := math.Float64frombits(bits)
			newVal := val / scalar
			if mask != nil && !mask[i] {
				newVal = val // no change if not masked
			}
			if offset+8 > len(result.Data) {
				return nil, ErrInvalidOperation
			}
			binary.LittleEndian.PutUint64(result.Data[offset:], math.Float64bits(newVal))
		}
	case Int64:
		scalar := k.Scalar.(int64)
		if scalar == 0 {
			return nil, ErrInvalidOperation
		}
		for i := 0; i < chunk.Len; i++ {
			if i >= len(col.Data)/8 {
				return nil, ErrInvalidOperation
			}
			offset := i * 8
			if offset+8 > len(col.Data) {
				return nil, ErrInvalidOperation
			}
			bits := binary.LittleEndian.Uint64(col.Data[offset:])
			val := int64(bits)
			newVal := val / scalar
			if mask != nil && !mask[i] {
				newVal = val // no change if not masked
			}
			if offset+8 > len(result.Data) {
				return nil, ErrInvalidOperation
			}
			binary.LittleEndian.PutUint64(result.Data[offset:], uint64(newVal))
		}
	default:
		return nil, ErrInvalidColumnType
	}

	return []Column{result}, nil
}

// Power kernel: raises a column to a power
type Power struct {
	Column string
	Power  float64
}

func (k Power) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	col := findColumn(chunk, k.Column)
	if col == nil {
		return nil, ErrColumnNotFound
	}
	if col.Type != Float64 {
		return nil, ErrInvalidColumnType
	}

	result := Column{
		Name: col.Name,
		Type: Float64,
		Data: make([]byte, len(col.Data)),
	}

	for i := 0; i < chunk.Len; i++ {
		if i >= len(col.Data)/8 {
			return nil, ErrInvalidOperation
		}
		offset := i * 8
		if offset+8 > len(col.Data) {
			return nil, ErrInvalidOperation
		}
		bits := binary.LittleEndian.Uint64(col.Data[offset:])
		val := math.Float64frombits(bits)
		newVal := math.Pow(val, k.Power)
		if mask != nil && !mask[i] {
			newVal = val // no change if not masked
		}
		if offset+8 > len(result.Data) {
			return nil, ErrInvalidOperation
		}
		binary.LittleEndian.PutUint64(result.Data[offset:], math.Float64bits(newVal))
	}

	return []Column{result}, nil
}

// Variance kernel: computes variance of a column
type Variance struct {
	Column string
}

func (k Variance) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	col := findColumn(chunk, k.Column)
	if col == nil {
		return nil, ErrColumnNotFound
	}
	if col.Type != Float64 {
		return nil, ErrInvalidColumnType
	}

	// First pass: compute mean
	var sum float64
	count := 0
	for i := 0; i < chunk.Len; i++ {
		if mask == nil || mask[i] {
			if i >= len(col.Data)/8 {
				return nil, ErrInvalidOperation
			}
			offset := i * 8
			if offset+8 > len(col.Data) {
				return nil, ErrInvalidOperation
			}
			bits := binary.LittleEndian.Uint64(col.Data[offset:])
			val := math.Float64frombits(bits)
			sum += val
			count++
		}
	}

	if count == 0 {
		result := Column{
			Name: "var_" + col.Name,
			Type: Float64,
			Data: make([]byte, 8),
		}
		binary.LittleEndian.PutUint64(result.Data, math.Float64bits(0.0))
		return []Column{result}, nil
	}

	mean := sum / float64(count)

	// Second pass: compute variance
	var sumSquares float64
	for i := 0; i < chunk.Len; i++ {
		if mask == nil || mask[i] {
			offset := i * 8
			bits := binary.LittleEndian.Uint64(col.Data[offset:])
			val := math.Float64frombits(bits)
			diff := val - mean
			sumSquares += diff * diff
		}
	}

	variance := sumSquares / float64(count)

	result := Column{
		Name: "var_" + col.Name,
		Type: Float64,
		Data: make([]byte, 8),
	}
	binary.LittleEndian.PutUint64(result.Data, math.Float64bits(variance))
	return []Column{result}, nil
}

// StdDev kernel: computes standard deviation of a column
type StdDev struct {
	Column string
}

func (k StdDev) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	// Get variance first
	varKernel := Variance{Column: k.Column}
	cols, err := varKernel.Execute(chunk, mask)
	if err != nil {
		return nil, err
	}

	if len(cols) != 1 {
		return nil, ErrInvalidOperation
	}

	col := cols[0]
	if len(col.Data) != 8 {
		return nil, ErrInvalidOperation
	}

	bits := binary.LittleEndian.Uint64(col.Data)
	variance := math.Float64frombits(bits)
	stddev := math.Sqrt(variance)

	result := Column{
		Name: "stddev_" + k.Column,
		Type: Float64,
		Data: make([]byte, 8),
	}
	binary.LittleEndian.PutUint64(result.Data, math.Float64bits(stddev))
	return []Column{result}, nil
}

// Median kernel: computes median of a column
type Median struct {
	Column string
}

func (k Median) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	col := findColumn(chunk, k.Column)
	if col == nil {
		return nil, ErrColumnNotFound
	}
	if col.Type != Float64 {
		return nil, ErrInvalidColumnType
	}

	// Collect values
	var values []float64
	for i := 0; i < chunk.Len; i++ {
		if mask == nil || mask[i] {
			if i >= len(col.Data)/8 {
				return nil, ErrInvalidOperation
			}
			offset := i * 8
			if offset+8 > len(col.Data) {
				return nil, ErrInvalidOperation
			}
			bits := binary.LittleEndian.Uint64(col.Data[offset:])
			val := math.Float64frombits(bits)
			values = append(values, val)
		}
	}

	if len(values) == 0 {
		result := Column{
			Name: "median_" + col.Name,
			Type: Float64,
			Data: make([]byte, 8),
		}
		binary.LittleEndian.PutUint64(result.Data, math.Float64bits(0.0))
		return []Column{result}, nil
	}

	// Sort values
	for i := 0; i < len(values)-1; i++ {
		for j := i + 1; j < len(values); j++ {
			if values[i] > values[j] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}

	// Find median
	var median float64
	n := len(values)
	if n%2 == 1 {
		median = values[n/2]
	} else {
		median = (values[n/2-1] + values[n/2]) / 2.0
	}

	result := Column{
		Name: "median_" + col.Name,
		Type: Float64,
		Data: make([]byte, 8),
	}
	binary.LittleEndian.PutUint64(result.Data, math.Float64bits(median))
	return []Column{result}, nil
}

// Count kernel: counts the number of rows
type Count struct {
	Column string // Optional: if specified, count non-null values in this column
}

func (k Count) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	var count int
	if k.Column != "" {
		// Count non-null values in the specified column
		col := findColumn(chunk, k.Column)
		if col == nil {
			return nil, ErrColumnNotFound
		}
		for i := 0; i < chunk.Len; i++ {
			if mask == nil || mask[i] {
				switch col.Type {
				case Float64, Int64:
					count++
				case Bool:
					count++
				case String:
					if i < len(col.Strings) && col.Strings[i] != "" {
						count++
					}
				}
			}
		}
	} else {
		// Count all rows
		for i := 0; i < chunk.Len; i++ {
			if mask == nil || mask[i] {
				count++
			}
		}
	}

	result := Column{
		Name: "count",
		Type: Int64,
		Data: make([]byte, 8),
	}
	binary.LittleEndian.PutUint64(result.Data, uint64(count))
	return []Column{result}, nil
}

// applyArithmeticColumns applies an arithmetic operation between two columns
func applyArithmeticColumns(chunk *Chunk, leftColumn, rightColumn string, mask Mask, op arithmeticOp) ([]Column, error) {
	leftCol := findColumn(chunk, leftColumn)
	if leftCol == nil {
		return nil, ErrColumnNotFound
	}

	rightCol := findColumn(chunk, rightColumn)
	if rightCol == nil {
		return nil, ErrColumnNotFound
	}

	// Validate that both columns are numeric types
	if (leftCol.Type != Float64 && leftCol.Type != Int64) || (rightCol.Type != Float64 && rightCol.Type != Int64) {
		return nil, ErrInvalidColumnType
	}

	// Determine result type: if either column is Float64, result is Float64
	resultType := leftCol.Type
	if rightCol.Type == Float64 {
		resultType = Float64
	}

	result := Column{
		Name: leftColumn + "_" + rightColumn, // e.g., "price_total"
		Type: resultType,
		Data: make([]byte, len(leftCol.Data)),
	}

	for i := 0; i < chunk.Len; i++ {
		if i >= len(leftCol.Data)/8 || i >= len(rightCol.Data)/8 {
			return nil, ErrInvalidOperation
		}
		offset := i * 8
		if offset+8 > len(leftCol.Data) || offset+8 > len(rightCol.Data) {
			return nil, ErrInvalidOperation
		}

		// Get left value
		var leftVal float64
		leftBits := binary.LittleEndian.Uint64(leftCol.Data[offset:])
		if leftCol.Type == Float64 {
			leftVal = math.Float64frombits(leftBits)
		} else {
			leftVal = float64(int64(leftBits))
		}

		// Get right value
		var rightVal float64
		rightBits := binary.LittleEndian.Uint64(rightCol.Data[offset:])
		if rightCol.Type == Float64 {
			rightVal = math.Float64frombits(rightBits)
		} else {
			rightVal = float64(int64(rightBits))
		}

		newVal := op(leftVal, rightVal)
		if mask != nil && !mask[i] {
			newVal = leftVal // no change if not masked
		}

		if offset+8 > len(result.Data) {
			return nil, ErrInvalidOperation
		}

		if resultType == Float64 {
			binary.LittleEndian.PutUint64(result.Data[offset:], math.Float64bits(newVal))
		} else {
			binary.LittleEndian.PutUint64(result.Data[offset:], uint64(int64(newVal)))
		}
	}

	return []Column{result}, nil
}

// AddColumns kernel: adds two columns element-wise
type AddColumns struct {
	Left  string // left column name
	Right string // right column name
}

func (k AddColumns) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	return applyArithmeticColumns(chunk, k.Left, k.Right, mask, func(a, b float64) float64 { return a + b })
}

// SubtractColumns kernel: subtracts right column from left column element-wise
type SubtractColumns struct {
	Left  string // left column name
	Right string // right column name
}

func (k SubtractColumns) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	return applyArithmeticColumns(chunk, k.Left, k.Right, mask, func(a, b float64) float64 { return a - b })
}

// MultiplyColumns kernel: multiplies two columns element-wise
type MultiplyColumns struct {
	Left  string // left column name
	Right string // right column name
}

func (k MultiplyColumns) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	return applyArithmeticColumns(chunk, k.Left, k.Right, mask, func(a, b float64) float64 { return a * b })
}

// DivideColumns kernel: divides left column by right column element-wise
type DivideColumns struct {
	Left  string // left column name
	Right string // right column name
}

func (k DivideColumns) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	leftCol := findColumn(chunk, k.Left)
	if leftCol == nil {
		return nil, ErrColumnNotFound
	}

	rightCol := findColumn(chunk, k.Right)
	if rightCol == nil {
		return nil, ErrColumnNotFound
	}

	// Validate that both columns are numeric types
	if (leftCol.Type != Float64 && leftCol.Type != Int64) || (rightCol.Type != Float64 && rightCol.Type != Int64) {
		return nil, ErrInvalidColumnType
	}

	// Determine result type: if either column is Float64, result is Float64
	resultType := leftCol.Type
	if rightCol.Type == Float64 {
		resultType = Float64
	}

	result := Column{
		Name: k.Left + "_div_" + k.Right,
		Type: resultType,
		Data: make([]byte, len(leftCol.Data)),
	}

	for i := 0; i < chunk.Len; i++ {
		if i >= len(leftCol.Data)/8 || i >= len(rightCol.Data)/8 {
			return nil, ErrInvalidOperation
		}
		offset := i * 8
		if offset+8 > len(leftCol.Data) || offset+8 > len(rightCol.Data) {
			return nil, ErrInvalidOperation
		}

		// Get left value
		var leftVal float64
		leftBits := binary.LittleEndian.Uint64(leftCol.Data[offset:])
		if leftCol.Type == Float64 {
			leftVal = math.Float64frombits(leftBits)
		} else {
			leftVal = float64(int64(leftBits))
		}

		// Get right value
		var rightVal float64
		rightBits := binary.LittleEndian.Uint64(rightCol.Data[offset:])
		if rightCol.Type == Float64 {
			rightVal = math.Float64frombits(rightBits)
		} else {
			rightVal = float64(int64(rightBits))
		}

		// Check for division by zero
		if rightVal == 0 {
			return nil, ErrInvalidOperation
		}

		newVal := leftVal / rightVal
		if mask != nil && !mask[i] {
			newVal = leftVal // no change if not masked
		}

		if offset+8 > len(result.Data) {
			return nil, ErrInvalidOperation
		}

		if resultType == Float64 {
			binary.LittleEndian.PutUint64(result.Data[offset:], math.Float64bits(newVal))
		} else {
			binary.LittleEndian.PutUint64(result.Data[offset:], uint64(int64(newVal)))
		}
	}

	return []Column{result}, nil
}

// PowerColumns kernel: raises left column to the power of right column element-wise
type PowerColumns struct {
	Left  string // base column name
	Right string // exponent column name
}

func (k PowerColumns) Execute(chunk *Chunk, mask Mask) ([]Column, error) {
	leftCol := findColumn(chunk, k.Left)
	if leftCol == nil {
		return nil, ErrColumnNotFound
	}

	rightCol := findColumn(chunk, k.Right)
	if rightCol == nil {
		return nil, ErrColumnNotFound
	}

	// Validate that both columns are numeric types
	if (leftCol.Type != Float64 && leftCol.Type != Int64) || (rightCol.Type != Float64 && rightCol.Type != Int64) {
		return nil, ErrInvalidColumnType
	}

	// Power operations always result in Float64
	result := Column{
		Name: k.Left + "_pow_" + k.Right,
		Type: Float64,
		Data: make([]byte, len(leftCol.Data)),
	}

	for i := 0; i < chunk.Len; i++ {
		if i >= len(leftCol.Data)/8 || i >= len(rightCol.Data)/8 {
			return nil, ErrInvalidOperation
		}
		offset := i * 8
		if offset+8 > len(leftCol.Data) || offset+8 > len(rightCol.Data) {
			return nil, ErrInvalidOperation
		}

		// Get left value
		var leftVal float64
		leftBits := binary.LittleEndian.Uint64(leftCol.Data[offset:])
		if leftCol.Type == Float64 {
			leftVal = math.Float64frombits(leftBits)
		} else {
			leftVal = float64(int64(leftBits))
		}

		// Get right value
		var rightVal float64
		rightBits := binary.LittleEndian.Uint64(rightCol.Data[offset:])
		if rightCol.Type == Float64 {
			rightVal = math.Float64frombits(rightBits)
		} else {
			rightVal = float64(int64(rightBits))
		}

		newVal := math.Pow(leftVal, rightVal)
		if mask != nil && !mask[i] {
			newVal = leftVal // no change if not masked
		}
		if offset+8 > len(result.Data) {
			return nil, ErrInvalidOperation
		}
		binary.LittleEndian.PutUint64(result.Data[offset:], math.Float64bits(newVal))
	}

	return []Column{result}, nil
}
