package rag

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/tildaslashalef/mindnest/internal/loggy"
)

// VecOps provides optimized vector operations using the SQLite-vec extension
type VecOps struct {
	db     *sql.DB
	logger *loggy.Logger
	// Cache extension status to avoid repeated checks
	extensionLoaded bool
}

// NewVecOps creates a new vector operations wrapper
func NewVecOps(db *sql.DB, logger *loggy.Logger) *VecOps {
	return &VecOps{
		db:     db,
		logger: logger,
	}
}

// IsExtensionLoaded checks if the SQLite-vec extension is available
func (v *VecOps) IsExtensionLoaded(ctx context.Context) (string, error) {
	var version string
	err := v.db.QueryRowContext(ctx, "SELECT vec_version()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("SQLite-vec extension not loaded: %w", err)
	}

	v.extensionLoaded = true
	return version, nil
}

// EnsureExtensionLoaded ensures the extension is loaded before operations
func (v *VecOps) EnsureExtensionLoaded(ctx context.Context) error {
	if v.extensionLoaded {
		return nil
	}

	_, err := v.IsExtensionLoaded(ctx)
	return err
}

// SerializeVector converts a float32 vector to binary blob representation
func (v *VecOps) SerializeVector(vec []float32, vType VectorType) ([]byte, error) {
	if len(vec) == 0 {
		return nil, ErrInvalidVector
	}

	switch vType {
	case VectorTypeFloat32:
		return sqlite_vec.SerializeFloat32(vec)
	case VectorTypeInt8:
		return v.serializeInt8(vec)
	case VectorTypeBit:
		return v.serializeBit(vec)
	default:
		return nil, fmt.Errorf("unsupported vector type: %s", vType)
	}
}

// DeserializeVector converts a binary blob back to a float32 vector
func (v *VecOps) DeserializeVector(blob []byte, vType VectorType) ([]float32, error) {
	if len(blob) == 0 {
		return nil, ErrInvalidVector
	}

	switch vType {
	case VectorTypeFloat32:
		return v.deserializeFloat32(blob)
	case VectorTypeInt8:
		return v.deserializeInt8(blob)
	case VectorTypeBit:
		return v.deserializeBit(blob)
	default:
		return nil, fmt.Errorf("unsupported vector type: %s", vType)
	}
}

// NormalizeVector normalizes a vector to unit length (L2 norm)
func (v *VecOps) NormalizeVector(ctx context.Context, vec []float32) ([]float32, error) {
	if err := v.EnsureExtensionLoaded(ctx); err != nil {
		return nil, err
	}

	serialized, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		return nil, fmt.Errorf("serializing vector: %w", err)
	}

	var normalized []byte
	err = v.db.QueryRowContext(ctx, "SELECT vec_normalize(?)", serialized).Scan(&normalized)
	if err != nil {
		return nil, fmt.Errorf("normalizing vector: %w", err)
	}

	return v.deserializeFloat32(normalized)
}

// QuantizeBinary converts a float32 vector to a binary vector (1 bit per value)
// This provides significant storage savings (32x smaller than float32)
func (v *VecOps) QuantizeBinary(ctx context.Context, vec []float32) ([]byte, error) {
	if err := v.EnsureExtensionLoaded(ctx); err != nil {
		return nil, err
	}

	serialized, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		return nil, fmt.Errorf("serializing vector: %w", err)
	}

	var binary []byte
	err = v.db.QueryRowContext(ctx, "SELECT vec_quantize_binary(?)", serialized).Scan(&binary)
	if err != nil {
		return nil, fmt.Errorf("binary quantization: %w", err)
	}

	return binary, nil
}

// QuantizeInt8 converts a float32 vector to an int8 vector
// This provides good storage savings (4x smaller than float32)
// while preserving reasonable accuracy
func (v *VecOps) QuantizeInt8(ctx context.Context, vec []float32, min, max float32) ([]byte, error) {
	if err := v.EnsureExtensionLoaded(ctx); err != nil {
		return nil, err
	}

	serialized, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		return nil, fmt.Errorf("serializing vector: %w", err)
	}

	var int8Blob []byte
	var query string
	var args []interface{}

	if min != 0 || max != 0 {
		query = "SELECT vec_quantize_i8(?, ?, ?)"
		args = []interface{}{serialized, min, max}
	} else {
		query = "SELECT vec_quantize_i8(?)"
		args = []interface{}{serialized}
	}

	err = v.db.QueryRowContext(ctx, query, args...).Scan(&int8Blob)
	if err != nil {
		return nil, fmt.Errorf("int8 quantization: %w", err)
	}

	return int8Blob, nil
}

// SliceVector extracts a subset of a vector from start to end
// Useful for Matryoshka embeddings or adaptive dimensionality
func (v *VecOps) SliceVector(ctx context.Context, vec []float32, start, end int) ([]float32, error) {
	if err := v.EnsureExtensionLoaded(ctx); err != nil {
		return nil, err
	}

	if start < 0 || end > len(vec) || start >= end {
		return nil, fmt.Errorf("invalid slice range: start=%d, end=%d, len=%d", start, end, len(vec))
	}

	serialized, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		return nil, fmt.Errorf("serializing vector: %w", err)
	}

	var sliced []byte
	err = v.db.QueryRowContext(ctx, "SELECT vec_slice(?, ?, ?)", serialized, start, end).Scan(&sliced)
	if err != nil {
		return nil, fmt.Errorf("slicing vector: %w", err)
	}

	return v.deserializeFloat32(sliced)
}

// CalculateDistance computes the distance between two vectors using the specified metric
func (v *VecOps) CalculateDistance(
	ctx context.Context,
	vecA, vecB []float32,
	metric DistanceMetric,
) (float64, error) {
	if err := v.EnsureExtensionLoaded(ctx); err != nil {
		return 0, err
	}

	if len(vecA) != len(vecB) {
		return 0, fmt.Errorf("vector length mismatch: %d vs %d", len(vecA), len(vecB))
	}

	serializedA, err := sqlite_vec.SerializeFloat32(vecA)
	if err != nil {
		return 0, fmt.Errorf("serializing vector A: %w", err)
	}

	serializedB, err := sqlite_vec.SerializeFloat32(vecB)
	if err != nil {
		return 0, fmt.Errorf("serializing vector B: %w", err)
	}

	var query string
	var distance float64

	switch metric {
	case DistanceMetricCosine:
		query = "SELECT vec_distance_cosine(?, ?)"
	case DistanceMetricL2:
		query = "SELECT vec_distance_L2(?, ?)"
	case DistanceMetricHamming:
		// For Hamming distance, we need to quantize to bitvectors first
		serializedA, err = v.QuantizeBinary(ctx, vecA)
		if err != nil {
			return 0, fmt.Errorf("quantizing vector A: %w", err)
		}
		serializedB, err = v.QuantizeBinary(ctx, vecB)
		if err != nil {
			return 0, fmt.Errorf("quantizing vector B: %w", err)
		}
		query = "SELECT vec_distance_hamming(?, ?)"
	default:
		return 0, fmt.Errorf("unsupported distance metric: %s", metric)
	}

	err = v.db.QueryRowContext(ctx, query, serializedA, serializedB).Scan(&distance)
	if err != nil {
		return 0, fmt.Errorf("calculating %s distance: %w", metric, err)
	}

	return distance, nil
}

// DistanceToSimilarity converts a distance score to a similarity score in [0,1]
// where 1 means identical and 0 means completely different
func (v *VecOps) DistanceToSimilarity(distance float64, metric DistanceMetric) float64 {
	switch metric {
	case DistanceMetricCosine:
		// Cosine distance from sqlite-vec: 0 = identical, 2 = opposite
		// Convert to similarity: 1 - (distance/2)
		similarity := 1.0 - (distance / 2.0)
		if similarity < 0 {
			similarity = 0
		}
		return similarity
	case DistanceMetricL2:
		// L2 distance: 0 = identical, unbounded maximum
		// Convert to similarity using a falloff function
		return 1.0 / (1.0 + distance)
	case DistanceMetricHamming:
		// Hamming distance: 0 = identical, max = vector length
		// Use inverse proportional similarity
		return 1.0 / (1.0 + (distance / 10.0))
	default:
		// Default fallback
		return 1.0 - math.Min(1.0, distance)
	}
}

// FindKNN performs a k-nearest neighbor search using the vector index
func (v *VecOps) FindKNN(
	ctx context.Context,
	vec []float32,
	k int,
	additionalWhere string,
	args ...interface{},
) ([]int64, []float64, error) {
	if err := v.EnsureExtensionLoaded(ctx); err != nil {
		return nil, nil, err
	}

	if k <= 0 {
		return nil, nil, fmt.Errorf("k must be positive: %d", k)
	}

	serialized, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		return nil, nil, fmt.Errorf("serializing vector: %w", err)
	}

	// Build the query with the vector match predicate
	query := `
		SELECT rowid, distance
		FROM vectors
		WHERE embedding MATCH ?
		AND k = ?
	`

	// Add additional WHERE clauses if provided
	if additionalWhere != "" {
		query += " AND " + additionalWhere
	}

	query += " ORDER BY distance"

	// Prepare arguments
	queryArgs := make([]interface{}, 0, 2+len(args))
	queryArgs = append(queryArgs, serialized, k)
	queryArgs = append(queryArgs, args...)

	// Execute the query
	rows, err := v.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("executing KNN query: %w", err)
	}
	defer rows.Close()

	// Collect results
	var ids []int64
	var distances []float64

	for rows.Next() {
		var id int64
		var distance float64
		if err := rows.Scan(&id, &distance); err != nil {
			return nil, nil, fmt.Errorf("scanning KNN result: %w", err)
		}
		ids = append(ids, id)
		distances = append(distances, distance)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterating KNN results: %w", err)
	}

	return ids, distances, nil
}

// VectorToJSON converts a vector to JSON representation for debugging
func (v *VecOps) VectorToJSON(ctx context.Context, vec []float32) (string, error) {
	if err := v.EnsureExtensionLoaded(ctx); err != nil {
		return "", err
	}

	serialized, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		return "", fmt.Errorf("serializing vector: %w", err)
	}

	var jsonText string
	err = v.db.QueryRowContext(ctx, "SELECT vec_to_json(?)", serialized).Scan(&jsonText)
	if err != nil {
		return "", fmt.Errorf("converting vector to JSON: %w", err)
	}

	return jsonText, nil
}

// GetVectorDimensions returns the number of dimensions in a vector
func (v *VecOps) GetVectorDimensions(ctx context.Context, blob []byte) (int, error) {
	if err := v.EnsureExtensionLoaded(ctx); err != nil {
		return 0, err
	}

	var dimensions int
	err := v.db.QueryRowContext(ctx, "SELECT vec_length(?)", blob).Scan(&dimensions)
	if err != nil {
		return 0, fmt.Errorf("getting vector dimensions: %w", err)
	}

	return dimensions, nil
}

// GetVectorType determines the type of a vector blob
func (v *VecOps) GetVectorType(ctx context.Context, blob []byte) (VectorType, error) {
	if err := v.EnsureExtensionLoaded(ctx); err != nil {
		return "", err
	}

	var typeStr string
	err := v.db.QueryRowContext(ctx, "SELECT vec_type(?)", blob).Scan(&typeStr)
	if err != nil {
		return "", fmt.Errorf("getting vector type: %w", err)
	}

	return VectorType(typeStr), nil
}

// Helper functions for serialization and deserialization

func (v *VecOps) serializeInt8(vec []float32) ([]byte, error) {
	// Convert float32 to int8, clamping values
	blob := make([]byte, len(vec))
	for i, val := range vec {
		// Clamp to -128..127 range
		iv := int8(math.Max(-128, math.Min(127, float64(val))))
		blob[i] = byte(iv)
	}
	return blob, nil
}

func (v *VecOps) serializeBit(vec []float32) ([]byte, error) {
	// Each byte stores 8 bits
	blobSize := (len(vec) + 7) / 8
	blob := make([]byte, blobSize)

	for i, val := range vec {
		if val > 0 {
			byteIdx := i / 8
			bitIdx := i % 8
			blob[byteIdx] |= 1 << bitIdx
		}
	}
	return blob, nil
}

func (v *VecOps) deserializeFloat32(blob []byte) ([]float32, error) {
	if len(blob)%4 != 0 {
		return nil, fmt.Errorf("invalid float32 blob size: %d bytes", len(blob))
	}

	vec := make([]float32, len(blob)/4)
	for i := 0; i < len(vec); i++ {
		bits := binary.LittleEndian.Uint32(blob[i*4 : (i+1)*4])
		vec[i] = math.Float32frombits(bits)
	}
	return vec, nil
}

func (v *VecOps) deserializeInt8(blob []byte) ([]float32, error) {
	vec := make([]float32, len(blob))
	for i, b := range blob {
		vec[i] = float32(int8(b))
	}
	return vec, nil
}

func (v *VecOps) deserializeBit(blob []byte) ([]float32, error) {
	// Each byte contains 8 bits
	vec := make([]float32, len(blob)*8)
	for byteIdx, b := range blob {
		for bitIdx := 0; bitIdx < 8; bitIdx++ {
			i := byteIdx*8 + bitIdx
			if i < len(vec) {
				if (b & (1 << bitIdx)) != 0 {
					vec[i] = 1.0
				} else {
					vec[i] = 0.0
				}
			}
		}
	}
	return vec, nil
}

// VerifyVectorExtensionCapabilities checks which capabilities are supported by the loaded sqlite-vec extension
func (v *VecOps) VerifyVectorExtensionCapabilities(ctx context.Context) (map[string]bool, error) {
	if err := v.EnsureExtensionLoaded(ctx); err != nil {
		return nil, err
	}

	capabilities := make(map[string]bool)

	// Check for basic vec0 virtual table support
	var count int
	err := v.db.QueryRowContext(ctx, `
		SELECT count(*) FROM sqlite_master 
		WHERE type='table' AND name='vectors' AND sql LIKE '%USING vec0%'
	`).Scan(&count)

	if err != nil {
		return nil, fmt.Errorf("checking vec0 support: %w", err)
	}
	capabilities["vec0"] = count > 0

	// Try to check supported distance metrics
	metrics := []struct {
		name string
		key  string
	}{
		{"l2", "l2_distance"},
		{"cosine", "cosine_distance"},
		{"dot", "dot_product"},
		{"hamming", "hamming_distance"},
	}

	for _, metric := range metrics {
		var result float64
		err := v.db.QueryRowContext(ctx, fmt.Sprintf(`
			WITH temp(v) AS (SELECT vec_random(4))
			SELECT CASE WHEN EXISTS(
				SELECT 1 FROM temp WHERE vec_distance_%s(v, v) IS NOT NULL
			) THEN 1 ELSE 0 END
		`, metric.name)).Scan(&result)

		if err == nil && result > 0 {
			capabilities[metric.key] = true
		} else {
			capabilities[metric.key] = false
		}
	}

	// Check for function existence
	functions := []string{
		"vec_normalize",
		"vec_quantize_i8",
		"vec_quantize_binary",
		"vec_slice",
		"vec_to_json",
		"vec_length",
		"vec_type",
	}

	for _, fn := range functions {
		var result int
		err := v.db.QueryRowContext(ctx, `
			SELECT count(*) FROM pragma_function_list
			WHERE name = ?
		`, fn).Scan(&result)

		if err == nil && result > 0 {
			capabilities[fn] = true
		} else {
			capabilities[fn] = false
		}
	}

	v.logger.Info("Vector extension capabilities verified",
		"vec0", capabilities["vec0"],
		"l2_distance", capabilities["l2_distance"],
		"cosine_distance", capabilities["cosine_distance"],
		"functions_available", len(functions))

	return capabilities, nil
}
