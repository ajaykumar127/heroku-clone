package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "platform",
	Short: "Platform CLI — deploy apps like Heroku",
	Long:  `platform is the CLI for the cloud-agnostic PaaS platform.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.platform/config)")
	rootCmd.PersistentFlags().StringP("api-url", "u", "http://localhost:8080", "Platform API URL")
	rootCmd.PersistentFlags().StringP("token", "t", "", "API token (or set PLATFORM_TOKEN env var)")
	viper.BindPFlag("api_url", rootCmd.PersistentFlags().Lookup("api-url"))
	viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, _ := os.UserHomeDir()
		viper.AddConfigPath(home + "/.platform")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}
	viper.SetEnvPrefix("PLATFORM")
	viper.AutomaticEnv()
	viper.ReadInConfig()
}

func apiURL() string  { return viper.GetString("api_url") }
func apiToken() string { return viper.GetString("token") }
