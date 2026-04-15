package git

import (
	"bufio"
	"context"
	"encoding/json"
	"strings"
	"time"
)

type Commit struct {
	Hash       string
	Author     string
	AuthorTime time.Time
	Message    string
}

type BlameLine struct {
	LineNum int
	Hash    string
	Author  string
	Date    time.Time
	Content string
}

type GitClient interface {
	Clone(ctx context.Context, url, branch, dir string) error
	RemoteURL(ctx context.Context, dir string) (string, error)
	Fetch(ctx context.Context, dir string) error
	ResetHard(ctx context.Context, dir, ref string) error
	Log(ctx context.Context, dir, path string, limit int) ([]Commit, error)
	Blame(ctx context.Context, dir, path string) ([]BlameLine, error)
}

const (
	ObjectPrefix     = "author "
	AuthorPrefix     = "author-mail "
	AuthorTimePrefix = "author-time "
)

func ParseBlameOutput(output string) ([]BlameLine, error) {
	var lines []BlameLine
	var current BlameLine
	scanner := bufio.NewScanner(strings.NewReader(output))
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, ObjectPrefix) {
			if current.Hash != "" {
				lines = append(lines, current)
			}
			hash := strings.TrimPrefix(line, ObjectPrefix)
			current = BlameLine{
				Hash:    hash,
				LineNum: 0,
			}
			lineNum = 0
			continue
		}

		if strings.HasPrefix(line, AuthorPrefix) {
			author := strings.TrimPrefix(line, AuthorPrefix)
			author = strings.Trim(author, "<>")
			if idx := strings.Index(author, "@"); idx != -1 {
				author = author[:idx]
			}
			author = strings.TrimSpace(author)
			current.Author = author
			continue
		}

		if strings.HasPrefix(line, AuthorTimePrefix) {
			ts, _ := time.Parse("1136214245", strings.TrimPrefix(line, AuthorTimePrefix))
			current.Date = ts
			continue
		}

		if strings.HasPrefix(line, "\t") {
			current.Content = strings.TrimPrefix(line, "\t")
			current.LineNum = lineNum
			lines = append(lines, current)
			current = BlameLine{}
		}
		lineNum++
	}

	return lines, nil
}

type LogOutput struct {
	Commits []Commit `json:"commits"`
}

func (c Commit) MarshalJSON() ([]byte, error) {
	type Alias Commit
	return json.Marshal(&struct {
		AuthorTime string `json:"author_time"`
		*Alias
	}{
		AuthorTime: c.AuthorTime.Format(time.RFC3339),
		Alias:      (*Alias)(&c),
	})
}
