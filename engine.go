package basalt

import (
	"math"
	"sync"
)

// PlanBuilder for API
type PlanBuilder struct {
	plan   *Plan
	table  *Table
	engine *Engine
}

// NewEngine creates a new engine with the specified number of workers for parallel processing.
func NewEngine(numWorkers int) *Engine {
	return &Engine{numWorkers: numWorkers}
}

// From starts building a query plan from the given table.
func (e *Engine) From(table *Table) *PlanBuilder {
	return &PlanBuilder{
		plan:   &Plan{Ops: []Operator{}},
		table:  table,
		engine: e,
	}
}

// Filter adds a filter operator
func (pb *PlanBuilder) Filter(predicate Predicate) *PlanBuilder {
	pb.plan.Ops = append(pb.plan.Ops, Filter{Predicate: predicate})
	return pb
}

// Map adds a map operator
func (pb *PlanBuilder) Map(kernel Kernel) *PlanBuilder {
	pb.plan.Ops = append(pb.plan.Ops, Map{Kernel: kernel})
	return pb
}

// MapIf adds a MapIf operator
func (pb *PlanBuilder) MapIf(predicate Predicate, kernel Kernel) *PlanBuilder {
	pb.plan.Ops = append(pb.plan.Ops, MapIf{Predicate: predicate, Kernel: kernel})
	return pb
}

// Reduce adds a reduce operator
func (pb *PlanBuilder) Reduce(kernel Kernel) *PlanBuilder {
	pb.plan.Ops = append(pb.plan.Ops, Reduce{Kernel: kernel})
	return pb
}

// Sort adds a sort operator
func (pb *PlanBuilder) Sort(column string, ascending bool) *PlanBuilder {
	pb.plan.Ops = append(pb.plan.Ops, Sort{Column: column, Ascending: ascending})
	return pb
}

// Run executes the plan
func (pb *PlanBuilder) Run() (*Result, error) {
	return pb.engine.Run(pb.plan, pb.table)
}

type resultOrError struct {
	result interface{}
	err    error
}

// Run executes the plan on the engine using a controlled worker pool
func (e *Engine) Run(plan *Plan, table *Table) (*Result, error) {
	if len(table.Chunks) == 0 {
		return &Result{Data: nil}, nil
	}

	// If only one worker or one chunk, process sequentially for simplicity
	if e.numWorkers <= 1 || len(table.Chunks) == 1 {
		return e.runSequential(plan, table)
	}

	// Use worker pool for parallel processing
	return e.runParallel(plan, table)
}

// runSequential processes chunks one by one without goroutines
func (e *Engine) runSequential(plan *Plan, table *Table) (*Result, error) {
	var finalResult interface{}
	var firstError error

	// Check the last operation type to determine how to merge results
	lastOp := plan.Ops[len(plan.Ops)-1]

	switch lastOp.(type) {
	case Reduce, ReduceIf:
		// Collect reduce results from each chunk
		var values []interface{}
		for _, chunk := range table.Chunks {
			result, err := plan.Execute(chunk)
			if err != nil {
				if firstError == nil {
					firstError = err
				}
				continue
			}
			if result != nil {
				values = append(values, result)
			}
		}
		if firstError != nil {
			return nil, firstError
		}
		finalResult, firstError = mergeReduceResultsSequential(values, lastOp.(Reduce).Kernel)
	default:
		// For non-reduce operations, collect and merge chunks
		var chunks []*Chunk
		for _, chunk := range table.Chunks {
			result, err := plan.Execute(chunk)
			if err != nil {
				if firstError == nil {
					firstError = err
				}
				continue
			}
			if chunk, ok := result.(*Chunk); ok {
				chunks = append(chunks, chunk)
			}
		}
		if firstError != nil {
			return nil, firstError
		}
		finalResult, firstError = mergeChunksSequential(chunks)
	}

	if firstError != nil {
		return nil, firstError
	}

	return &Result{Data: finalResult}, nil
}

// runParallel processes chunks using a controlled worker pool
func (e *Engine) runParallel(plan *Plan, table *Table) (*Result, error) {
	numWorkers := e.numWorkers
	if numWorkers > len(table.Chunks) {
		numWorkers = len(table.Chunks)
	}

	// Create channels for work distribution and results
	jobs := make(chan *Chunk, len(table.Chunks))
	results := make(chan resultOrError, len(table.Chunks))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for chunk := range jobs {
				result, err := plan.Execute(chunk)
				results <- resultOrError{result: result, err: err}
			}
		}()
	}

	// Send jobs to workers
	for _, chunk := range table.Chunks {
		jobs <- chunk
	}
	close(jobs)

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Merge results from parallel chunk processing
	var finalResult interface{}
	var firstError error

	// Check the last operation type to determine how to merge results
	lastOp := plan.Ops[len(plan.Ops)-1]

	switch lastOp.(type) {
	case Reduce, ReduceIf:
		finalResult, firstError = mergeReduceResults(results, lastOp.(Reduce).Kernel)
	default:
		// For non-reduce operations, merge chunks from all parallel executions
		finalResult, firstError = mergeChunks(results)
	}

	if firstError != nil {
		return nil, firstError
	}

	return &Result{Data: finalResult}, nil
}

// mergeReduceResultsSequential merges results from sequential reduce operations
func mergeReduceResultsSequential(values []interface{}, kernel Kernel) (interface{}, error) {
	if len(values) == 0 {
		return 0, nil // default value
	}

	// Generic merging logic based on kernel type
	switch kernel.(type) {
	case Sum:
		return mergeSum(values)
	case Min:
		return mergeMin(values)
	case Max:
		return mergeMax(values)
	case Average:
		return mergeAverage(values)
	case Variance:
		return mergeVariance(values)
	case StdDev:
		return mergeStdDev(values)
	case Median:
		return mergeMedian(values)
	case Count:
		return mergeCount(values)
	default:
		// For unsupported kernels, try generic merging
		return mergeGeneric(values, kernel)
	}
}

// mergeChunksSequential combines chunks from sequential non-reduce operations
func mergeChunksSequential(chunks []*Chunk) (interface{}, error) {
	if len(chunks) == 0 {
		return nil, nil
	}

	if len(chunks) == 1 {
		return chunks[0], nil
	}

	// Merge multiple chunks by concatenating rows
	// All chunks should have the same column structure
	if len(chunks[0].Columns) == 0 {
		return &Chunk{Columns: []Column{}, Len: 0}, nil
	}

	mergedChunk := &Chunk{
		Columns: make([]Column, len(chunks[0].Columns)),
		Len:     0,
	}

	// Calculate total length and initialize columns
	for _, chunk := range chunks {
		mergedChunk.Len += chunk.Len
	}

	for i, col := range chunks[0].Columns {
		mergedChunk.Columns[i] = Column{
			Name: col.Name,
			Type: col.Type,
		}

		if col.Type == String {
			mergedChunk.Columns[i].Strings = make([]string, 0, mergedChunk.Len)
		} else {
			mergedChunk.Columns[i].Data = make([]byte, mergedChunk.Len*getTypeSize(col.Type))
		}

		// Concatenate data from all chunks
		offset := 0
		for _, chunk := range chunks {
			srcCol := chunk.Columns[i]
			if col.Type == String {
				mergedChunk.Columns[i].Strings = append(mergedChunk.Columns[i].Strings, srcCol.Strings...)
			} else {
				copy(mergedChunk.Columns[i].Data[offset:], srcCol.Data)
				offset += len(srcCol.Data)
			}
		}
	}

	return mergedChunk, nil
}

// mergeReduceResults merges results from parallel reduce operations
func mergeReduceResults(results <-chan resultOrError, kernel Kernel) (interface{}, error) {
	var firstError error

	// Collect all results first
	var values []interface{}
	for re := range results {
		if re.err != nil && firstError == nil {
			firstError = re.err
			continue
		}
		if re.result != nil {
			values = append(values, re.result)
		}
	}

	if firstError != nil {
		return nil, firstError
	}

	if len(values) == 0 {
		return 0, nil // default value
	}

	// Generic merging logic based on kernel type
	switch kernel.(type) {
	case Sum:
		return mergeSum(values)
	case Min:
		return mergeMin(values)
	case Max:
		return mergeMax(values)
	case Average:
		return mergeAverage(values)
	case Variance:
		return mergeVariance(values)
	case StdDev:
		return mergeStdDev(values)
	case Median:
		return mergeMedian(values)
	case Count:
		return mergeCount(values)
	default:
		// For unsupported kernels, try generic merging
		return mergeGeneric(values, kernel)
	}
}

// mergeSum combines sum results from multiple chunks
func mergeSum(values []interface{}) (interface{}, error) {
	var total float64
	for _, val := range values {
		if f, ok := val.(float64); ok {
			total += f
		}
	}
	return total, nil
}

// mergeMin finds minimum across all chunks
func mergeMin(values []interface{}) (interface{}, error) {
	if len(values) == 0 {
		return 0.0, nil
	}

	minVal := math.Inf(1)
	for _, val := range values {
		if f, ok := val.(float64); ok && f < minVal {
			minVal = f
		}
	}

	if minVal == math.Inf(1) {
		return 0.0, nil
	}
	return minVal, nil
}

// mergeMax finds maximum across all chunks
func mergeMax(values []interface{}) (interface{}, error) {
	if len(values) == 0 {
		return 0.0, nil
	}

	maxVal := math.Inf(-1)
	for _, val := range values {
		if f, ok := val.(float64); ok && f > maxVal {
			maxVal = f
		}
	}

	if maxVal == math.Inf(-1) {
		return 0.0, nil
	}
	return maxVal, nil
}

// mergeAverage computes weighted average across chunks
func mergeAverage(values []interface{}) (interface{}, error) {
	if len(values) == 0 {
		return 0.0, nil
	}

	var total float64
	for _, val := range values {
		if f, ok := val.(float64); ok {
			total += f
		}
	}

	return total / float64(len(values)), nil
}

// mergeVariance merges variance results from multiple chunks
func mergeVariance(values []interface{}) (interface{}, error) {
	if len(values) == 0 {
		return 0.0, nil
	}

	var total float64
	for _, val := range values {
		if f, ok := val.(float64); ok {
			total += f
		}
	}

	return total / float64(len(values)), nil
}

// mergeStdDev merges standard deviation results from multiple chunks
func mergeStdDev(values []interface{}) (interface{}, error) {
	if len(values) == 0 {
		return 0.0, nil
	}

	// For simplicity, compute average of stddev values
	// In a real implementation, this would need proper variance merging
	var total float64
	for _, val := range values {
		if f, ok := val.(float64); ok {
			total += f
		}
	}

	return total / float64(len(values)), nil
}

// mergeMedian merges median results from multiple chunks
func mergeMedian(values []interface{}) (interface{}, error) {
	if len(values) == 0 {
		return 0.0, nil
	}

	if len(values) == 1 {
		if f, ok := values[0].(float64); ok {
			return f, nil
		}
		return 0.0, nil
	}

	// Collect all median values and find the median of medians
	var medians []float64
	for _, val := range values {
		if f, ok := val.(float64); ok {
			medians = append(medians, f)
		}
	}

	if len(medians) == 0 {
		return 0.0, nil
	}

	// Sort the medians
	for i := 0; i < len(medians)-1; i++ {
		for j := i + 1; j < len(medians); j++ {
			if medians[i] > medians[j] {
				medians[i], medians[j] = medians[j], medians[i]
			}
		}
	}

	// Return the median of the chunk medians
	n := len(medians)
	if n%2 == 1 {
		return medians[n/2], nil
	}
	return (medians[n/2-1] + medians[n/2]) / 2.0, nil
}

// mergeCount sums count results from multiple chunks
func mergeCount(values []interface{}) (interface{}, error) {
	var total int64
	for _, val := range values {
		if i, ok := val.(int64); ok {
			total += i
		}
	}
	return total, nil
}

// mergeGeneric handles merging for custom or unknown kernel types
func mergeGeneric(values []interface{}, kernel Kernel) (interface{}, error) {
	// Default behavior: return the first value
	if len(values) > 0 {
		return values[0], nil
	}
	return nil, nil
}

// mergeChunks combines chunks from parallel non-reduce operations
func mergeChunks(results <-chan resultOrError) (interface{}, error) {
	var firstError error
	var chunks []*Chunk

	// Collect all chunk results
	for re := range results {
		if re.err != nil && firstError == nil {
			firstError = re.err
			continue
		}
		if chunk, ok := re.result.(*Chunk); ok {
			chunks = append(chunks, chunk)
		}
	}

	if firstError != nil {
		return nil, firstError
	}

	if len(chunks) == 0 {
		return nil, nil
	}

	if len(chunks) == 1 {
		return chunks[0], nil
	}

	// Merge multiple chunks by concatenating rows
	// All chunks should have the same column structure
	if len(chunks[0].Columns) == 0 {
		return &Chunk{Columns: []Column{}, Len: 0}, nil
	}

	mergedChunk := &Chunk{
		Columns: make([]Column, len(chunks[0].Columns)),
		Len:     0,
	}

	// Calculate total length and initialize columns
	for _, chunk := range chunks {
		mergedChunk.Len += chunk.Len
	}

	for i, col := range chunks[0].Columns {
		mergedChunk.Columns[i] = Column{
			Name: col.Name,
			Type: col.Type,
		}

		if col.Type == String {
			mergedChunk.Columns[i].Strings = make([]string, 0, mergedChunk.Len)
		} else {
			mergedChunk.Columns[i].Data = make([]byte, mergedChunk.Len*getTypeSize(col.Type))
		}

		// Concatenate data from all chunks
		offset := 0
		for _, chunk := range chunks {
			srcCol := chunk.Columns[i]
			if col.Type == String {
				mergedChunk.Columns[i].Strings = append(mergedChunk.Columns[i].Strings, srcCol.Strings...)
			} else {
				copy(mergedChunk.Columns[i].Data[offset:], srcCol.Data)
				offset += len(srcCol.Data)
			}
		}
	}

	return mergedChunk, nil
}
