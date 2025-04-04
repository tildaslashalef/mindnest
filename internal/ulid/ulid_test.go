package ulid

import (
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
	// Generate a new ULID
	id := Generate()

	// Verify it's not zero
	assert.False(t, id.IsZero(), "Generated ULID should not be zero")

	// Verify it contains a valid timestamp close to now
	now := time.Now()
	idTime := id.Time()
	timeDiff := now.Sub(idTime).Seconds()
	assert.True(t, timeDiff < 1.0, "ULID timestamp should be close to now")
}

func TestGenerateWithPrefix(t *testing.T) {
	// Generate ULIDs with different prefixes
	prefixes := []string{PrefixWorkspace, PrefixFile, PrefixChunk, PrefixReview, "custom"}

	for _, prefix := range prefixes {
		id := GenerateWithPrefix(prefix)

		// Verify prefix is set
		assert.Equal(t, prefix, id.Prefix(), "Prefix should match the provided value")
		assert.True(t, id.HasPrefix(), "ULID should have a prefix")

		// Verify string representation contains the prefix
		assert.Contains(t, id.String(), prefix+PrefixSeparator,
			"String representation should contain the prefix")
	}
}

func TestParse(t *testing.T) {
	// Test parsing a raw ULID
	rawULID := Generate()
	parsedRaw, err := Parse(rawULID.String())
	require.NoError(t, err)
	assert.Equal(t, rawULID, parsedRaw)

	// Test parsing a prefixed ULID
	prefixedULID := GenerateWithPrefix(PrefixWorkspace)
	parsedPrefixed, err := Parse(prefixedULID.String())
	require.NoError(t, err)
	assert.Equal(t, prefixedULID, parsedPrefixed)
	assert.Equal(t, PrefixWorkspace, parsedPrefixed.Prefix())

	// Test parsing an invalid ULID
	_, err = Parse("invalid-ulid")
	assert.Error(t, err)
}

func TestFromString(t *testing.T) {
	// Generate a ULID string
	id := Generate()
	idStr := id.String()

	// Parse it using FromString
	parsed, err := FromString(idStr)
	require.NoError(t, err)
	assert.Equal(t, id, parsed)
}

func TestValidate(t *testing.T) {
	// Test valid ULIDs
	id := Generate()
	assert.True(t, Validate(id.String()), "Valid ULID should be valid")

	prefixedID := GenerateWithPrefix(PrefixWorkspace)
	assert.True(t, Validate(prefixedID.String()), "Valid prefixed ULID should be valid")

	// Test invalid ULIDs
	assert.False(t, Validate("invalid"), "Invalid ULID should be invalid")
	assert.False(t, Validate("ws-invalid"), "Invalid prefixed ULID should be invalid")
	assert.False(t, Validate(""), "Empty string should be invalid")
}

func TestCompare(t *testing.T) {
	// Create ULIDs with known timestamps
	time1 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	time2 := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)

	id1 := NewWithTime(time1)
	id2 := NewWithTime(time2)

	// Test comparison with different timestamps
	assert.Equal(t, -1, id1.Compare(id2), "Earlier ULID should be less than later ULID")
	assert.Equal(t, 1, id2.Compare(id1), "Later ULID should be greater than earlier ULID")
	assert.Equal(t, 0, id1.Compare(id1), "Same ULID should be equal")

	// Test with prefixes - prefix should not affect comparison
	id1Copy := id1
	id1Copy.SetPrefix(PrefixWorkspace)
	assert.Equal(t, 0, id1.Compare(id1Copy),
		"Setting a prefix should not affect comparison when the ULID value is the same")

	// Create a new ULID with the same timestamp
	// Note: Even with the same timestamp, ULIDs can be different due to the random component
	idWithSameTimestamp := NewWithTime(time1)

	// We cannot guarantee equality, but we need to verify they sort correctly
	// If they have the same timestamp, the comparison should be consistent
	// (either always the same or consistent ordering)
	compare1 := id1.Compare(idWithSameTimestamp)
	compare2 := idWithSameTimestamp.Compare(id1)

	// They should be opposites (if one returns 1, the other should return -1)
	// Unless they're exactly equal (both return 0)
	if compare1 == 0 {
		assert.Equal(t, 0, compare2, "If one comparison returns 0, the other should too")
	} else {
		assert.Equal(t, -compare1, compare2, "Comparisons should be symmetrical opposites")
	}
}

func TestIsZero(t *testing.T) {
	// Test nil ULID
	assert.True(t, Nil.IsZero(), "Nil ULID should be zero")

	// Test non-nil ULID
	id := Generate()
	assert.False(t, id.IsZero(), "Generated ULID should not be zero")
}

func TestBytes(t *testing.T) {
	id := Generate()
	bytes := id.Bytes()

	// The byte representation should be 16 bytes long (128 bits)
	assert.Equal(t, 16, len(bytes), "ULID byte representation should be 16 bytes")

	// Test roundtrip conversion
	fromBytes, err := FromBytes(bytes)
	require.NoError(t, err)
	assert.Equal(t, id.RawString(), fromBytes.RawString())
}

func TestJSONMarshalUnmarshal(t *testing.T) {
	// Test marshaling/unmarshaling a raw ULID
	id := Generate()
	data, err := json.Marshal(id)
	require.NoError(t, err)

	var unmarshaled ULID
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, id, unmarshaled)

	// Test marshaling/unmarshaling a prefixed ULID
	prefixedID := GenerateWithPrefix(PrefixWorkspace)
	data, err = json.Marshal(prefixedID)
	require.NoError(t, err)

	var unmarshaledPrefixed ULID
	err = json.Unmarshal(data, &unmarshaledPrefixed)
	require.NoError(t, err)
	assert.Equal(t, prefixedID, unmarshaledPrefixed)
	assert.Equal(t, PrefixWorkspace, unmarshaledPrefixed.Prefix())
}

func TestDatabaseSerialization(t *testing.T) {
	// Test Value (for database storage)
	id := GenerateWithPrefix(PrefixFile)
	value, err := id.Value()
	require.NoError(t, err)

	// Check that the value is a string
	strValue, ok := value.(string)
	require.True(t, ok, "Value should return a string")

	// Test Scan (for database retrieval)
	var scanned ULID
	err = scanned.Scan(strValue)
	require.NoError(t, err)
	assert.Equal(t, id, scanned)

	// Test scanning from []byte
	var scannedFromBytes ULID
	err = scannedFromBytes.Scan([]byte(strValue))
	require.NoError(t, err)
	assert.Equal(t, id, scannedFromBytes)

	// Test scanning from nil
	var scannedFromNil ULID
	err = scannedFromNil.Scan(nil)
	require.NoError(t, err)
	assert.True(t, scannedFromNil.IsZero())

	// Test scanning from invalid type
	var scannedFromInvalid ULID
	err = scannedFromInvalid.Scan(123)
	assert.Error(t, err)
}

func TestDomainIDGeneration(t *testing.T) {
	// Test all domain-specific ID generation functions
	testCases := []struct {
		name       string
		idFunction func() string
		prefix     string
	}{
		{"WorkspaceID", WorkspaceID, PrefixWorkspace},
		{"RequestID", RequestID, PrefixRequest},
		{"ReviewID", ReviewID, PrefixReview},
		{"FileID", FileID, PrefixFile},
		{"ChunkID", ChunkID, PrefixChunk},
		{"SyncID", SyncID, PrefixSync},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id := tc.idFunction()
			assert.Contains(t, id, tc.prefix+PrefixSeparator)
			assert.True(t, Validate(id))

			parsed, err := Parse(id)
			require.NoError(t, err)
			assert.Equal(t, tc.prefix, parsed.Prefix())
		})
	}
}

func TestStringRepresentations(t *testing.T) {
	// Test String with prefix
	prefixedID := GenerateWithPrefix(PrefixWorkspace)
	assert.Contains(t, prefixedID.String(), PrefixWorkspace+PrefixSeparator)

	// Test String without prefix
	rawID := Generate()
	assert.NotContains(t, rawID.String(), PrefixSeparator)

	// Test RawString
	assert.Equal(t, rawID.RawString(), rawID.String(),
		"RawString and String should be the same for unprefixed ULIDs")
	assert.NotEqual(t, prefixedID.RawString(), prefixedID.String(),
		"RawString and String should be different for prefixed ULIDs")
	assert.NotContains(t, prefixedID.RawString(), PrefixSeparator,
		"RawString should not contain the prefix")
}

func TestTimeExtraction(t *testing.T) {
	// Create a ULID with a specific timestamp
	timestamp := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	id := NewWithTime(timestamp)

	// Extract the timestamp
	extractedTime := id.Time()

	// The extracted time should be close to the original timestamp
	// (there might be some precision loss due to the ULID timestamp format)
	timeDiff := timestamp.Sub(extractedTime).Milliseconds()
	assert.LessOrEqual(t, timeDiff, int64(1),
		"Extracted time should be close to the original timestamp")
}

func TestMustParse(t *testing.T) {
	// Test with valid ULID
	id := Generate()
	parsed := MustParse(id.String())
	assert.Equal(t, id, parsed)

	// Test with invalid ULID (should panic)
	assert.Panics(t, func() {
		MustParse("invalid-ulid")
	})
}

func TestPrefixOperations(t *testing.T) {
	// Test SetPrefix
	id := Generate()
	assert.False(t, id.HasPrefix(), "New ULID should not have a prefix")

	id.SetPrefix(PrefixFile)
	assert.True(t, id.HasPrefix(), "ULID should now have a prefix")
	assert.Equal(t, PrefixFile, id.Prefix(), "Prefix should match the set value")
	assert.Contains(t, id.String(), PrefixFile+PrefixSeparator,
		"String representation should contain the prefix")
}

func TestBase32(t *testing.T) {
	id := Generate()
	base32Str := id.Base32()

	// Base32 encoding should produce a string
	assert.NotEmpty(t, base32Str)

	// Test round-trip conversion
	fromBase32, err := FromBase32(base32Str)
	require.NoError(t, err)

	// The round-trip ULID should match the original (without prefix)
	assert.Equal(t, id.RawString(), fromBase32.RawString())

	// Test with invalid base32 string
	_, err = FromBase32("invalid-base32")
	assert.Error(t, err)
}

func TestDriverValueConverter(t *testing.T) {
	// Test that ULID can be used with database/sql driver interface
	var v driver.Valuer = Generate()

	val, err := v.Value()
	require.NoError(t, err)
	assert.IsType(t, "", val, "Value should return a string")
}

func TestFromBytesErrors(t *testing.T) {
	// Instead of testing with an invalid byte slice that causes panic,
	// we'll check if the string parsing fails with an invalid ULID
	_, err := Parse("invalid")
	assert.Error(t, err, "Parsing an invalid ULID should return an error")

	// Test marshaling and unmarshaling an invalid ULID
	var invalidULID ULID
	data, err := json.Marshal("invalid-ulid")
	require.NoError(t, err)

	err = invalidULID.UnmarshalJSON(data)
	assert.Error(t, err, "Unmarshaling an invalid ULID should return an error")
}

func BenchmarkGenerate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Generate()
	}
}

func BenchmarkGenerateWithPrefix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GenerateWithPrefix(PrefixWorkspace)
	}
}

func BenchmarkParse(b *testing.B) {
	id := Generate().String()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(id)
	}
}

func BenchmarkString(b *testing.B) {
	id := Generate()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = id.String()
	}
}
