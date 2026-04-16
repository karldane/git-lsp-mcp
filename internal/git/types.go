package git

import (
	"bufio"
	"context"
	"encoding/json"
	"strconv"
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
	ObjectPrefix     = ""
	AuthorPrefix     = "author-mail "
	AuthorTimePrefix = "author-time "
	AuthorNamePrefix = "author "
)

func ParseBlameOutput(output string) ([]BlameLine, error) {
	var lines []BlameLine
	var current BlameLine
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()

		fields := strings.Fields(line)
		if len(fields) >= 1 && current.Hash == "" {
			current = BlameLine{
				Hash:    fields[0],
				LineNum: 0,
			}
			if len(fields) >= 2 {
				if n, err := strconv.Atoi(fields[1]); err == nil {
					current.LineNum = n
				}
			}
			continue
		}

		if strings.HasPrefix(line, AuthorNamePrefix) && current.Hash != "" {
			author := strings.TrimPrefix(line, AuthorNamePrefix)
			author = strings.TrimSpace(author)
			current.Author = author
			continue
		}

		if strings.HasPrefix(line, AuthorPrefix) {
			if current.Author == "" {
				author := strings.TrimPrefix(line, AuthorPrefix)
				author = strings.Trim(author, "<>")
				if idx := strings.Index(author, "@"); idx != -1 {
					author = author[:idx]
				}
				author = strings.TrimSpace(author)
				current.Author = author
			}
			continue
		}

		if strings.HasPrefix(line, AuthorTimePrefix) {
			tsStr := strings.TrimPrefix(line, AuthorTimePrefix)
			tsStr = strings.TrimSpace(tsStr)
			if sec, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
				current.Date = time.Unix(sec, 0)
			}
			continue
		}

		if strings.HasPrefix(line, "\t") {
			current.Content = strings.TrimPrefix(line, "\t")
			if current.LineNum == 0 {
				if len(fields) >= 2 {
					if n, err := strconv.Atoi(fields[1]); err == nil {
						current.LineNum = n
					}
				}
			}
			lines = append(lines, current)
			current = BlameLine{}
		}
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
