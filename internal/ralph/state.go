package ralph

import (
	"encoding/json"
	"os"
	"time"
)

// State tracks iteration history for rate limiting.
type State struct {
	TotalIterations int       `json:"total_iterations"`
	Timestamps      []int64   `json:"timestamps"`
	LastRun         time.Time `json:"last_run"`
}

func loadState() State {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return State{Timestamps: []int64{}}
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{Timestamps: []int64{}}
	}
	if state.Timestamps == nil {
		state.Timestamps = []int64{}
	}
	return state
}

func saveState(state State) {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(stateFile, data, 0644)
}

func pruneOldTimestamps(state *State) {
	cutoff := time.Now().Add(-24 * time.Hour).Unix()
	var kept []int64
	for _, ts := range state.Timestamps {
		if ts > cutoff {
			kept = append(kept, ts)
		}
	}
	state.Timestamps = kept
}

func countRecentIterations(timestamps []int64) (hourCount, dayCount int) {
	now := time.Now()
	hourAgo := now.Add(-time.Hour).Unix()
	dayAgo := now.Add(-24 * time.Hour).Unix()

	for _, ts := range timestamps {
		if ts > dayAgo {
			dayCount++
			if ts > hourAgo {
				hourCount++
			}
		}
	}
	return
}
