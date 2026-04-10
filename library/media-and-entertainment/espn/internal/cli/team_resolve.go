package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/client"
)

// resolveTeamID resolves a team identifier to a numeric ESPN team ID.
// Accepts numeric IDs (passthrough), team names ("warriors", "Golden State Warriors"),
// abbreviations ("GS"), or slugs ("golden-state-warriors").
func resolveTeamID(c *client.Client, sport, league, input string) (string, error) {
	// Numeric ID passthrough
	if isNumeric(input) {
		return input, nil
	}

	// Fetch teams list
	path := fmt.Sprintf("/%s/%s/teams", sport, league)
	data, err := c.Get(path, map[string]string{})
	if err != nil {
		return "", fmt.Errorf("fetching teams list: %w", err)
	}

	teams := extractTeams(data)
	if len(teams) == 0 {
		return "", fmt.Errorf("no teams found for %s/%s", sport, league)
	}

	// Exact match (case-insensitive) against all name fields
	lower := strings.ToLower(input)
	var matches []teamInfo
	for _, t := range teams {
		if strings.EqualFold(t.DisplayName, input) ||
			strings.EqualFold(t.ShortDisplayName, input) ||
			strings.EqualFold(t.Name, input) ||
			strings.EqualFold(t.Abbreviation, input) ||
			strings.EqualFold(t.Slug, input) {
			matches = append(matches, t)
		}
	}

	if len(matches) == 1 {
		return matches[0].ID, nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous team %q matches %d teams:\n%s", input, len(matches), formatTeamList(matches))
	}

	// Substring match as fallback
	for _, t := range teams {
		if strings.Contains(strings.ToLower(t.DisplayName), lower) ||
			strings.Contains(strings.ToLower(t.ShortDisplayName), lower) ||
			strings.Contains(strings.ToLower(t.Name), lower) {
			matches = append(matches, t)
		}
	}

	if len(matches) == 1 {
		return matches[0].ID, nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous team %q matches %d teams:\n%s\nhint: use a more specific name or the numeric team ID", input, len(matches), formatTeamList(matches))
	}

	// No match - suggest closest
	suggestion := suggestTeam(input, teams)
	if suggestion != "" {
		return "", fmt.Errorf("no team found matching %q\nhint: did you mean %s?", input, suggestion)
	}
	return "", fmt.Errorf("no team found matching %q\nhint: run 'espn-pp-cli teams list %s %s' to see available teams", input, sport, league)
}

type teamInfo struct {
	ID               string
	DisplayName      string
	ShortDisplayName string
	Name             string
	Abbreviation     string
	Slug             string
}

// extractTeams pulls team info from the ESPN teams list response.
// The response structure is: {sports: [{leagues: [{teams: [{team: {...}}]}]}]}
func extractTeams(data json.RawMessage) []teamInfo {
	var resp struct {
		Sports []struct {
			Leagues []struct {
				Teams []struct {
					Team struct {
						ID               string `json:"id"`
						DisplayName      string `json:"displayName"`
						ShortDisplayName string `json:"shortDisplayName"`
						Name             string `json:"name"`
						Abbreviation     string `json:"abbreviation"`
						Slug             string `json:"slug"`
					} `json:"team"`
				} `json:"teams"`
			} `json:"leagues"`
		} `json:"sports"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}

	var teams []teamInfo
	for _, sport := range resp.Sports {
		for _, league := range sport.Leagues {
			for _, t := range league.Teams {
				teams = append(teams, teamInfo{
					ID:               t.Team.ID,
					DisplayName:      t.Team.DisplayName,
					ShortDisplayName: t.Team.ShortDisplayName,
					Name:             t.Team.Name,
					Abbreviation:     t.Team.Abbreviation,
					Slug:             t.Team.Slug,
				})
			}
		}
	}
	return teams
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func formatTeamList(teams []teamInfo) string {
	var lines []string
	for _, t := range teams {
		lines = append(lines, fmt.Sprintf("  %s: %s (%s)", t.ID, t.DisplayName, t.Abbreviation))
	}
	return strings.Join(lines, "\n")
}

func suggestTeam(input string, teams []teamInfo) string {
	best := ""
	bestDist := 4
	for _, t := range teams {
		for _, name := range []string{t.Name, t.ShortDisplayName, t.Abbreviation} {
			d := levenshteinDistance(strings.ToLower(input), strings.ToLower(name))
			if d < bestDist {
				bestDist = d
				best = fmt.Sprintf("%q (ID: %s)", t.DisplayName, t.ID)
			}
		}
	}
	return best
}
