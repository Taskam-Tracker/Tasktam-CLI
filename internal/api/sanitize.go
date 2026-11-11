package api

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode"
)

// cleanJSON strips terminal control characters from every string (and key) of a JSON document. Titles,
// comments and names on shared projects are written by other people; printed raw, an ESC sequence in one
// could rewrite the screen, the window title or the clipboard.
func cleanJSON(data []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // keep ids exact through the round trip
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return json.Marshal(clean(v))
}

func clean(v any) any {
	switch x := v.(type) {
	case string:
		return Clean(x)
	case []any:
		for i := range x {
			x[i] = clean(x[i])
		}
		return x
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[Clean(k)] = clean(e)
		}
		return out
	}
	return v
}

// Clean drops C0/C1 control characters (ESC, CR, BEL, DEL, CSI, …) but keeps newlines and tabs.
func Clean(s string) string {
	if !strings.ContainsFunc(s, isControl) {
		return s
	}
	return strings.Map(func(r rune) rune {
		if isControl(r) {
			return -1
		}
		return r
	}, s)
}

func isControl(r rune) bool { return r != '\n' && r != '\t' && unicode.IsControl(r) }
