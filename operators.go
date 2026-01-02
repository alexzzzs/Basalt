package basalt

import (
	"encoding/binary"
	"math"
	"sort"
)

// Filter operator
type Filter struct {
	Predicate Predicate
}

func (op Filter) Apply(chunk *Chunk) (interface{}, error) {
	if op.Predicate == nil {
		return nil, ErrPredicateNil
	}
	return op.Predicate.Evaluate(chunk)
}

// Map operator
type Map struct {
	Kernel Kernel
}

func (op Map) Apply(chunk *Chunk) (interface{}, error) {
	cols, err := op.Kernel.Execute(chunk, nil)
	if err != nil {
		return nil, err
	}
	return &Chunk{
		Columns: cols,
		Len:     chunk.Len,
	}, nil
}

// MapIf operator
type MapIf struct {
	Predicate Predicate
	Kernel    Kernel
}

func (op MapIf) Apply(chunk *Chunk) (interface{}, error) {
	// Evaluate predicate to get mask
	mask, err := op.Predicate.Evaluate(chunk)
	if err != nil {
		return nil, err
	}
	// Apply kernel with mask
	newCols, err := op.Kernel.Execute(chunk, mask)
	if err != nil {
		return nil, err
	}
	result := &Chunk{
		Columns: newCols,
		Len:     chunk.Len,
	}
	return result, nil
}

// Reduce operator
type Reduce struct {
	Kernel Kernel
	Mask   Mask
}

func (op Reduce) Apply(chunk *Chunk) (interface{}, error) {
	// Assume kernel returns a column with one value
	cols, err := op.Kernel.Execute(chunk, op.Mask)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return 0.0, nil
	}
	col := cols[0]
	if len(col.Data) < 8 {
		return 0.0, nil
	}
	bits := binary.LittleEndian.Uint64(col.Data)
	return math.Float64frombits(bits), nil
}

// ReduceIf operator
type ReduceIf struct {
	Mask   Mask
	Kernel Kernel
}

func (op ReduceIf) Apply(chunk *Chunk) (interface{}, error) {
	cols, err := op.Kernel.Execute(chunk, op.Mask)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return 0.0, nil
	}
	col := cols[0]
	if len(col.Data) < 8 {
		return 0.0, nil
	}
	bits := binary.LittleEndian.Uint64(col.Data)
	return math.Float64frombits(bits), nil
}

/*
// Branch operator - commented out due to error handling complexity
type Branch struct {
	Mask       Mask
	ThenPlan   *Plan
	ElsePlan   *Plan
}

func (op Branch) Apply(chunk *Chunk) (interface{}, error) {
	// Execute both branches in parallel if possible
	// For now, sequential
	thenChunks := make([]*Chunk, 0)
	elseChunks := make([]*Chunk, 0)

	for i, masked := range op.Mask {
		if masked {
			// Create single-row chunk for then
			thenChunk := singleRowChunk(chunk, i)
			thenChunks = append(thenChunks, thenChunk)
		} else {
			elseChunk := singleRowChunk(chunk, i)
			elseChunks = append(elseChunks, elseChunk)
		}
	}

	// Execute plans on respective chunks
	thenResults := make([]interface{}, 0)
	for _, ch := range thenChunks {
		result, err := op.ThenPlan.Execute(ch)
		if err != nil {
			return nil, err
		}
		thenResults = append(thenResults, result)
	}

	elseResults := make([]interface{}, 0)
	for _, ch := range elseChunks {
		result, err := op.ElsePlan.Execute(ch)
		if err != nil {
			return nil, err
		}
		elseResults = append(elseResults, result)
	}

	return map[string][]interface{}{
		"then": thenResults,
		"else": elseResults,
	}, nil
}
*/

func singleRowChunk(chunk *Chunk, row int) *Chunk {
	newChunk := &Chunk{
		Columns: make([]Column, len(chunk.Columns)),
		Len:     1,
	}
	for i, col := range chunk.Columns {
		size := getTypeSize(col.Type)
		newChunk.Columns[i] = Column{
			Name: col.Name,
			Type: col.Type,
			Data: make([]byte, size),
		}
		copy(newChunk.Columns[i].Data, col.Data[row*size:(row+1)*size])
	}
	return newChunk
}

// Sort operator: sorts rows by a column
type Sort struct {
	Column    string
	Ascending bool // true for ascending, false for descending
}

func (op Sort) Apply(chunk *Chunk) (interface{}, error) {
	col := findColumn(chunk, op.Column)
	if col == nil {
		return nil, ErrColumnNotFound
	}

	// Validate that column type supports sorting
	if col.Type != Float64 && col.Type != Int64 {
		return nil, ErrUnsupportedType
	}

	// Create indices for sorting
	indices := make([]int, chunk.Len)
	for i := range indices {
		indices[i] = i
	}

	// Sort indices based on column values using efficient sorting
	switch col.Type {
	case Float64:
		sort.Slice(indices, func(i, j int) bool {
			offsetI := indices[i] * 8
			offsetJ := indices[j] * 8
			valI := math.Float64frombits(binary.LittleEndian.Uint64(col.Data[offsetI:]))
			valJ := math.Float64frombits(binary.LittleEndian.Uint64(col.Data[offsetJ:]))
			if op.Ascending {
				return valI < valJ
			}
			return valI > valJ
		})
	case Int64:
		sort.Slice(indices, func(i, j int) bool {
			offsetI := indices[i] * 8
			offsetJ := indices[j] * 8
			valI := int64(binary.LittleEndian.Uint64(col.Data[offsetI:]))
			valJ := int64(binary.LittleEndian.Uint64(col.Data[offsetJ:]))
			if op.Ascending {
				return valI < valJ
			}
			return valI > valJ
		})
	default:
		return nil, ErrUnsupportedType
	}

	// Create new chunk with sorted data
	result := &Chunk{
		Columns: make([]Column, len(chunk.Columns)),
		Len:     chunk.Len,
	}

	for i, col := range chunk.Columns {
		result.Columns[i] = Column{
			Name: col.Name,
			Type: col.Type,
			Data: make([]byte, len(col.Data)),
		}
		if col.Type == String {
			result.Columns[i].Strings = make([]string, len(col.Strings))
		}

		size := getTypeSize(col.Type)
		for j, idx := range indices {
			offsetSrc := idx * size
			offsetDst := j * size
			copy(result.Columns[i].Data[offsetDst:], col.Data[offsetSrc:offsetSrc+size])
			if col.Type == String {
				result.Columns[i].Strings[j] = col.Strings[idx]
			}
		}
	}

	return result, nil
}
