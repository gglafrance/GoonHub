package core

import (
	"testing"
)

func TestPrepareImportInteractions_JizzHistory(t *testing.T) {
	count := 1
	p, err := prepareImportInteractions(&ImportInteractionsInput{
		JizzedCount: &count,
		JizzHistory: []string{"2024-05-01T20:15:00Z", "2024-06-02T21:00:00+02:00"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.jizzHistory) != 2 || p.jizzHistory[1].Hour() != 19 {
		t.Fatalf("history not parsed to UTC: %v", p.jizzHistory)
	}
	if p.jizzedCount == nil || *p.jizzedCount != 2 {
		t.Fatalf("count must be raised to the number of events, got %v", p.jizzedCount)
	}
}

func TestPrepareImportInteractions_InvalidJizzDate(t *testing.T) {
	if _, err := prepareImportInteractions(&ImportInteractionsInput{JizzHistory: []string{"yesterday"}}); err == nil {
		t.Fatal("expected a validation error")
	}
}
