package core

import (
	"errors"
	"testing"
	"time"

	"goonhub/internal/data"
)

func TestGetJizzHistory_CapsAndKeepsTotal(t *testing.T) {
	svc, repo := newTestInteractionService(t)
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	events := make([]data.UserSceneJizzEvent, JizzHistoryLimit+5)
	for i := range events {
		// Repository returns most recent first
		events[i] = data.UserSceneJizzEvent{JizzedAt: base.Add(-time.Duration(i) * time.Hour)}
	}
	repo.EXPECT().GetJizzEvents(uint(1), uint(2)).Return(events, nil)

	history, err := svc.GetJizzHistory(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if history.Total != JizzHistoryLimit+5 || len(history.Events) != JizzHistoryLimit {
		t.Fatalf("total=%d events=%d", history.Total, len(history.Events))
	}
	if !history.Events[0].Equal(base) {
		t.Fatalf("first event = %v, want most recent %v", history.Events[0], base)
	}
}

func TestGetJizzHistory_Empty(t *testing.T) {
	svc, repo := newTestInteractionService(t)
	repo.EXPECT().GetJizzEvents(uint(1), uint(2)).Return(nil, nil)
	history, err := svc.GetJizzHistory(1, 2)
	if err != nil || history.Total != 0 || history.Events == nil || len(history.Events) != 0 {
		t.Fatalf("unexpected empty history: %+v, %v", history, err)
	}
}

func TestGetJizzHistory_Error(t *testing.T) {
	svc, repo := newTestInteractionService(t)
	repo.EXPECT().GetJizzEvents(uint(1), uint(2)).Return(nil, errors.New("boom"))
	if _, err := svc.GetJizzHistory(1, 2); err == nil {
		t.Fatal("expected an error")
	}
}
