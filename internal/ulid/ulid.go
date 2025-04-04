// Package ulid provides a type-safe wrapper around github.com/oklog/ulid/v2
// with additional functionality for database/json integration and utilities.
//
// ULIDs are Universally Unique Lexicographically Sortable Identifiers.
// They are suitable for use as primary keys and provide several advantages over UUIDs:
// - Lexicographically sortable (by time, making them ideal for database indexes)
// - 128-bit compatible with UUID
// - Case-insensitive
// - No special characters (URL safe)
// - Monotonic sort order (correctly detects and handles the same timestamp)
package ulid

import (
	"bytes"
	"crypto/rand"
	"database/sql/driver"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// Common prefixes for different parts of the application
const (
	// Prefix for workspace-related ULIDs
	PrefixWorkspace = "ws"

	// Prefix for request IDs
	PrefixRequest = "req"

	// Prefix for review-related ULIDs
	PrefixReview = "rev"

	// Prefix for issue-related ULIDs
	PrefixIssue = "iss"

	// Prefix for file-related ULIDs
	PrefixFile = "file"

	// Prefix for chunk-related ULIDs
	PrefixChunk = "chunk"

	// Prefix for user-related ULIDs
	PrefixSync = "sync"

	// Prefix for setting-related ULIDs
	PrefixSetting = "set"

	// PrefixSeparator is used to separate the prefix from the ULID
	PrefixSeparator = "-"
)

var (
	entropy     = ulid.Monotonic(rand.Reader, 0)
	entropyLock sync.Mutex
	// Nil represents the zero value of ULID, useful for nil checks
	Nil = ULID{ulid.ULID{}, ""}
)

// ULID is a custom type that wraps ulid.ULID with additional functionality
// for database integration, JSON serialization, and comparison utilities.
type ULID struct {
	ulid.ULID
	prefix string
}

// Generate creates a new ULID with the current timestamp.
// This is the recommended function to use for most ULID generation cases.
func Generate() ULID {
	return NewWithTime(time.Now())
}

// GenerateWithPrefix creates a new ULID with the current timestamp and a prefix.
// The prefix provides context about what the ID represents (e.g., "ws" for workspace).
func GenerateWithPrefix(prefix string) ULID {
	id := NewWithTime(time.Now())
	id.prefix = prefix
	return id
}

// NewWithTime creates a new ULID with a specific timestamp.
// This is useful for generating ULIDs with custom timestamps.
func NewWithTime(t time.Time) ULID {
	entropyLock.Lock()
	id := ulid.MustNew(ulid.Timestamp(t), entropy)
	entropyLock.Unlock()
	return ULID{id, ""}
}

// NewWithTimeAndPrefix creates a new ULID with a specific timestamp and prefix.
func NewWithTimeAndPrefix(t time.Time, prefix string) ULID {
	id := NewWithTime(t)
	id.prefix = prefix
	return id
}

// Parse parses a ULID string and returns the ULID struct.
// Returns an error if the string is not a valid ULID.
// It handles both plain ULIDs and prefixed ULIDs (e.g., "ws-01AN4Z07BY79KA1307SR9X4MV3").
func Parse(id string) (ULID, error) {
	// Check if the ID has a prefix
	parts := strings.Split(id, PrefixSeparator)

	var rawID string
	var prefix string

	if len(parts) > 1 {
		// ID has a prefix
		prefix = parts[0]
		rawID = parts[1]
	} else {
		// No prefix
		rawID = id
	}

	// Parse the raw ULID part
	parsed, err := ulid.Parse(rawID)
	if err != nil {
		return ULID{}, err
	}

	return ULID{parsed, prefix}, nil
}

// FromString is an alias for Parse for API compatibility.
func FromString(s string) (ULID, error) {
	return Parse(s)
}

// Validate checks if a string is a valid ULID format.
// Returns true if valid, false otherwise.
// It supports both plain ULIDs and prefixed ULIDs.
func Validate(id string) bool {
	// Check if the ID has a prefix
	parts := strings.Split(id, PrefixSeparator)

	var rawID string

	if len(parts) > 1 {
		// ID has a prefix, validate the second part
		rawID = parts[1]
	} else {
		// No prefix
		rawID = id
	}

	_, err := ulid.Parse(rawID)
	return err == nil
}

// Compare compares two ULIDs lexicographically.
// Returns -1 if u < other, 1 if u > other, and 0 if they're equal.
// The comparison ignores prefixes and only compares the actual ULID values.
func (u ULID) Compare(other ULID) int {
	return bytes.Compare(u.ULID[:], other.ULID[:])
}

// IsZero returns true if the ULID is the zero value (Nil).
func (u ULID) IsZero() bool {
	return u.ULID == ulid.ULID{}
}

// Bytes returns the ULID as a byte slice.
func (u ULID) Bytes() []byte {
	return u.ULID[:]
}

// Base32 returns the base32 encoded representation of the ULID.
func (u ULID) Base32() string {
	return base32.StdEncoding.EncodeToString(u.Bytes())
}

// SetPrefix sets the prefix for the ULID.
func (u *ULID) SetPrefix(prefix string) {
	u.prefix = prefix
}

// Prefix returns the prefix of the ULID.
func (u ULID) Prefix() string {
	return u.prefix
}

// HasPrefix returns true if the ULID has a prefix.
func (u ULID) HasPrefix() bool {
	return u.prefix != ""
}

// FromBytes creates a ULID from a byte slice.
// Returns an error if the byte slice is not a valid ULID.
func FromBytes(b []byte) (ULID, error) {
	id, err := ulid.Parse(ulid.ULID(b).String())
	return ULID{id, ""}, err
}

// FromBase32 creates a ULID from a base32 encoded string.
// Returns an error if the string is not a valid base32 encoded ULID.
func FromBase32(s string) (ULID, error) {
	b, err := base32.StdEncoding.DecodeString(s)
	if err != nil {
		return ULID{}, err
	}
	return FromBytes(b)
}

// MustParse is like Parse but panics if the string cannot be parsed.
// This simplifies initialization of global variables.
func MustParse(s string) ULID {
	id, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return id
}

// String returns the string representation of the ULID.
// If the ULID has a prefix, it's included in the format "prefix-ulid".
func (u ULID) String() string {
	if u.prefix != "" {
		return u.prefix + PrefixSeparator + u.ULID.String()
	}
	return u.ULID.String()
}

// RawString returns the string representation of the ULID without any prefix.
func (u ULID) RawString() string {
	return u.ULID.String()
}

// Time returns the timestamp component of the ULID.
func (u ULID) Time() time.Time {
	return ulid.Time(u.ULID.Time())
}

// MarshalJSON implements the json.Marshaler interface.
// ULIDs are marshaled as strings in JSON.
func (u ULID) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// ULIDs are unmarshaled from string representations in JSON.
func (u *ULID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := Parse(s)
	if err != nil {
		return err
	}
	*u = parsed
	return nil
}

// Value implements the driver.Valuer interface for database serialization.
// ULIDs are stored as strings in databases.
func (u ULID) Value() (driver.Value, error) {
	return u.String(), nil
}

// Scan implements the sql.Scanner interface for database deserialization.
// ULIDs can be scanned from strings or byte slices.
func (u *ULID) Scan(src interface{}) error {
	switch src := src.(type) {
	case nil:
		return nil
	case string:
		parsed, err := Parse(src)
		if err != nil {
			return err
		}
		*u = parsed
		return nil
	case []byte:
		parsed, err := Parse(string(src))
		if err != nil {
			return err
		}
		*u = parsed
		return nil
	}
	return fmt.Errorf("cannot scan %T into ULID", src)
}

// Domain-specific ID generation methods

// WorkspaceID generates a new ULID with the workspace prefix
func WorkspaceID() string {
	return GenerateWithPrefix(PrefixWorkspace).String()
}

// RequestID generates a new ULID with the request prefix
func RequestID() string {
	return GenerateWithPrefix(PrefixRequest).String()
}

// ReviewID generates a new ULID with the review prefix
func ReviewID() string {
	return GenerateWithPrefix(PrefixReview).String()
}

// IssueID generates a new ULID with the issue prefix
func IssueID() string {
	return GenerateWithPrefix(PrefixIssue).String()
}

// FileID generates a new ULID with the file prefix
func FileID() string {
	return GenerateWithPrefix(PrefixFile).String()
}

// ChunkID generates a new ULID with the chunk prefix
func ChunkID() string {
	return GenerateWithPrefix(PrefixChunk).String()
}

// SyncID generates a new ULID with the user prefix
func SyncID() string {
	return GenerateWithPrefix(PrefixSync).String()
}

// SettingID generates a new ULID with the setting prefix
func SettingID() string {
	return GenerateWithPrefix(PrefixSetting).String()
}
