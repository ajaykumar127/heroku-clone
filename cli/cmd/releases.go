package cmd

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type releaseResponse struct {
	ID          string    `json:"id"`
	AppID       string    `json:"app_id"`
	Version     int       `json:"version"`
	Commit      string    `json:"commit"`
	Status      string    `json:"status"`
	BuildOutput string    `json:"build_output,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

var releasesCmd = &cobra.Command{
	Use:   "releases",
	Short: "Manage app releases",
}

var releasesListCmd = &cobra.Command{
	Use:   "list <app>",
	Short: "List releases for an app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		resp, err := client.do("GET", "/v1/apps/"+args[0]+"/releases", nil)
		if err != nil {
			return err
		}

		var releases []releaseResponse
		if err := client.decodeJSON(resp, &releases); err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "VERSION\tCOMMIT\tSTATUS\tCREATED")
		for _, r := range releases {
			fmt.Fprintf(w, "v%d\t%s\t%s\t%s\n",
				r.Version, r.Commit, r.Status, r.CreatedAt.Format("2006-01-02 15:04:05"))
		}
		w.Flush()
		return nil
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs <app> <release-id>",
	Short: "Stream build logs for a release",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		releaseID := args[1]
		stream, _ := cmd.Flags().GetBool("tail")

		client := newClient()

		if stream {
			return streamLogs(client, appName, releaseID)
		}

		// Snapshot
		resp, err := client.do("GET", fmt.Sprintf("/v1/apps/%s/releases/%s/logs", appName, releaseID), nil)
		if err != nil {
			return err
		}
		var result struct {
			Status string `json:"status"`
			Output string `json:"output"`
		}
		if err := client.decodeJSON(resp, &result); err != nil {
			return err
		}
		fmt.Printf("Status: %s\n\n%s\n", result.Status, result.Output)
		return nil
	},
}

func streamLogs(client *apiClient, appName, releaseID string) error {
	url := fmt.Sprintf("%s/v1/apps/%s/releases/%s/logs/stream", client.baseURL, appName, releaseID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	if client.token != "" {
		req.Header.Set("Authorization", "Bearer "+client.token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("stream request failed: %w", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			fmt.Println(strings.TrimPrefix(line, "data: "))
		} else if strings.HasPrefix(line, "event: done") {
			// next line has the status
			scanner.Scan()
			status := strings.TrimPrefix(scanner.Text(), "data: ")
			fmt.Printf("\n==> Build %s\n", status)
			return nil
		}
	}
	return scanner.Err()
}

func init() {
	logsCmd.Flags().BoolP("tail", "f", false, "Stream logs in real-time")
	releasesCmd.AddCommand(releasesListCmd)
	rootCmd.AddCommand(releasesCmd, logsCmd)
}
