package domain_test

import (
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewID(t *testing.T) {
	t.Run("returns version 7 UUID", func(t *testing.T) {
		id := domain.NewID()

		parsed, err := uuid.Parse(id)

		require.NoError(t, err)
		assert.EqualValues(t, 7, parsed.Version())
	})

	t.Run("sorts later IDs after earlier ones when generated in sequence", func(t *testing.T) {
		ids := make([]string, 100)

		for i := range ids {
			ids[i] = domain.NewID()
		}

		assert.True(t, slices.IsSorted(ids))
	})
}
