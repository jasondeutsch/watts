// Package redact removes known credential values from diagnostics. It is not a
// detector for unknown secrets or a substitute for workload isolation.
package redact

import (
	"bytes"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const Marker = "[REDACTED]"

// Set is immutable and may be shared by independent output streams.
type Set struct {
	values   []string
	replacer *strings.Replacer
}

func New(values ...string) *Set {
	unique := map[string]bool{}
	for _, value := range values {
		if value == "" {
			continue
		}
		encoded, _ := json.Marshal(value)
		escaped := value
		for depth := 0; depth < 3; depth++ {
			data, _ := json.Marshal(escaped)
			escaped = string(data[1 : len(data)-1])
			unique[escaped] = true
		}
		for _, variant := range []string{value, html.EscapeString(value), string(encoded[1 : len(encoded)-1]), strconv.Quote(value)[1 : len(strconv.Quote(value))-1], url.QueryEscape(value), url.PathEscape(value)} {
			unique[variant] = true
		}
	}
	s := &Set{}
	for value := range unique {
		s.values = append(s.values, value)
	}
	sort.Slice(s.values, func(i, j int) bool { return len(s.values[i]) > len(s.values[j]) })
	pairs := make([]string, 0, len(s.values)*2)
	for _, value := range s.values {
		pairs = append(pairs, value, Marker)
	}
	s.replacer = strings.NewReplacer(pairs...)
	return s
}

func (s *Set) Text(text string) string {
	if s == nil || len(s.values) == 0 {
		return text
	}
	return s.replacer.Replace(text)
}

// JSON sanitizes string values and keys without changing numeric metrics or
// booleans. JSONL is supported. Malformed diagnostics are still redacted as text.
func (s *Set) JSON(data []byte) []byte {
	if s == nil || len(s.values) == 0 {
		return data
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var output bytes.Buffer
	for {
		var value any
		err := decoder.Decode(&value)
		if err == io.EOF {
			return output.Bytes()
		}
		if err != nil {
			return []byte(s.Text(string(data)))
		}
		encoded, err := json.Marshal(s.jsonValue(value))
		if err != nil {
			return []byte(s.Text(string(data)))
		}
		output.Write(encoded)
		output.WriteByte('\n')
	}
}
func (s *Set) jsonValue(value any) any {
	switch typed := value.(type) {
	case string:
		return s.Text(typed)
	case []any:
		for i := range typed {
			typed[i] = s.jsonValue(typed[i])
		}
	case map[string]any:
		sanitized := make(map[string]any, len(typed))
		for key, item := range typed {
			sanitized[s.Text(key)] = s.jsonValue(item)
		}
		return sanitized
	}
	return value
}

// Error deliberately omits a secret-bearing cause: SDKs serialize error chains.
// Unchanged errors retain their identity, including cancellation and timeouts.
func (s *Set) Error(err error) error {
	if err == nil {
		return nil
	}
	text := s.Text(err.Error())
	if text == err.Error() {
		return err
	}
	return errors.New(text)
}

// Writer holds only a possible credential prefix. Each stdout/stderr stream
// requires its own Writer; Flush must be called when that stream ends.
type Writer struct {
	set     *Set
	output  io.Writer
	pending string
	err     error
}

func (s *Set) Writer(output io.Writer) *Writer {
	if s == nil {
		s = New()
	}
	return &Writer{set: s, output: output}
}
func (w *Writer) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	w.pending += string(data)
	w.drain(false)
	return len(data), w.err
}
func (w *Writer) Flush() error { w.drain(true); return w.err }
func (w *Writer) drain(final bool) {
	var safe strings.Builder
	for len(w.pending) > 0 && w.err == nil {
		// Wait for a longer overlapping match before consuming a shorter one.
		if !final && w.set.incompletePrefix(w.pending) {
			break
		}
		matched := false
		for _, value := range w.set.values {
			if strings.HasPrefix(w.pending, value) {
				safe.WriteString(Marker)
				w.pending = w.pending[len(value):]
				matched = true
				break
			}
		}
		if !matched {
			safe.WriteByte(w.pending[0])
			w.pending = w.pending[1:]
		}
	}
	if safe.Len() == 0 || w.err != nil {
		return
	}
	n, err := io.WriteString(w.output, safe.String())
	if err != nil || n != safe.Len() {
		w.err = errors.New("redacted diagnostic output could not be written")
	}
}

func (s *Set) incompletePrefix(text string) bool {
	for _, value := range s.values {
		if len(text) < len(value) && strings.HasPrefix(value, text) {
			return true
		}
	}
	return false
}
