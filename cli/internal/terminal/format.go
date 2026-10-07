package terminal

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// IsInteractive reports whether r is an interactive terminal.
func IsInteractive(r any) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// Ask prompts on the terminal and returns the answer, or def for an empty line.
func (a *Context) Ask(question, def string) string {
	fmt.Fprintf(a.Output, "%s [%s]: ", question, def)
	var line []byte
	buf := make([]byte, 1)
	for {
		n, err := a.Input.Read(buf)
		if n > 0 {
			if buf[0] == '\n' {
				break
			}
			line = append(line, buf[0])
		}
		if err != nil {
			break
		}
	}
	if ans := strings.TrimSpace(string(line)); ans != "" {
		return ans
	}
	return def
}

func JSON(v any) (string, error) {
	var sb strings.Builder
	enc := Encoder(&sb)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

func Encoder(w io.Writer) *json.Encoder {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder
}
