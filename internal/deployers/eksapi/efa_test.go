package eksapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Card counts and EFA interface limits as reported by DescribeInstanceTypes.
func TestEFACardLayout(t *testing.T) {
	cases := []struct {
		name             string
		instanceType     string
		cardCount        int
		maxEfaInterfaces int
		wantEfaCards     []int
	}{
		{
			// A RunInstances probe rejects card 1 with "limit: 0".
			name:             "two cards one interface puts EFA on the primary",
			instanceType:     "i7ie.48xlarge",
			cardCount:        2,
			maxEfaInterfaces: 1,
			wantEfaCards:     []int{0},
		},
		{
			name:             "every card carries EFA when the limit matches the count",
			instanceType:     "p5.48xlarge",
			cardCount:        32,
			maxEfaInterfaces: 32,
			wantEfaCards:     seq(0, 32),
		},
		{
			name:             "primary is skipped for an ENA-only type",
			instanceType:     "p6-b300.48xlarge",
			cardCount:        17,
			maxEfaInterfaces: 16,
			wantEfaCards:     seq(1, 16),
		},
		{
			// Fewer EFA interfaces than cards: the limit caps the list so we never
			// request EFA on more cards than are allowed.
			name:             "limit caps the cards",
			instanceType:     "example.8xlarge",
			cardCount:        9,
			maxEfaInterfaces: 2,
			wantEfaCards:     []int{0, 1},
		},
		{
			name:             "single card single interface",
			instanceType:     "c5n.18xlarge",
			cardCount:        1,
			maxEfaInterfaces: 1,
			wantEfaCards:     []int{0},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			efaCards, err := efaCardLayout(c.instanceType, c.cardCount, c.maxEfaInterfaces)
			require.NoError(t, err)
			assert.Equal(t, c.wantEfaCards, efaCards)
		})
	}
}

func TestEFACardLayoutRejectsATypeWithNoUsableCard(t *testing.T) {
	// Nowhere left for EFA must be an error, not a launch without it.
	_, err := efaCardLayout("p6-b300.48xlarge", 1, 1)
	assert.ErrorContains(t, err, "no network card on p6-b300.48xlarge can carry an EFA interface")
}

func seq(start, n int) []int {
	out := make([]int, 0, n)
	for i := start; i < start+n; i++ {
		out = append(out, i)
	}
	return out
}
