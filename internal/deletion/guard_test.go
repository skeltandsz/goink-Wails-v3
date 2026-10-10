package deletion

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuardChecksOnlyTargetRegistrationsInOrder(t *testing.T) {
	chapterFirst := func(_ context.Context, novelID, targetID int64) ([]Blocker, error) {
		assert.Equal(t, int64(1), novelID)
		assert.Equal(t, int64(2), targetID)
		return []Blocker{{Kind: "first", ID: 10}}, nil
	}
	chapterSecond := func(_ context.Context, _, _ int64) ([]Blocker, error) {
		return []Blocker{{Kind: "second", ID: 11}}, nil
	}
	characterChecker := func(_ context.Context, _, _ int64) ([]Blocker, error) {
		t.Fatal("checker for another target kind must not run")
		return nil, nil
	}

	guard := NewGuard(
		For(EntityChapter, chapterFirst),
		For(EntityChapter, chapterSecond),
		For(EntityCharacter, characterChecker),
	)

	blockers, err := guard.Blockers(context.Background(), Target{NovelID: 1, Kind: EntityChapter, ID: 2})
	require.NoError(t, err)
	assert.Equal(t, []Blocker{{Kind: "first", ID: 10}, {Kind: "second", ID: 11}}, blockers)
}

func TestGuardStopsWhenCheckerFails(t *testing.T) {
	called := false
	guard := NewGuard(
		For(EntityChapter, func(context.Context, int64, int64) ([]Blocker, error) {
			return nil, errors.New("database unavailable")
		}),
		For(EntityChapter, func(context.Context, int64, int64) ([]Blocker, error) {
			called = true
			return nil, nil
		}),
	)

	_, err := guard.Blockers(context.Background(), Target{Kind: EntityChapter})
	require.Error(t, err)
	assert.ErrorContains(t, err, "check chapter deletion blockers")
	assert.False(t, called)
}
