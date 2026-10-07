package application

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	projectconfig "github.com/jasondeutsch/watts/internal/config"
)

type SessionInfo struct {
	File          string
	Models        []string
	Assistant     int
	ToolCalls     int
	Errors        int
	ErrorMessages []string
}

// newestSession finds the most recently written session file of an agent, optionally only one
// modified at or after since.
func (a *Service) NewestSession(cfg projectconfig.Config, agent string, since time.Time) (string, bool) {
	var best string
	var bestTime time.Time
	_ = filepath.WalkDir(filepath.Join(a.AgentDir(cfg, agent), "sessions"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".jsonl") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if mt := info.ModTime(); mt.After(bestTime) && !mt.Before(since.Add(-2*time.Second)) {
			best, bestTime = p, mt
		}
		return nil
	})
	return best, best != ""
}

func (a *Service) ReadSession(cfg projectconfig.Config, agent string, since time.Time) (SessionInfo, bool) {
	file, ok := a.NewestSession(cfg, agent, since)
	if !ok {
		return SessionInfo{}, false
	}
	f, err := os.Open(file)
	if err != nil {
		return SessionInfo{}, false
	}
	defer f.Close()
	info := SessionInfo{File: file}
	seen := map[string]bool{}
	addModel := func(v any) {
		if s, ok := v.(string); ok && s != "" && !seen[s] {
			seen[s] = true
			info.Models = append(info.Models, s)
		}
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		var m map[string]any
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			continue
		}
		typ, _ := m["type"].(string)
		if typ == "model_change" {
			for _, k := range []string{"model", "modelId", "to"} {
				addModel(m[k])
			}
		}
		if typ == "error" {
			info.Errors++
		}
		msg, _ := m["message"].(map[string]any)
		if msg == nil {
			continue
		}
		if role, _ := msg["role"].(string); role == "assistant" {
			info.Assistant++
			addModel(msg["model"])
		}
		if sr, _ := msg["stopReason"].(string); sr == "error" {
			info.Errors++
			detail, _ := msg["errorMessage"].(string)
			if detail == "" {
				detail = "agent stopped with an error; provider supplied no details"
			}
			info.ErrorMessages = append(info.ErrorMessages, detail)
		}
		if ie, _ := msg["isError"].(bool); ie {
			info.Errors++
		}
		if parts, ok := msg["content"].([]any); ok {
			for _, p := range parts {
				if pm, ok := p.(map[string]any); ok && pm["type"] == "toolCall" {
					info.ToolCalls++
				}
			}
		}
	}
	return info, true
}

func OneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		s = s[:max] + "..."
	}
	return s
}

func FirstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// formatEntry turns one session line into one readable line, or "" to skip it.
func FormatEntry(line []byte) string {
	var m map[string]any
	if json.Unmarshal(line, &m) != nil {
		return ""
	}
	stamp := ""
	if ts, _ := m["timestamp"].(string); ts != "" {
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			stamp = t.Local().Format("15:04:05") + " "
		}
	}
	switch typ, _ := m["type"].(string); typ {
	case "model_change":
		return stamp + "model: " + FirstString(m, "model", "modelId", "to")
	case "message":
		msg, _ := m["message"].(map[string]any)
		role, _ := msg["role"].(string)
		var out []string
		switch c := msg["content"].(type) {
		case string:
			out = append(out, role+": "+OneLine(c, 200))
		case []any:
			for _, p := range c {
				pm, _ := p.(map[string]any)
				switch pm["type"] {
				case "text":
					if t, _ := pm["text"].(string); strings.TrimSpace(t) != "" {
						prefix := role
						if role == "toolResult" {
							prefix = "result"
							if ie, _ := msg["isError"].(bool); ie {
								prefix = "result (error)"
							}
						}
						out = append(out, prefix+": "+OneLine(t, 160))
					}
				case "toolCall":
					args := ""
					for _, k := range []string{"arguments", "args", "input"} {
						if v, ok := pm[k]; ok {
							b, _ := json.Marshal(v)
							args = OneLine(string(b), 140)
							break
						}
					}
					out = append(out, "call "+FirstString(pm, "name", "toolName", "tool")+" "+args)
				}
			}
		}
		if len(out) == 0 {
			return ""
		}
		return stamp + strings.Join(out, "\n"+strings.Repeat(" ", len(stamp)))
	}
	return ""
}

// follow prints a readable feed of an agent's newest session. With once it prints what exists and
// returns; otherwise it keeps going until stop is closed. since limits it to sessions written
// after that time (zero means the newest session at all). tail is how many recent lines to show
// first, or zero for everything.
func (a *Service) Follow(cfg projectconfig.Config, agent string, since time.Time, once bool, tail int, stop <-chan struct{}) error {
	var file string
	var offset int64
	var pending []byte
	first := true
	emit := func(lines [][]byte) {
		for _, l := range lines {
			if out := FormatEntry(l); out != "" {
				a.say("%s", out)
			}
		}
	}
	for {
		if f, ok := a.NewestSession(cfg, agent, since); ok && f != file {
			file, offset, pending = f, 0, nil
			if !once {
				a.say("-- following %s", a.Rel(f))
			}
		}
		if file != "" {
			data, err := os.ReadFile(file)
			if err == nil && int64(len(data)) > offset {
				chunk := append(pending, data[offset:]...)
				offset = int64(len(data))
				lines := bytes.Split(chunk, []byte("\n"))
				pending = lines[len(lines)-1]
				lines = lines[:len(lines)-1]
				if first && tail > 0 && len(lines) > tail {
					lines = lines[len(lines)-tail:]
				}
				emit(lines)
				first = false
			}
		}
		if once {
			if file == "" {
				return errors.New("no session record found for the " + agent + " agent yet")
			}
			return nil
		}
		select {
		case <-stop:
			// One last read so nothing written just before the stop is lost.
			if file != "" {
				if data, err := os.ReadFile(file); err == nil && int64(len(data)) > offset {
					lines := bytes.Split(append(pending, data[offset:]...), []byte("\n"))
					emit(lines)
				}
			}
			return nil
		case <-time.After(400 * time.Millisecond):
		}
	}
}
