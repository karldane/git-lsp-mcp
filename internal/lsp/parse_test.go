package lsp

import (
	"encoding/json"
	"testing"
)

func TestParseLocation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Location
		wantErr bool
	}{
		{
			name:  "valid location",
			input: `{"uri":"file:///path/to/file.go","range":{"start":{"line":10,"character":5}}}`,
			want: &Location{
				Path:   "/path/to/file.go",
				Line:   10,
				Column: 5,
			},
		},
		{
			name:  "null result",
			input: "null",
			want:  nil,
		},
		{
			name:  "empty object",
			input: "{}",
			want:  nil,
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:    "invalid json",
			input:   "not json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := json.RawMessage(tt.input)
			got, err := ParseLocation(data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLocation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got == nil && tt.want != nil {
				t.Errorf("ParseLocation() = nil, want non-nil")
				return
			}
			if got != nil && tt.want == nil {
				t.Errorf("ParseLocation() = %v, want nil", got)
				return
			}
			if got != nil && tt.want != nil {
				if got.Path != tt.want.Path {
					t.Errorf("ParseLocation() Path = %v, want %v", got.Path, tt.want.Path)
				}
				if got.Line != tt.want.Line {
					t.Errorf("ParseLocation() Line = %v, want %v", got.Line, tt.want.Line)
				}
				if got.Column != tt.want.Column {
					t.Errorf("ParseLocation() Column = %v, want %v", got.Column, tt.want.Column)
				}
			}
		})
	}
}

func TestParseLocations(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{
			name:  "two locations",
			input: `[{"uri":"file:///a.go","range":{"start":{"line":1,"character":2}}},{"uri":"file:///b.go","range":{"start":{"line":3,"character":4}}}]`,
			want:  2,
		},
		{
			name:  "null result",
			input: "null",
			want:  0,
		},
		{
			name:    "invalid json",
			input:   "{invalid}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := json.RawMessage(tt.input)
			got, err := ParseLocations(data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLocations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.want {
				t.Errorf("ParseLocations() len = %v, want %v", len(got), tt.want)
			}
		})
	}
}

func TestParseHover(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "valid hover",
			input: `{"contents":{"value":"func main()"}}`,
			want:  "func main()",
		},
		{
			name:  "null result",
			input: "null",
			want:  "",
		},
		{
			name:  "empty result",
			input: "",
			want:  "",
		},
		{
			name:    "invalid json",
			input:   "not json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := json.RawMessage(tt.input)
			got, err := ParseHover(data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseHover() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseHover() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseDiagnostics(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{
			name:  "one diagnostic error",
			input: `[{"severity":1,"range":{"start":{"line":1,"character":2}},"message":"error","source":"gopls"}]`,
			want:  1,
		},
		{
			name:  "multiple diagnostics mixed",
			input: `[{"severity":1,"range":{"start":{"line":1,"character":2}},"message":"err","source":"linter"},{"severity":2,"range":{"start":{"line":3,"character":4}},"message":"warn","source":"linter"}]`,
			want:  2,
		},
		{
			name:  "null result",
			input: "null",
			want:  0,
		},
		{
			name:    "invalid json",
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := json.RawMessage(tt.input)
			got, err := ParseDiagnostics(data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDiagnostics() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.want {
				t.Errorf("ParseDiagnostics() len = %v, want %v", len(got), tt.want)
			}
		})
	}
}

func TestFileToURI(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/path/to/file.go", "file:///path/to/file.go"},
		{"file:///path/to/file.go", "file:///path/to/file.go"},
		{"/a/b/c.go", "file:///a/b/c.go"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := fileToURI(tt.input)
			if got != tt.want {
				t.Errorf("fileToURI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestURIToFile(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"file:///path/to/file.go", "/path/to/file.go"},
		{"/path/to/file.go", "/path/to/file.go"},
		{"file:///a/b/c.go", "/a/b/c.go"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := uriToFile(tt.input)
			if got != tt.want {
				t.Errorf("uriToFile(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
