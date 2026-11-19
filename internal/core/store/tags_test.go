package store_test

import (
	"strings"
	"testing"

	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/skarabasakis/subtill/internal/core/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTags(t *testing.T) {
	tags := store.Tags{}

	t.Run("ListUnique", func(t *testing.T) {
		t.Run("collapses case when listing unique tags", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix", "video", "family")
			insertSubscription(t, db, "02", "Disney", "Video")

			got, err := tags.ListUnique(db)

			require.NoError(t, err)
			assert.Len(t, got, 2)
			assert.Equal(t, "family", got[0])
			assert.Equal(t, "video", strings.ToLower(got[1]))
		})

		t.Run("returns empty list when no tags exist", func(t *testing.T) {
			db := newDB(t)

			got, err := tags.ListUnique(db)

			require.NoError(t, err)
			assert.Equal(t, []string{}, got)
		})
	})

	t.Run("ListBySubscriptionID", func(t *testing.T) {
		t.Run("lists tags of subscription sorted by tag", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix", "video", "family")
			insertSubscription(t, db, "02", "Spotify", "music")

			got, err := tags.ListBySubscriptionID(db, "01")

			require.NoError(t, err)
			assert.Equal(t, []domain.TagRecord{
				{ID: "01-family", SubscriptionID: "01", Tag: "family"},
				{ID: "01-video", SubscriptionID: "01", Tag: "video"},
			}, got)
		})
	})

	t.Run("Insert", func(t *testing.T) {
		t.Run("rejects insert when id is empty", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix")

			err := tags.Insert(db, domain.TagRecord{SubscriptionID: "01", Tag: "video"})

			assert.Error(t, err)
			got, err := tags.ListBySubscriptionID(db, "01")
			require.NoError(t, err)
			assert.Empty(t, got)
		})

		t.Run("ignores insert when tag exists in another case", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix", "video")

			err := tags.Insert(db, domain.TagRecord{ID: "other", SubscriptionID: "01", Tag: "VIDEO"})

			require.NoError(t, err)
			got, err := tags.ListBySubscriptionID(db, "01")
			require.NoError(t, err)
			assert.Equal(t, []domain.TagRecord{{ID: "01-video", SubscriptionID: "01", Tag: "video"}}, got)
		})
	})

	t.Run("DeleteBySubscriptionID", func(t *testing.T) {
		t.Run("removes only that subscription's tags when deleting by subscription", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix", "video", "family")
			insertSubscription(t, db, "02", "Spotify", "music")

			affected, err := tags.DeleteBySubscriptionID(db, "01")

			require.NoError(t, err)
			assert.EqualValues(t, 2, affected)
			remaining, err := tags.ListUnique(db)
			require.NoError(t, err)
			assert.Equal(t, []string{"music"}, remaining)
		})
	})

	t.Run("DeleteBySubscriptionIDAndTag", func(t *testing.T) {
		t.Run("removes tag case-insensitively when deleting one tag", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix", "video", "family")

			affected, err := tags.DeleteBySubscriptionIDAndTag(db, "01", "VIDEO")

			require.NoError(t, err)
			assert.EqualValues(t, 1, affected)
			remaining, err := tags.ListUnique(db)
			require.NoError(t, err)
			assert.Equal(t, []string{"family"}, remaining)
		})

		t.Run("reports zero rows when deleting missing tag", func(t *testing.T) {
			db := newDB(t)
			insertSubscription(t, db, "01", "Netflix")

			affected, err := tags.DeleteBySubscriptionIDAndTag(db, "01", "video")

			require.NoError(t, err)
			assert.Zero(t, affected)
		})
	})
}
