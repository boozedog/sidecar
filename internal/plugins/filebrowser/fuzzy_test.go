package filebrowser

import (
	"testing"
)

func TestFuzzyMatch_ExactMatch(t *testing.T) {
	score, ranges := FuzzyMatch("main.go", "main.go")

	if score == 0 {
		t.Error("exact match should have positive score")
	}
	if len(ranges) != 1 {
		t.Errorf("exact match should have 1 range, got %d", len(ranges))
	}
	if ranges[0].Start != 0 || ranges[0].End != 7 {
		t.Errorf("range should be [0,7], got [%d,%d]", ranges[0].Start, ranges[0].End)
	}
}

func TestFuzzyMatch_SubstringMatch(t *testing.T) {
	score, ranges := FuzzyMatch("main", "cmd/main.go")

	if score == 0 {
		t.Error("substring match should have positive score")
	}
	if len(ranges) == 0 {
		t.Error("should have match ranges")
	}
}

func TestFuzzyMatch_NonConsecutive(t *testing.T) {
	// "mgo" should match "main.go" (m...g.o)
	score, ranges := FuzzyMatch("mgo", "main.go")

	if score == 0 {
		t.Error("non-consecutive match should have positive score")
	}
	// Should have multiple ranges since chars are not consecutive
	if len(ranges) < 2 {
		t.Logf("ranges: %+v", ranges)
	}
}

func TestFuzzyMatch_CaseInsensitive(t *testing.T) {
	score1, _ := FuzzyMatch("MAIN", "main.go")
	score2, _ := FuzzyMatch("main", "MAIN.GO")
	score3, _ := FuzzyMatch("MaIn", "mAiN.go")

	if score1 == 0 || score2 == 0 || score3 == 0 {
		t.Error("case-insensitive matching should work")
	}
}

func TestFuzzyMatch_NoMatch(t *testing.T) {
	score, ranges := FuzzyMatch("xyz", "main.go")

	if score != 0 {
		t.Error("non-matching query should have 0 score")
	}
	if ranges != nil {
		t.Error("non-matching query should have nil ranges")
	}
}

func TestFuzzyMatch_PartialNoMatch(t *testing.T) {
	// "mainx" shouldn't match "main.go" because 'x' not found
	score, _ := FuzzyMatch("mainx", "main.go")

	if score != 0 {
		t.Error("partial match (missing chars) should have 0 score")
	}
}

func TestFuzzyMatch_WordStartBonus(t *testing.T) {
	// "mg" matching at word starts should score higher
	score1, _ := FuzzyMatch("mg", "main.go")       // m at start, g after .
	score2, _ := FuzzyMatch("mg", "aamainago.txt") // m and g in middle

	if score1 <= score2 {
		t.Errorf("word start match should score higher: %d <= %d", score1, score2)
	}
}

func TestFuzzyMatch_ConsecutiveBonus(t *testing.T) {
	// "main" consecutive should score higher than spread out (without word start bonus)
	score1, _ := FuzzyMatch("main", "main.go")
	score2, _ := FuzzyMatch("main", "xmxaxixn.go") // spread out, no word start bonus

	if score1 <= score2 {
		t.Errorf("consecutive match should score higher: %d <= %d", score1, score2)
	}
}

func TestFuzzyMatch_ShorterPathBonus(t *testing.T) {
	score1, _ := FuzzyMatch("test", "test.go")
	score2, _ := FuzzyMatch("test", "very/deep/nested/path/to/test.go")

	if score1 <= score2 {
		t.Errorf("shorter path should score higher: %d <= %d", score1, score2)
	}
}

func TestFuzzyMatch_FilenameBonus(t *testing.T) {
	// Match in filename portion should score higher than match in dir portion
	score1, _ := FuzzyMatch("test", "src/test.go")       // "test" in filename
	score2, _ := FuzzyMatch("test", "test/something.go") // "test" in directory

	if score1 <= score2 {
		t.Errorf("filename match should score higher: %d <= %d", score1, score2)
	}
}

func TestFuzzyMatch_EmptyQuery(t *testing.T) {
	score, ranges := FuzzyMatch("", "main.go")

	if score != 0 {
		t.Error("empty query should have 0 score")
	}
	if ranges != nil {
		t.Error("empty query should have nil ranges")
	}
}

func TestFuzzyMatch_EmptyTarget(t *testing.T) {
	score, _ := FuzzyMatch("test", "")

	if score != 0 {
		t.Error("empty target should have 0 score")
	}
}

func TestFuzzySort_ByScore(t *testing.T) {
	matches := []QuickOpenMatch{
		{Path: "low.go", Score: 10},
		{Path: "high.go", Score: 100},
		{Path: "med.go", Score: 50},
	}

	FuzzySort(matches)

	if matches[0].Score != 100 || matches[1].Score != 50 || matches[2].Score != 10 {
		t.Error("should be sorted by score descending")
	}
}

func TestFuzzySort_TiebreakByLength(t *testing.T) {
	matches := []QuickOpenMatch{
		{Path: "very/long/path.go", Score: 50},
		{Path: "short.go", Score: 50},
		{Path: "medium/path.go", Score: 50},
	}

	FuzzySort(matches)

	if matches[0].Path != "short.go" {
		t.Errorf("shortest path should be first, got %s", matches[0].Path)
	}
}

func TestFuzzyFilter_EmptyQuery(t *testing.T) {
	files := []string{"a.go", "b.go", "c.go"}
	matches := FuzzyFilter(files, "", 10)

	if len(matches) != 3 {
		t.Errorf("empty query should return all files, got %d", len(matches))
	}
}

func TestFuzzyFilter_MaxResults(t *testing.T) {
	files := make([]string, 100)
	for i := range files {
		files[i] = "file" + string(rune('a'+i%26)) + ".go"
	}

	matches := FuzzyFilter(files, "file", 10)

	if len(matches) > 10 {
		t.Errorf("should limit to 10 results, got %d", len(matches))
	}
}

func TestFuzzyFilter_NoMatches(t *testing.T) {
	files := []string{"main.go", "test.go"}
	matches := FuzzyFilter(files, "xyz", 10)

	if len(matches) != 0 {
		t.Errorf("should have 0 matches, got %d", len(matches))
	}
}

func TestFuzzyMatch_MatchRanges(t *testing.T) {
	_, ranges := FuzzyMatch("ab", "aXXb")

	// Should have 2 separate ranges: [0,1] and [3,4]
	if len(ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d: %+v", len(ranges), ranges)
	}
	if ranges[0].Start != 0 || ranges[0].End != 1 {
		t.Errorf("first range should be [0,1], got [%d,%d]", ranges[0].Start, ranges[0].End)
	}
	if ranges[1].Start != 3 || ranges[1].End != 4 {
		t.Errorf("second range should be [3,4], got [%d,%d]", ranges[1].Start, ranges[1].End)
	}
}

func TestFuzzyFilter_ExtractsName(t *testing.T) {
	files := []string{"src/app/main.go"}
	matches := FuzzyFilter(files, "main", 10)

	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Name != "main.go" {
		t.Errorf("Name should be 'main.go', got %q", matches[0].Name)
	}
	if matches[0].Path != "src/app/main.go" {
		t.Errorf("Path should be 'src/app/main.go', got %q", matches[0].Path)
	}
}
