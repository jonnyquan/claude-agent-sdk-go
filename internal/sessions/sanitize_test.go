package sessions

import "testing"

func TestSanitizeUnicode_AppliesNFKC(t *testing.T) {
	// U+FB00 (LATIN SMALL LIGATURE FF) NFKC-decomposes to "ff".
	// Pre-fix: Go skipped NFKC and returned the input unchanged.
	got := sanitizeUnicode("ﬀ")
	if got != "ff" {
		t.Fatalf("expected NFKC-decomposed 'ff', got %q", got)
	}
}

func TestSanitizeUnicode_StripsZeroWidthAndDirectionalChars(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"zero-width space U+200B", "hello​world", "helloworld"},
		{"RTL override U+202E", "hello‮world", "helloworld"},
		{"LRI directional isolate U+2066", "hello⁦world", "helloworld"},
		// Go source disallows a literal BOM character; use the escape form.
		{"BOM U+FEFF", "hello\ufeffworld", "helloworld"},
		{"PUA character U+E000", "helloworld", "helloworld"},
		{"plain text passes through", "plain text", "plain text"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sanitizeUnicode(c.in)
			if got != c.want {
				t.Errorf("sanitizeUnicode(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestSanitizeUnicode_Idempotent(t *testing.T) {
	once := sanitizeUnicode("café​test")
	twice := sanitizeUnicode(once)
	if once != twice {
		t.Fatalf("sanitizeUnicode should be idempotent: once=%q twice=%q", once, twice)
	}
}
