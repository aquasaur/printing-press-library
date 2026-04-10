package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/client"
)

// resolveLastEventID fetches a team's schedule and returns the Nth most recent
// completed game's event ID. n=1 means the most recent completed game.
func resolveLastEventID(c *client.Client, sport, league, teamID string, n int) (string, error) {
	if n < 1 {
		n = 1
	}

	path := fmt.Sprintf("/%s/%s/teams/%s/schedule", sport, league, teamID)
	data, err := c.Get(path, map[string]string{})
	if err != nil {
		return "", fmt.Errorf("fetching team schedule: %w", err)
	}

	var schedule struct {
		Events []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Competitions []struct {
				Status struct {
					Type struct {
						Completed bool `json:"completed"`
					} `json:"type"`
				} `json:"status"`
			} `json:"competitions"`
		} `json:"events"`
	}
	if err := json.Unmarshal(data, &schedule); err != nil {
		return "", fmt.Errorf("parsing schedule: %w", err)
	}

	// Collect completed games in order (schedule is chronological)
	var completed []string
	for _, e := range schedule.Events {
		if len(e.Competitions) > 0 && e.Competitions[0].Status.Type.Completed {
			completed = append(completed, e.ID)
		}
	}

	if len(completed) == 0 {
		return "", fmt.Errorf("no completed games found for this team")
	}
	if n > len(completed) {
		return "", fmt.Errorf("only %d completed games found, but --last %d requested", len(completed), n)
	}

	// Return the Nth from the end
	return completed[len(completed)-n], nil
}
