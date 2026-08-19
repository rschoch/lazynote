package notes

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func performanceStoreNotes(count, bodyBytes int) []Note {
	result := make([]Note, count)
	body := strings.Repeat("Performance Body Text ", bodyBytes/22+1)[:bodyBytes]
	for i := range result {
		result[i] = Note{
			ID:        fmt.Sprintf("note-%08d", i),
			Title:     fmt.Sprintf("Note Title %08d", i),
			Body:      body,
			CreatedAt: time.Unix(int64(i), 0),
			Tags:      []string{"work", "benchmark"},
		}
	}
	return result
}

func BenchmarkStoreLoad1000(b *testing.B) {
	store := NewStore(filepath.Join(b.TempDir(), "notes.json"))
	if err := store.Save(performanceStoreNotes(1000, 1024)); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := store.Load(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStoreSave1000(b *testing.B) {
	store := NewStore(filepath.Join(b.TempDir(), "notes.json"))
	loaded := performanceStoreNotes(1000, 1024)

	b.ReportAllocs()
	for b.Loop() {
		if err := store.Save(loaded); err != nil {
			b.Fatal(err)
		}
	}
}
