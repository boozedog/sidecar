package gitstatus

import "testing"

func makeEntries(count int, status FileStatus) []*FileEntry {
	entries := make([]*FileEntry, count)
	for i := 0; i < count; i++ {
		entries[i] = &FileEntry{
			Path:   "file",
			Status: status,
		}
	}
	return entries
}

func TestCommitSectionCapacity_TruncatesFiles(t *testing.T) {
	p := &Plugin{
		tree: &FileTree{
			Staged:    makeEntries(2, StatusAdded),
			Modified:  makeEntries(3, StatusModified),
			Untracked: makeEntries(2, StatusUntracked),
		},
		recentCommits: make([]*Commit, 10),
	}

	got := p.commitSectionCapacity(16)
	want := 5
	if got != want {
		t.Fatalf("commitSectionCapacity = %d, want %d", got, want)
	}
}

func TestCommitSectionCapacity_CleanWithStatus(t *testing.T) {
	p := &Plugin{
		tree:           &FileTree{},
		pushInProgress: true,
	}

	got := p.commitSectionCapacity(10)
	want := 5
	if got != want {
		t.Fatalf("commitSectionCapacity = %d, want %d", got, want)
	}
}

func TestCommitSectionCapacity_Minimum(t *testing.T) {
	p := &Plugin{
		tree: &FileTree{},
	}

	got := p.commitSectionCapacity(5)
	want := 2
	if got != want {
		t.Fatalf("commitSectionCapacity = %d, want %d", got, want)
	}
}
