package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage app config vars",
}

var configGetCmd = &cobra.Command{
	Use:   "get <app>",
	Short: "Show all config vars for an app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		resp, err := client.do("GET", "/v1/apps/"+args[0]+"/config-vars", nil)
		if err != nil {
			return err
		}

		var vars map[string]string
		if err := client.decodeJSON(resp, &vars); err != nil {
			return err
		}

		keys := make([]string, 0, len(vars))
		for k := range vars {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, k := range keys {
			fmt.Fprintf(w, "%s:\t%s\n", k, vars[k])
		}
		w.Flush()
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <app> KEY=VALUE [KEY=VALUE...]",
	Short: "Set one or more config vars",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		pairs := args[1:]

		body := map[string]*string{}
		for _, p := range pairs {
			parts := strings.SplitN(p, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid format %q — expected KEY=VALUE", p)
			}
			v := parts[1]
			body[parts[0]] = &v
		}

		client := newClient()
		resp, err := client.do("PATCH", "/v1/apps/"+appName+"/config-vars", body)
		if err != nil {
			return err
		}

		var vars map[string]string
		if err := client.decodeJSON(resp, &vars); err != nil {
			return err
		}

		fmt.Printf("Config vars updated for %s\n", appName)
		for _, p := range pairs {
			key := strings.SplitN(p, "=", 2)[0]
			fmt.Printf("  %s = %s\n", key, vars[key])
		}
		return nil
	},
}

var configUnsetCmd = &cobra.Command{
	Use:   "unset <app> KEY [KEY...]",
	Short: "Remove one or more config vars",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		keys := args[1:]

		body := map[string]*string{}
		for _, k := range keys {
			body[k] = nil
		}

		client := newClient()
		resp, err := client.do("PATCH", "/v1/apps/"+appName+"/config-vars", body)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		fmt.Printf("Unset %s for %s\n", strings.Join(keys, ", "), appName)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configGetCmd, configSetCmd, configUnsetCmd)
	rootCmd.AddCommand(configCmd)
}
