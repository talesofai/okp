package main

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	apiBase string
	apiToken string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "okp",
		Short: "Open Knowledge Pool — 捏Ta 统一知识库 CLI",
		Long: `okp 是 Open Knowledge Pool 的命令行工具。
管理知识池中的 concept：写入、查询、导出、校验。

环境变量：
  OKP_API_BASE    API 服务地址（默认 http://localhost:8720）
  OKP_API_TOKEN   API 认证 token`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if apiBase == "" {
				apiBase = os.Getenv("OKP_API_BASE")
				if apiBase == "" {
					apiBase = "http://localhost:8720"
				}
			}
			if apiToken == "" {
				apiToken = os.Getenv("OKP_API_TOKEN")
			}
		},
	}

	rootCmd.PersistentFlags().StringVar(&apiBase, "api-base", "", "API 服务地址")
	rootCmd.PersistentFlags().StringVar(&apiToken, "token", "", "API 认证 token")

	rootCmd.AddCommand(cmdPut())
	rootCmd.AddCommand(cmdGet())
	rootCmd.AddCommand(cmdSearch())
	rootCmd.AddCommand(cmdBatch())
	rootCmd.AddCommand(cmdLinks())
	rootCmd.AddCommand(cmdExport())
	rootCmd.AddCommand(cmdLint())
	rootCmd.AddCommand(cmdDomains())
	rootCmd.AddCommand(cmdMigrate())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
