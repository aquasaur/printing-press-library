// Hand-authored (not generated): wires the transcendence commands onto the
// generated command tree. Called from root.go's command builder after the
// spec-derived commands are registered.
package cli

import "github.com/spf13/cobra"

func attachNovelCommands(root *cobra.Command, flags *rootFlags) {
	for _, c := range root.Commands() {
		switch c.Name() {
		case "sites":
			c.AddCommand(newSitesHealthCmd(flags))
			c.AddCommand(newSitesTriageCmd(flags))
		case "traffic":
			c.AddCommand(newTrafficCtrGapsCmd(flags))
		case "keywords":
			c.AddCommand(newKeywordsCannibalizationCmd(flags))
		case "submit":
			c.AddCommand(newSubmitSmartCmd(flags))
		case "crawl":
			c.AddCommand(newCrawlTriageCmd(flags))
		case "url":
			c.AddCommand(newURLCheckCmd(flags))
		}
	}
}
