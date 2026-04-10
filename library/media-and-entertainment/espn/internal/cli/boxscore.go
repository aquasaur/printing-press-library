package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type playerStat struct {
	Name  string            `json:"name"`
	Stats map[string]string `json:"stats"`
}

type teamBoxScore struct {
	Team    string       `json:"team"`
	Players []playerStat `json:"players"`
}

func newBoxScoreCmd(flags *rootFlags) *cobra.Command {
	var flagEvent string
	var flagTeam string
	var flagLast int

	cmd := &cobra.Command{
		Use:   "boxscore <sport> <league>",
		Short: "Box score with player stats for a game",
		Long:  "Extract and display the box score from a game summary.\nUse --team to automatically find the most recent game, or --event for a specific game.",
		Example: `  espn-pp-cli boxscore basketball nba --team warriors
  espn-pp-cli boxscore basketball nba --team GS --last 2 --json
  espn-pp-cli boxscore basketball nba --event 401811009`,
		RunE: func(cmd *cobra.Command, args []string) error {
			hasEvent := cmd.Flags().Changed("event")
			hasTeam := cmd.Flags().Changed("team")
			if !hasEvent && !hasTeam && !flags.dryRun {
				return fmt.Errorf("either --event or --team is required")
			}
			if len(args) < 1 {
				return usageErr(fmt.Errorf("sport is required\nUsage: boxscore <sport> <league>"))
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("league is required\nUsage: boxscore <sport> <league>"))
			}
			sport, league := args[0], args[1]

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Resolve event ID from --team if needed
			if !hasEvent && hasTeam {
				teamID, resolveErr := resolveTeamID(c, sport, league, flagTeam)
				if resolveErr != nil {
					return resolveErr
				}
				eventID, eventErr := resolveLastEventID(c, sport, league, teamID, flagLast)
				if eventErr != nil {
					return eventErr
				}
				flagEvent = eventID
			}

			// Fetch summary
			path := fmt.Sprintf("/%s/%s/summary", sport, league)
			params := map[string]string{"event": flagEvent}
			data, err := c.Get(path, params)
			if err != nil {
				return classifyAPIError(err)
			}

			// Parse box score
			boxScores := parseBoxScores(data)
			if len(boxScores) == 0 {
				return fmt.Errorf("no box score data found for event %s", flagEvent)
			}

			// JSON output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				filtered, _ := json.Marshal(boxScores)
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				}
				return printOutput(cmd.OutOrStdout(), filtered, true)
			}

			// Table output
			return printBoxScoreTable(cmd, boxScores)
		},
	}

	cmd.Flags().StringVar(&flagEvent, "event", "", "ESPN event or game ID")
	cmd.Flags().StringVar(&flagTeam, "team", "", "Team name, abbreviation, or ID (resolves most recent game)")
	cmd.Flags().IntVar(&flagLast, "last", 1, "Which completed game to show (1=most recent)")

	return cmd
}

func parseBoxScores(data json.RawMessage) []teamBoxScore {
	var summary struct {
		Boxscore struct {
			Players []struct {
				Team struct {
					DisplayName string `json:"displayName"`
				} `json:"team"`
				Statistics []struct {
					Labels   []string `json:"labels"`
					Athletes []struct {
						Athlete struct {
							DisplayName string `json:"displayName"`
						} `json:"athlete"`
						Stats []string `json:"stats"`
					} `json:"athletes"`
				} `json:"statistics"`
			} `json:"players"`
		} `json:"boxscore"`
	}
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil
	}

	var result []teamBoxScore
	for _, teamData := range summary.Boxscore.Players {
		tbs := teamBoxScore{Team: teamData.Team.DisplayName}
		if len(teamData.Statistics) == 0 {
			continue
		}
		labels := teamData.Statistics[0].Labels
		for _, athlete := range teamData.Statistics[0].Athletes {
			if len(athlete.Stats) == 0 {
				continue
			}
			ps := playerStat{
				Name:  athlete.Athlete.DisplayName,
				Stats: make(map[string]string),
			}
			for i, label := range labels {
				if i < len(athlete.Stats) {
					ps.Stats[label] = athlete.Stats[i]
				}
			}
			tbs.Players = append(tbs.Players, ps)
		}
		result = append(result, tbs)
	}
	return result
}

func printBoxScoreTable(cmd *cobra.Command, boxScores []teamBoxScore) error {
	w := cmd.OutOrStdout()
	// Columns to display in order
	columns := []string{"MIN", "PTS", "FG", "3PT", "FT", "REB", "AST", "TO", "STL", "BLK", "+/-"}

	for i, tbs := range boxScores {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, bold(tbs.Team))
		fmt.Fprintln(w, strings.Repeat("-", 90))

		// Header
		tw := newTabWriter(w)
		header := bold("PLAYER")
		for _, col := range columns {
			header += "\t" + bold(col)
		}
		fmt.Fprintln(tw, header)

		// Players
		for _, p := range tbs.Players {
			line := p.Name
			for _, col := range columns {
				val := p.Stats[col]
				if val == "" {
					val = "-"
				}
				line += "\t" + val
			}
			fmt.Fprintln(tw, line)
		}
		tw.Flush()
	}
	return nil
}
