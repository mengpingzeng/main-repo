package adapter

import (
	"context"
	"math/rand"
	"time"
)

type MockAdapter struct {
	FailPostIDs map[string]bool
	FailAll     bool
}

func NewMockAdapter() *MockAdapter {
	return &MockAdapter{
		FailPostIDs: make(map[string]bool),
	}
}

func (m *MockAdapter) Fetch(ctx context.Context, postID string, platform string) (*PlatformStats, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(50 * time.Millisecond):
	}

	if m.FailAll {
		return nil, &FetchError{Type: "network", PostID: postID, Platform: platform, Err: context.DeadlineExceeded}
	}

	if m.FailPostIDs[postID] {
		return nil, &FetchError{Type: "not_found", PostID: postID, Platform: platform, Err: context.DeadlineExceeded}
	}

	return &PlatformStats{
		PostID:   postID,
		Platform: platform,
		Views:    int64(rand.Intn(50000)),
		Likes:    int64(rand.Intn(2000)),
		Comments: int64(rand.Intn(200)),
		Shares:   int64(rand.Intn(100)),
	}, nil
}
