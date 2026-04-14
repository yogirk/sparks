package manifest

import (
	"errors"
	"testing"
)

func TestOpenIngestFresh(t *testing.T) {
	db := openTestDB(t)
	rec, err := db.OpenIngest()
	if err != nil {
		t.Fatalf("OpenIngest: %v", err)
	}
	if rec.ID == 0 || rec.Status != IngestInProgress {
		t.Errorf("rec = %+v", rec)
	}
}

func TestOpenIngestBlocksConcurrent(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.OpenIngest(); err != nil {
		t.Fatalf("first OpenIngest: %v", err)
	}
	_, err := db.OpenIngest()
	if !errors.Is(err, ErrIngestInProgress) {
		t.Errorf("second OpenIngest err = %v, want ErrIngestInProgress", err)
	}
}

func TestCompleteIngestAllowsNewIngest(t *testing.T) {
	db := openTestDB(t)
	rec, err := db.OpenIngest()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.CompleteIngest(rec.ID, 3, "abc1234"); err != nil {
		t.Fatalf("CompleteIngest: %v", err)
	}
	// After completing, a new ingest can open.
	if _, err := db.OpenIngest(); err != nil {
		t.Errorf("OpenIngest after complete: %v", err)
	}
}

func TestCompleteIngestWithoutOpenErrors(t *testing.T) {
	db := openTestDB(t)
	err := db.CompleteIngest(9999, 0, "")
	if !errors.Is(err, ErrNoActiveIngest) {
		t.Errorf("err = %v, want ErrNoActiveIngest", err)
	}
}

func TestAbortIngestClearsLock(t *testing.T) {
	db := openTestDB(t)
	rec, _ := db.OpenIngest()
	if err := db.AbortIngest(rec.ID); err != nil {
		t.Fatalf("AbortIngest: %v", err)
	}
	if _, err := db.OpenIngest(); err != nil {
		t.Errorf("OpenIngest after abort: %v", err)
	}
}

func TestCurrentIngestReturnsNotFoundWhenNone(t *testing.T) {
	db := openTestDB(t)
	_, err := db.CurrentIngest()
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("CurrentIngest err = %v, want ErrNotFound", err)
	}
}
