package claudecode

import (
	"testing"
)

// TestWithMaxBudgetUSD tests the WithMaxBudgetUSD option
func TestWithMaxBudgetUSD(t *testing.T) {
	tests := []struct {
		name     string
		budget   float64
		expected *float64
	}{
		{
			name:     "set budget to 0.10",
			budget:   0.10,
			expected: float64Ptr(0.10),
		},
		{
			name:     "set budget to 1.00",
			budget:   1.00,
			expected: float64Ptr(1.00),
		},
		{
			name:     "set very small budget",
			budget:   0.0001,
			expected: float64Ptr(0.0001),
		},
		{
			name:     "set zero budget",
			budget:   0,
			expected: float64Ptr(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewOptions(WithMaxBudgetUSD(tt.budget))

			if opts.MaxBudgetUSD == nil {
				t.Fatal("MaxBudgetUSD should not be nil")
			}

			if *opts.MaxBudgetUSD != *tt.expected {
				t.Errorf("Expected budget %f, got %f", *tt.expected, *opts.MaxBudgetUSD)
			}
		})
	}
}

// TestWithMaxThinkingTokens tests the WithMaxThinkingTokens option
func TestWithMaxThinkingTokens(t *testing.T) {
	tests := []struct {
		name     string
		tokens   int
		expected int
	}{
		{
			name:     "set to 5000",
			tokens:   5000,
			expected: 5000,
		},
		{
			name:     "set to 10000",
			tokens:   10000,
			expected: 10000,
		},
		{
			name:     "set to zero",
			tokens:   0,
			expected: 0,
		},
		{
			name:     "set to negative (should still set)",
			tokens:   -1,
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewOptions(WithMaxThinkingTokens(tt.tokens))

			if opts.MaxThinkingTokens != tt.expected {
				t.Errorf("Expected tokens %d, got %d", tt.expected, opts.MaxThinkingTokens)
			}
		})
	}
}

// TestWithMaxBudgetUSDAndMaxThinkingTokens tests using both options together
func TestWithMaxBudgetUSDAndMaxThinkingTokens(t *testing.T) {
	budget := 0.50
	tokens := 8000

	opts := NewOptions(
		WithMaxBudgetUSD(budget),
		WithMaxThinkingTokens(tokens),
	)

	if opts.MaxBudgetUSD == nil {
		t.Fatal("MaxBudgetUSD should not be nil")
	}

	if *opts.MaxBudgetUSD != budget {
		t.Errorf("Expected budget %f, got %f", budget, *opts.MaxBudgetUSD)
	}

	if opts.MaxThinkingTokens != tokens {
		t.Errorf("Expected tokens %d, got %d", tokens, opts.MaxThinkingTokens)
	}
}

// Helper function to create float64 pointer
func float64Ptr(f float64) *float64 {
	return &f
}
