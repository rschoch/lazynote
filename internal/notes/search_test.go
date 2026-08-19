package notes

import "testing"

func TestFoldSearchTextRemovesCanonicalDiacritics(t *testing.T) {
	tests := map[string]string{
		"Café":               "cafe",
		"NAÏVE":              "naive",
		"re\u0301sume\u0301": "resume",
		"smørrebrød":         "smørrebrød",
	}

	for input, want := range tests {
		if got := FoldSearchText(input); got != want {
			t.Errorf("FoldSearchText(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMatchesQueryIgnoresAccents(t *testing.T) {
	note := Note{
		Title: "Café ideas",
		Body:  "Write a résumé for Zoë",
		Tags:  []string{"déjà-vu"},
	}

	for _, query := range []string{"cafe", "resume", "ZOE", "#deja-vu"} {
		if !MatchesQuery(note, query) {
			t.Errorf("MatchesQuery(note, %q) = false, want true", query)
		}
	}
}
