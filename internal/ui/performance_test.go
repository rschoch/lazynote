package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rschoch/lazynote/internal/notes"
)

func performanceNotes(count, bodyBytes int) []notes.Note {
	result := make([]notes.Note, count)
	body := strings.Repeat("Performance Body Text ", bodyBytes/22+1)[:bodyBytes]
	for i := range result {
		result[i] = notes.Note{
			ID:        fmt.Sprintf("note-%08d", i),
			Title:     fmt.Sprintf("Note Title %08d", i),
			Body:      body,
			CreatedAt: time.Unix(int64(i), 0),
			Tags:      []string{"work", "benchmark"},
		}
	}
	return result
}

func BenchmarkIndexedFilter(b *testing.B) {
	for _, count := range []int{1000, 10000} {
		app := &App{allNotes: performanceNotes(count, 1024)}
		app.rebuildSearchIndex()
		app.applyFilter("")
		app.filterQuery = "not-present"

		b.Run(fmt.Sprintf("notes=%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				app.applyFilter("")
			}
		})
	}
}

func BenchmarkVisibleListFormatting(b *testing.B) {
	loaded := performanceNotes(10000, 128)
	const height = 40

	b.ReportAllocs()
	for b.Loop() {
		start, end, _ := listViewport(len(loaded), 5000, 4980, height)
		for i := start; i < end; i++ {
			_ = listLine(loaded[i], i == 5000, false, 28)
		}
	}
}

func BenchmarkDetailMetrics(b *testing.B) {
	note := notes.Note{ID: "large", Body: strings.Repeat("A fairly long line of note body text for wrapping\n", 2200)}
	b.Run("cached", func(b *testing.B) {
		app := &App{}
		_ = app.cachedDetailMaxOffset(note, 80, 30)
		b.ReportAllocs()
		for b.Loop() {
			_ = app.cachedDetailMaxOffset(note, 80, 30)
		}
	})
	b.Run("recalculate", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = maxDetailOffset(note.Body, 80, 30)
		}
	})
}
