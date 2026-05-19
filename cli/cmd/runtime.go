package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type runtimeResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Cloud     string    `json:"cloud"`
	Region    string    `json:"region"`
	Status    string    `json:"status"`
	LastSeen  time.Time `json:"last_seen"`
	AgentURL  string    `json:"agent_url"`
}

// timeAgo returns a human-readable relative time string (e.g. "2 minutes ago").
func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		secs := int(d.Seconds())
		if secs <= 1 {
			return "1 second ago"
		}
		return fmt.Sprintf("%d seconds ago", secs)
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

var runtimeCmd = &cobra.Command{
	Use:   "runtime",
	Short: "Manage runtime planes",
}

var runtimeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered runtime planes",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		resp, err := client.do("GET", "/v1/runtimes", nil)
		if err != nil {
			return err
		}

		var runtimes []runtimeResponse
		if err := client.decodeJSON(resp, &runtimes); err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tCLOUD\tREGION\tSTATUS\tLAST SEEN")
		for _, r := range runtimes {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				r.Name, r.Cloud, r.Region, r.Status, timeAgo(r.LastSeen))
		}
		w.Flush()
		return nil
	},
}

var runtimeRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Manually register a runtime plane",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		cloud, _ := cmd.Flags().GetString("cloud")
		region, _ := cmd.Flags().GetString("region")
		agentURL, _ := cmd.Flags().GetString("agent-url")

		if name == "" {
			return fmt.Errorf("--name is required")
		}
		if cloud == "" {
			return fmt.Errorf("--cloud is required")
		}
		if region == "" {
			return fmt.Errorf("--region is required")
		}
		if agentURL == "" {
			return fmt.Errorf("--agent-url is required")
		}

		client := newClient()
		resp, err := client.do("POST", "/internal/runtime/register", map[string]string{
			"name":      name,
			"cloud":     cloud,
			"region":    region,
			"agent_url": agentURL,
		})
		if err != nil {
			return err
		}

		var result runtimeResponse
		if err := client.decodeJSON(resp, &result); err != nil {
			return err
		}

		fmt.Printf("Registered runtime %s (id: %s)\n", result.Name, result.ID)
		return nil
	},
}

var runtimeDeregisterCmd = &cobra.Command{
	Use:   "deregister <name>",
	Short: "Remove a runtime plane",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Not yet implemented. Remove the runtime agent pod from your cluster to deregister.")
		return nil
	},
}

func init() {
	runtimeRegisterCmd.Flags().String("name", "", "Runtime name (e.g. aws-us-east-1)")
	runtimeRegisterCmd.Flags().String("cloud", "", "Cloud provider: aws, gcp, azure, or local")
	runtimeRegisterCmd.Flags().String("region", "", "Cloud region (e.g. us-east-1)")
	runtimeRegisterCmd.Flags().String("agent-url", "", "URL of the runtime agent")

	runtimeCmd.AddCommand(runtimeListCmd, runtimeRegisterCmd, runtimeDeregisterCmd)
	rootCmd.AddCommand(runtimeCmd)
}
