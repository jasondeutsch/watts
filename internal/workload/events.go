package workload

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

type Usage struct {
	Input      *int64   `json:"input_tokens,omitempty"`
	Output     *int64   `json:"output_tokens,omitempty"`
	CacheRead  *int64   `json:"cache_read_tokens,omitempty"`
	CacheWrite *int64   `json:"cache_write_tokens,omitempty"`
	Cost       *float64 `json:"cost_usd,omitempty"`
	Provider   string   `json:"provider,omitempty"`
	Model      string   `json:"model,omitempty"`
}
type Event struct {
	Details map[string]any `json:"details,omitempty"`
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Message string         `json:"message,omitempty"`
	Usage   *Usage         `json:"usage,omitempty"`
}

func ReadEvents(file string) ([]Event, error) {
	data, _, err := ReadFile(filepath.Dir(file), filepath.Base(file), MaximumFileBytes)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	seen := map[string]string{}
	var events []Event
	for scanner.Scan() {
		var event Event
		data := scanner.Bytes()
		if err := json.Unmarshal(data, &event); err != nil {
			return events, fmt.Errorf("invalid workload event: %w", err)
		}
		if event.ID == "" || event.Type == "" {
			return events, fmt.Errorf("workload event needs an id")
		}
		encoded, _ := json.Marshal(event)
		if previous, exists := seen[event.ID]; exists {
			if previous != string(encoded) {
				return events, fmt.Errorf("conflicting event id %s", event.ID)
			}
			continue
		}
		seen[event.ID] = string(encoded)
		if event.Type == "usage" {
			if event.Usage == nil {
				return events, fmt.Errorf("usage event needs usage data")
			}
			if cost := event.Usage.Cost; cost != nil && (*cost < 0 || math.IsNaN(*cost) || math.IsInf(*cost, 0)) {
				return events, fmt.Errorf("invalid workload cost")
			}
			for _, count := range []*int64{event.Usage.Input, event.Usage.Output, event.Usage.CacheRead, event.Usage.CacheWrite} {
				if count != nil && *count < 0 {
					return events, fmt.Errorf("negative workload usage")
				}
			}
		}
		events = append(events, event)
	}
	return events, scanner.Err()
}

// Summarize reports missing counters as unknown, rather than treating them as
// zero. Cache tokens remain separate; provider input counters may include them.
func Summarize(events []Event) *Usage {
	var usage []Usage
	for _, event := range events {
		if event.Type == "usage" && event.Usage != nil {
			usage = append(usage, *event.Usage)
		}
	}
	if len(usage) == 0 {
		return nil
	}
	result := &Usage{}
	result.Input = sumCounts(usage, func(u Usage) *int64 { return u.Input })
	result.Output = sumCounts(usage, func(u Usage) *int64 { return u.Output })
	result.CacheRead = sumCounts(usage, func(u Usage) *int64 { return u.CacheRead })
	result.CacheWrite = sumCounts(usage, func(u Usage) *int64 { return u.CacheWrite })
	cost := 0.0
	for _, item := range usage {
		if item.Cost == nil {
			return result
		}
		cost += *item.Cost
	}
	result.Cost = &cost
	return result
}
func sumCounts(usage []Usage, selectCount func(Usage) *int64) *int64 {
	sum := int64(0)
	for _, item := range usage {
		count := selectCount(item)
		if count == nil || *count > math.MaxInt64-sum {
			return nil
		}
		sum += *count
	}
	return &sum
}
