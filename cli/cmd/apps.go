package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type appResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	GitURL    string    `json:"git_url"`
	WebURL    string    `json:"web_url"`
	CreatedAt time.Time `json:"created_at"`
}

var appsCmd = &cobra.Command{
	Use:   "apps",
	Short: "Manage apps",
}

var appsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all apps",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		resp, err := client.do("GET", "/v1/apps", nil)
		if err != nil {
			return err
		}

		var apps []appResponse
		if err := client.decodeJSON(resp, &apps); err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tWEB URL\tCREATED")
		for _, a := range apps {
			fmt.Fprintf(w, "%s\t%s\t%s\n", a.Name, a.WebURL, a.CreatedAt.Format("2006-01-02"))
		}
		w.Flush()
		return nil
	},
}

var appsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		resp, err := client.do("POST", "/v1/apps", map[string]string{"name": args[0]})
		if err != nil {
			return err
		}

		var app appResponse
		if err := client.decodeJSON(resp, &app); err != nil {
			return err
		}

		fmt.Printf("Created app %s\n", app.Name)
		fmt.Printf("  Git URL: %s\n", app.GitURL)
		fmt.Printf("  Web URL: %s\n", app.WebURL)
		fmt.Printf("\nAdd the remote:\n  git remote add platform %s\n", app.GitURL)
		fmt.Printf("Deploy:\n  git push platform main\n")
		return nil
	},
}

var appsDestroyCmd = &cobra.Command{
	Use:   "destroy <name>",
	Short: "Destroy an app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm {
			return fmt.Errorf("this action is destructive — pass --confirm to proceed")
		}

		client := newClient()
		resp, err := client.do("DELETE", "/v1/apps/"+args[0], nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == 204 {
			fmt.Printf("Destroyed app %s\n", args[0])
			return nil
		}
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	},
}

func init() {
	appsDestroyCmd.Flags().Bool("confirm", false, "Confirm destruction")
	appsCmd.AddCommand(appsListCmd, appsCreateCmd, appsDestroyCmd)
	rootCmd.AddCommand(appsCmd)
}
