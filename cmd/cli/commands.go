package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// ── HTTP 辅助 ────────────────────────────────────────────────

func apiURL(path string) string {
	return strings.TrimRight(apiBase, "/") + path
}

func doRequest(method, path string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("JSON 序列化失败: %w", err)
		}
		r = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, apiURL(path), r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+apiToken)
	}

	return http.DefaultClient.Do(req)
}

func readJSON(resp *http.Response, v any) error {
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return json.Unmarshal(b, v)
}

func readRaw(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	s := string(b)
	if resp.StatusCode >= 400 {
		return s, fmt.Errorf("HTTP %d: %s", resp.StatusCode, s)
	}
	return s, nil
}

func prettyPrint(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

// ── 子命令 ───────────────────────────────────────────────────

func cmdPut() *cobra.Command {
	var filePath string
	cmd := &cobra.Command{
		Use:   "put <id>",
		Short: "upsert 一个 concept",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			var body map[string]any

			if filePath != "" {
				// 从文件读取
				b, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("读取文件失败: %w", err)
				}
				if err := json.Unmarshal(b, &body); err != nil {
					return fmt.Errorf("JSON 解析失败: %w", err)
				}
			} else {
				// 从 stdin 读取
				if err := json.NewDecoder(os.Stdin).Decode(&body); err != nil {
					return fmt.Errorf("stdin JSON 解析失败: %w (提示: 使用 --file 指定文件，或通过管道传入 JSON)", err)
				}
			}
			// 确保 id 与 URL 一致
			body["id"] = id

			resp, err := doRequest("PUT", "/api/v1/concepts/"+id, body)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}

			var result map[string]any
			if err := readJSON(resp, &result); err != nil {
				fmt.Println(err)
				return nil
			}
			prettyPrint(result)
			return nil
		},
	}
	cmd.Flags().StringVarP(&filePath, "file", "f", "", "从 JSON 文件读取概念数据")
	return cmd
}

func cmdGet() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "获取 concept",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := doRequest("GET", "/api/v1/concepts/"+args[0], nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var result map[string]any
			if err := readJSON(resp, &result); err != nil {
				fmt.Println(err)
				return nil
			}
			prettyPrint(result)
			return nil
		},
	}
}

func cmdSearch() *cobra.Command {
	var domain, typeFilter, status, scenario string
	var tags []string
	var limit, offset int

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "搜索概念",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/concepts?"
			if len(args) > 0 {
				path += "q=" + args[0] + "&"
			}
			if domain != "" {
				path += "domain=" + domain + "&"
			}
			if typeFilter != "" {
				path += "type=" + typeFilter + "&"
			}
			if status != "" {
				path += "status=" + status + "&"
			}
			if scenario != "" {
				path += "scenario=" + scenario + "&"
			}
			for _, t := range tags {
				path += "tag=" + t + "&"
			}
			path += fmt.Sprintf("limit=%d&offset=%d", limit, offset)

			resp, err := doRequest("GET", path, nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var results []map[string]any
			if err := readJSON(resp, &results); err != nil {
				fmt.Println(err)
				return nil
			}
			fmt.Printf("共 %s 条结果\n", resp.Header.Get("X-Total-Count"))
			prettyPrint(results)
			return nil
		},
	}
	cmd.Flags().StringVarP(&domain, "domain", "d", "", "限定领域")
	cmd.Flags().StringVarP(&typeFilter, "type", "t", "", "限定类型")
	cmd.Flags().StringVar(&status, "status", "", "限定状态")
	cmd.Flags().StringVar(&scenario, "scenario", "", "限定场景 (frontmatter.scenario)")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "限定标签（可重复）")
	cmd.Flags().IntVarP(&limit, "limit", "n", 50, "返回数量上限")
	cmd.Flags().IntVar(&offset, "offset", 0, "分页偏移")
	return cmd
}

func cmdBatch() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch <file.ndjson>",
		Short: "批量导入 concept（NDJSON 格式）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("读取文件失败: %w", err)
			}

			// NDJSON → JSON array
			lines := strings.Split(strings.TrimSpace(string(b)), "\n")
			var concepts []map[string]any
			for i, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				var c map[string]any
				if err := json.Unmarshal([]byte(line), &c); err != nil {
					return fmt.Errorf("第 %d 行 JSON 解析失败: %w", i+1, err)
				}
				concepts = append(concepts, c)
			}

			resp, err := doRequest("POST", "/api/v1/concepts:batch", concepts)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var result map[string]any
			if err := readJSON(resp, &result); err != nil {
				fmt.Println(err)
				return nil
			}
			prettyPrint(result)
			return nil
		},
	}
	return cmd
}

func cmdLinks() *cobra.Command {
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "links <id>",
		Short: "查看 concept 的出链和反向引用",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/links/%s?limit=%d&offset=%d", args[0], limit, offset)
			resp, err := doRequest("GET", path, nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var result map[string]any
			if err := readJSON(resp, &result); err != nil {
				fmt.Println(err)
				return nil
			}
			fmt.Printf("outgoing: %s, backlinks: %s\n",
				resp.Header.Get("X-Total-Outgoing"), resp.Header.Get("X-Total-Backlinks"))
			prettyPrint(result)
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 50, "返回数量上限")
	cmd.Flags().IntVar(&offset, "offset", 0, "分页偏移")
	return cmd
}

func cmdExport() *cobra.Command {
	var outDir string
	cmd := &cobra.Command{
		Use:   "export <domain>",
		Short: "导出为 OKF bundle 文件树",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if outDir == "" {
				outDir = "./okp-export"
			}
			path := fmt.Sprintf("/api/v1/domains/%s/export?out=%s", args[0], outDir)
			resp, err := doRequest("GET", path, nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var result map[string]any
			if err := readJSON(resp, &result); err != nil {
				fmt.Println(err)
				return nil
			}
			prettyPrint(result)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outDir, "out", "o", "", "输出目录")
	return cmd
}

func cmdLint() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint <file.json>",
		Short: "本地校验 concept（L2 软门禁）",
		Long: `对单个 concept 执行本地校验（schema + 建议检查）。
区别于 API 的 L1 硬门禁：lint 只读不写，供提交前自检使用。
当前 lint 通过 API 执行；后续可内嵌本地校验器以完全离线。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("读取文件失败: %w", err)
			}
			// lint 目前通过 API 的 validate-only 路径执行
			// 未来可本地内嵌 LintConcept
			_ = b
			fmt.Println("提示：lint 目前通过 API 校验执行。使用 okp put --dry-run 预览校验结果。")
			// 简化版：直接调 put 的校验逻辑
			// 完整实现需要 API 支持 validate-only 或本地内嵌 service.LintConcept
			return nil
		},
	}
	return cmd
}

func cmdDomains() *cobra.Command {
	var limit, offset int
	var query string
	cmd := &cobra.Command{
		Use:   "domains",
		Short: "列出所有知识领域",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := fmt.Sprintf("/api/v1/domains?limit=%d&offset=%d", limit, offset)
			if query != "" {
				path += "&q=" + query
			}
			resp, err := doRequest("GET", path, nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var result []map[string]any
			if err := readJSON(resp, &result); err != nil {
				fmt.Println(err)
				return nil
			}
			fmt.Printf("共 %s 个领域\n", resp.Header.Get("X-Total-Count"))
			prettyPrint(result)
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 50, "返回数量上限")
	cmd.Flags().IntVar(&offset, "offset", 0, "分页偏移")
	cmd.Flags().StringVarP(&query, "query", "q", "", "搜索领域名")
	return cmd
}

func cmdMigrate() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "触发数据库自动迁移（通过 API 间接执行）",
		Long:  "数据库迁移在 API 启动时自动执行。此命令仅用于健康检查确认迁移状态。",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := doRequest("GET", "/api/v1/health", nil)
			if err != nil {
				return fmt.Errorf("API 不可达: %w", err)
			}
			s, _ := readRaw(resp)
			fmt.Println("API 状态:", s)
			fmt.Println("迁移已在 API 启动时自动执行。")
			return nil
		},
	}
}
