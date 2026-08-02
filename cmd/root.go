package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"zomboid-manager/internal/bot"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "zomboid-manager",
	Short: "A lightweight Discord bot for managing a Project Zomboid server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := bot.Config{
			Token:          viper.GetString("token"),
			GuildID:        viper.GetString("guild-id"),
			ServiceName:    viper.GetString("service-name"),
			AllowedUserIDs: splitAndTrim(viper.GetString("allowed-user-ids")),
			JournalctlSudo: viper.GetBool("journalctl-sudo"),
			IPv4Only:       viper.GetBool("ipv4-only"),
		}

		if cfg.Token == "" {
			return fmt.Errorf("token is required (set --token, ZM_TOKEN, or token in config file)")
		}
		if len(cfg.AllowedUserIDs) == 0 {
			return fmt.Errorf("allowed-user-ids is required (set --allowed-user-ids, ZM_ALLOWED_USER_IDS, or allowed-user-ids in config file)")
		}

		return bot.Run(cfg)
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./zomboid-manager.yaml)")

	rootCmd.Flags().String("token", "", "Discord bot token")
	rootCmd.Flags().String("guild-id", "", "Discord guild ID for instant command registration (optional; global registration is used if empty, which can take up to an hour to propagate)")
	rootCmd.Flags().String("service-name", "pzserver", "systemd service name for the Zomboid server")
	rootCmd.Flags().String("allowed-user-ids", "", "comma-separated Discord user IDs allowed to run commands")
	rootCmd.Flags().Bool("journalctl-sudo", false, "run journalctl with sudo (requires a sudoers entry); prefer adding the bot's user to the systemd-journal group instead")
	rootCmd.Flags().Bool("ipv4-only", true, "connect to Discord over IPv4 only; avoids multi-second stalls on hosts with a broken/black-holed IPv6 route")

	viper.BindPFlag("token", rootCmd.Flags().Lookup("token"))
	viper.BindPFlag("guild-id", rootCmd.Flags().Lookup("guild-id"))
	viper.BindPFlag("service-name", rootCmd.Flags().Lookup("service-name"))
	viper.BindPFlag("allowed-user-ids", rootCmd.Flags().Lookup("allowed-user-ids"))
	viper.BindPFlag("journalctl-sudo", rootCmd.Flags().Lookup("journalctl-sudo"))
	viper.BindPFlag("ipv4-only", rootCmd.Flags().Lookup("ipv4-only"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("zomboid-manager")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
	}

	viper.SetEnvPrefix("zm")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "using config file:", viper.ConfigFileUsed())
	}
}

func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
