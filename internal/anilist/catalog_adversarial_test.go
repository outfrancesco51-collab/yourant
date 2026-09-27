package anilist

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalog_Adversarial_PaginationAndSearch(t *testing.T) {
	t.Run("negative page and perPage clamping", func(t *testing.T) {
		res := FallbackTrending(-5, -10)
		require.NotNil(t, res)
		assert.Equal(t, 1, res.PageInfo.CurrentPage, "Negative page should clamp to 1")
		assert.Equal(t, 20, res.PageInfo.PerPage, "Negative perPage should clamp to 20")
		assert.Len(t, res.Items, 10, "Should return all 10 items")
		assert.False(t, res.PageInfo.HasNextPage)
	})

	t.Run("out of bounds page returns empty items", func(t *testing.T) {
		res := FallbackPopular(999, 10)
		require.NotNil(t, res)
		assert.Equal(t, 999, res.PageInfo.CurrentPage)
		assert.Empty(t, res.Items)
		assert.False(t, res.PageInfo.HasNextPage)
	})

	t.Run("empty search query returns all catalog items", func(t *testing.T) {
		res := FallbackSearch("", 1, 50)
		require.NotNil(t, res)
		assert.Len(t, res.Items, 10)
	})

	t.Run("whitespace search query returns all catalog items", func(t *testing.T) {
		res := FallbackSearch("   ", 1, 50)
		require.NotNil(t, res)
		assert.Len(t, res.Items, 10)
	})

	t.Run("case insensitive and partial search", func(t *testing.T) {
		resUpper := FallbackSearch("ATTACK", 1, 10)
		require.NotEmpty(t, resUpper.Items)
		assert.Equal(t, 16498, resUpper.Items[0].ID)

		resMixed := FallbackSearch("sTeInS", 1, 10)
		require.NotEmpty(t, resMixed.Items)
		assert.Equal(t, 9253, resMixed.Items[0].ID)
	})

	t.Run("search with no matches returns empty items", func(t *testing.T) {
		res := FallbackSearch("completely_nonexistent_anime_xyz_12345", 1, 10)
		require.NotNil(t, res)
		assert.Empty(t, res.Items)
		assert.Equal(t, 0, res.PageInfo.Total)
	})

	t.Run("detail lookup with zero and negative ID", func(t *testing.T) {
		_, errZero := FallbackDetail(0)
		assert.Equal(t, ErrAnimeNotFound, errZero)

		_, errNeg := FallbackDetail(-1)
		assert.Equal(t, ErrAnimeNotFound, errNeg)
	})
}
