package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type tokenResponse struct {
	ID        string    `json:"id"`
	Token     string    `json:"token,omitempty"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage API authentication tokens",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Create a new API token and save it to ~/.platform/config",
	RunE: func(cmd *cobra.Command, args []string) error {
		comment, _ := cmd.Flags().GetString("comment")

		client := newClient()
		resp, err := client.do("POST", "/v1/auth/tokens", map[string]string{"comment": comment})
		if err != nil {
			return err
		}

		var t tokenResponse
		if err := client.decodeJSON(resp, &t); err != nil {
			return err
		}

		if err := saveToken(t.Token); err != nil {
			return fmt.Errorf("token created but failed to save: %w\nToken: %s", err, t.Token)
		}

		fmt.Printf("Logged in. Token saved to ~/.platform/config\n")
		fmt.Printf("Token: %s\n", t.Token)
		return nil
	},
}

var authTokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "List all API tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		resp, err := client.do("GET", "/v1/auth/tokens", nil)
		if err != nil {
			return err
		}

		var tokens []tokenResponse
		if err := client.decodeJSON(resp, &tokens); err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tCOMMENT\tCREATED")
		for _, t := range tokens {
			fmt.Fprintf(w, "%s\t%s\t%s\n", t.ID[:8]+"...", t.Comment, t.CreatedAt.Format("2006-01-02"))
		}
		w.Flush()
		return nil
	},
}

func saveToken(token string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".platform")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	cfgPath := filepath.Join(dir, "config")
	cfg := map[string]string{
		"token":   token,
		"api_url": apiURL(),
	}

	f, err := os.OpenFile(cfgPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return yaml.NewEncoder(f).Encode(cfg)
}

func init() {
	authLoginCmd.Flags().String("comment", "cli", "Token description")
	authCmd.AddCommand(authLoginCmd, authTokensCmd)
	rootCmd.AddCommand(authCmd)
}
