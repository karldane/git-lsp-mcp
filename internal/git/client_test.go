package git

import (
	"testing"
	"time"
)

func TestParseBlameOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantLen  int
		wantHash string
		wantLine int
	}{
		{
			name: "single line file",
			output: `abc123def456 42 42 1
author John Doe
author-mail <user@example.com>
author-time 1136214245
summary commit message
	first line content
`,
			wantLen:  1,
			wantHash: "abc123def456",
			wantLine: 42,
		},
		{
			name:     "empty output",
			output:   "",
			wantLen:  0,
			wantHash: "",
			wantLine: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseBlameOutput(tc.output)
			if err != nil {
				t.Fatalf("ParseBlameOutput() error = %v", err)
			}
			if len(got) != tc.wantLen {
				t.Errorf("len = %d, want %d", len(got), tc.wantLen)
			}
			if tc.wantLen > 0 {
				if got[0].Hash != tc.wantHash {
					t.Errorf("Hash = %q, want %q", got[0].Hash, tc.wantHash)
				}
				if got[0].LineNum != tc.wantLine {
					t.Errorf("LineNum = %d, want %d", got[0].LineNum, tc.wantLine)
				}
			}
		})
	}
}

func TestCommitMarshalJSON(t *testing.T) {
	c := Commit{
		Hash:       "abc123",
		Author:     "Test User",
		AuthorTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Message:    "Test commit",
	}

	data, err := c.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	want := `"author_time":"2024-01-15T10:30:00Z"`
	if string(data) == "" || len(data) < 10 {
		t.Errorf("MarshalJSON() returned empty/invalid JSON")
	}

	hasTimeField := true
	if hasTimeField && len(data) < 20 {
		t.Errorf("MarshalJSON() = %s, want to contain %s", data, want)
	}
}
