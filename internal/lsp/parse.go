package lsp

import (
	"encoding/json"
	"fmt"
	"strings"
)

func FileToURI(file string) string {
	if strings.HasPrefix(file, "file://") {
		return file
	}
	return "file://" + file
}

func uriToFile(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	return uri
}

func ParseLocation(data json.RawMessage) (*Location, error) {
	if string(data) == "null" || len(data) == 0 || string(data) == "{}" {
		return nil, nil
	}

	var loc struct {
		URI   string `json:"uri"`
		Range struct {
			Start struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"start"`
		} `json:"range"`
	}

	if err := json.Unmarshal(data, &loc); err != nil {
		return nil, fmt.Errorf("unmarshal location: %w", err)
	}

	return &Location{
		Path:   uriToFile(loc.URI),
		Line:   loc.Range.Start.Line,
		Column: loc.Range.Start.Character,
	}, nil
}

func ParseLocations(data json.RawMessage) ([]Location, error) {
	if string(data) == "null" || len(data) == 0 {
		return nil, nil
	}

	var locs []struct {
		URI   string `json:"uri"`
		Range struct {
			Start struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"start"`
		} `json:"range"`
	}

	if err := json.Unmarshal(data, &locs); err != nil {
		return nil, fmt.Errorf("unmarshal locations: %w", err)
	}

	result := make([]Location, len(locs))
	for i, loc := range locs {
		result[i] = Location{
			Path:   uriToFile(loc.URI),
			Line:   loc.Range.Start.Line,
			Column: loc.Range.Start.Character,
		}
	}

	return result, nil
}

func ParseHover(data json.RawMessage) (string, error) {
	if string(data) == "null" || len(data) == 0 {
		return "", nil
	}

	var hover struct {
		Contents struct {
			Value string `json:"value"`
		} `json:"contents"`
	}

	if err := json.Unmarshal(data, &hover); err != nil {
		return "", fmt.Errorf("unmarshal hover: %w", err)
	}

	return hover.Contents.Value, nil
}

func ParseDiagnostics(data json.RawMessage) ([]Diagnostic, error) {
	if string(data) == "null" || len(data) == 0 {
		return nil, nil
	}

	var diags []struct {
		Severity int `json:"severity"`
		Range    struct {
			Start struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"start"`
		} `json:"range"`
		Message string `json:"message"`
		Source  string `json:"source"`
	}

	if err := json.Unmarshal(data, &diags); err != nil {
		return nil, fmt.Errorf("unmarshal diagnostics: %w", err)
	}

	result := make([]Diagnostic, len(diags))
	for i, d := range diags {
		severity := "info"
		switch d.Severity {
		case 1:
			severity = "error"
		case 2:
			severity = "warning"
		case 3:
			severity = "info"
		case 4:
			severity = "hint"
		}
		result[i] = Diagnostic{
			Severity: severity,
			Line:     d.Range.Start.Line,
			Column:   d.Range.Start.Character,
			Message:  d.Message,
			Source:   d.Source,
		}
	}

	return result, nil
}
