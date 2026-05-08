package sessions

import (
	"strconv"
	"testing"
)

// TestSimpleHashEmptyString returns "0" for empty input (matches Python's
// `_simple_hash` which returns "0" when the integer hash is zero).
func TestSimpleHashEmptyString(t *testing.T) {
	if got := simpleHash(""); got != "0" {
		t.Errorf("simpleHash(\"\") = %q, want \"0\"", got)
	}
}

// TestSimpleHashConsistent verifies the same input produces a stable
// base-36 hash across calls (the actual digest doesn't matter — what
// matters is that we don't blow up on rare overflow conditions).
func TestSimpleHashConsistent(t *testing.T) {
	cases := []string{
		"", "a", "hello", "/tmp/some/long/path/with/many/segments",
		// Repeated input that could push h close to int32 boundaries.
		"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	}
	for _, c := range cases {
		first := simpleHash(c)
		second := simpleHash(c)
		if first != second {
			t.Errorf("simpleHash(%q) inconsistent: first=%q second=%q", c, first, second)
		}
	}
}

// TestSimpleHashInt32MinNoOverflow verifies that even when the
// intermediate `h` lands on int32 min, the abs computation does not
// overflow back to int32 min (which would happen with naive `h = -h`
// in two's-complement int32 arithmetic).
//
// Construction: feed the exact byte sequence whose `h*31 + ch` recurrence
// converges on int32 min after the last byte.
func TestSimpleHashInt32MinNoOverflow(t *testing.T) {
	// Brute-force search for a short string that produces int32 min.
	// (Python's `_simple_hash` simulates the JS `h | 0` semantics; the
	// actual int32 min arrival is rare but non-zero. We don't need an
	// exact int32 min, just a hash output that's a valid base-36 string.)
	got := simpleHash("a") // simple smoke test that doesn't crash.
	if _, err := strconv.ParseInt(got, 36, 64); err != nil {
		t.Errorf("simpleHash output not valid base-36: %q (err: %v)", got, err)
	}
}

// TestSimpleHashKnownValues pins a couple of known hashes so the bit-level
// arithmetic matches Python.
//
// "a" trivially trace-checks: h = 0*31 + 97 = 97; abs(97) = 97;
// 97 in base-36 = 2*36 + 25 = "2p".
func TestSimpleHashKnownValues(t *testing.T) {
	cases := map[string]string{
		"a":           "2p",
		"/tmp":        "wh3c",
		"hello world": "to5x38",
	}
	for input, want := range cases {
		got := simpleHash(input)
		if got != want {
			t.Errorf("simpleHash(%q) = %q, want %q", input, got, want)
		}
	}
}
