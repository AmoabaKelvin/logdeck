package alerts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

func historyPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "alerts-history.json")
}

func TestHistoryCapEviction(t *testing.T) {
	h := newHistory(historyPath(t), 3)
	for i := 1; i <= 5; i++ {
		h.append(models.Alert{ID: strconv.Itoa(i)})
	}
	got := h.list(0)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	for i, want := range []string{"5", "4", "3"} {
		if got[i].ID != want {
			t.Fatalf("entry %d = %s, want %s (newest first)", i, got[i].ID, want)
		}
	}
}

func TestHistoryListLimitAndNonNil(t *testing.T) {
	h := newHistory(historyPath(t), 500)

	if got := h.list(0); got == nil || len(got) != 0 {
		t.Fatalf("empty list = %#v, want non-nil empty slice", got)
	}
	if got := h.list(-1); got == nil {
		t.Fatal("negative limit returned nil")
	}

	for i := 1; i <= 5; i++ {
		h.append(models.Alert{ID: strconv.Itoa(i)})
	}
	if got := h.list(2); len(got) != 2 || got[0].ID != "5" {
		t.Fatalf("list(2) = %#v, want 2 newest entries", got)
	}
	if got := h.list(0); len(got) != 5 {
		t.Fatalf("list(0) = %d entries, want all 5 (default limit)", len(got))
	}
}

func TestHistoryCorruptFileTolerated(t *testing.T) {
	path := historyPath(t)
	if err := os.WriteFile(path, []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}
	h := newHistory(path, 500)
	h.load()
	if got := h.list(0); len(got) != 0 {
		t.Fatalf("corrupt file yielded %d entries, want 0", len(got))
	}
	// Store still works after a corrupt load.
	h.append(models.Alert{ID: "a"})
	h.flushIfDirty()
	if got := h.list(0); len(got) != 1 {
		t.Fatalf("append after corrupt load failed: %d entries", len(got))
	}
}

func TestHistoryPersistRoundTrip(t *testing.T) {
	path := historyPath(t)
	h := newHistory(path, 500)
	h.append(models.Alert{ID: "old"})
	h.append(models.Alert{ID: "new"})
	h.flushIfDirty()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("history file not written: %v", err)
	}
	var onDisk []models.Alert
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatalf("history file is not a JSON array: %v", err)
	}
	if len(onDisk) != 2 || onDisk[0].ID != "new" {
		t.Fatalf("on-disk = %#v, want [new old]", onDisk)
	}

	reloaded := newHistory(path, 500)
	reloaded.load()
	if got := reloaded.list(0); len(got) != 2 || got[0].ID != "new" {
		t.Fatalf("reload = %#v, want [new old]", got)
	}
}

func TestHistoryClearPersistsEmptyArray(t *testing.T) {
	path := historyPath(t)
	h := newHistory(path, 500)
	h.append(models.Alert{ID: "a"})
	h.flushIfDirty()

	h.clear()

	if got := h.list(0); got == nil || len(got) != 0 {
		t.Fatalf("after clear list = %#v, want non-nil empty", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk []models.Alert
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatalf("cleared file is not valid JSON: %v (content %q)", err, data)
	}
	if len(onDisk) != 0 {
		t.Fatalf("cleared file has %d entries, want 0", len(onDisk))
	}
	if string(data) == "null" {
		t.Fatal("cleared file serialized as null, want []")
	}
}
