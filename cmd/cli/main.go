package main

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	apiBase  string
	apiToken string
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "okp",
		Short:   "Open Knowledge Protocol CLI",
		Version: version,
		Long: `okp is the CLI for Open Knowledge Protocol.
Create, search, and manage concepts across public and private domains.

环境变量：
  OKP_API_BASE    API 服务地址（默认 https://okp.neta.art）
  OKP_API_TOKEN   API 认证 token（可选；在 cohub sandbox 中自动使用 COHUB_EXECUTION_TOKEN）`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if apiBase == "" {
				apiBase = os.Getenv("OKP_API_BASE")
				if apiBase == "" {
					apiBase = "https://okp.neta.art"
				}
			}
			if apiToken == "" {
				apiToken = resolveToken()
			}
		},
	}

	rootCmd.PersistentFlags().StringVar(&apiBase, "api-base", "", "API 服务地址")
	rootCmd.PersistentFlags().StringVar(&apiToken, "token", "", "API 认证 token")

	rootCmd.AddCommand(cmdPut())
	rootCmd.AddCommand(cmdGet())
	rootCmd.AddCommand(cmdDelete())
	rootCmd.AddCommand(cmdSearch())
	rootCmd.AddCommand(cmdBatch())
	rootCmd.AddCommand(cmdLinks())
	rootCmd.AddCommand(cmdExport())
	rootCmd.AddCommand(cmdLint())
	rootCmd.AddCommand(cmdDomains())
	rootCmd.AddCommand(cmdDomain())
	rootCmd.AddCommand(cmdSample())
	rootCmd.AddCommand(cmdInvite())
	rootCmd.AddCommand(cmdAuth())
	rootCmd.AddCommand(cmdUpdate())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
