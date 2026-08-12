package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/service"
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

			resp, err := doRequest("PUT", "/api/v1/concepts/"+escapeConceptID(id), body)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}

			var result map[string]any
			if err := readJSON(resp, &result); err != nil {
				return err
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
			resp, err := doRequest("GET", "/api/v1/concepts/"+escapeConceptID(args[0]), nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var result map[string]any
			if err := readJSON(resp, &result); err != nil {
				return err
			}
			prettyPrint(result)
			return nil
		},
	}
}

func cmdDelete() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "删除一个 concept 及其 links/revisions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("删除 concept 需要显式传入 --yes")
			}
			id := args[0]
			resp, err := doRequest("DELETE", "/api/v1/concepts/"+escapeConceptID(id), nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			if _, err := readRaw(resp); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "concept %s 已删除\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "确认永久删除")
	return cmd
}

func escapeConceptID(id string) string {
	parts := strings.Split(id, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func cmdSearch() *cobra.Command {
	var domain, typeFilter, scenario, sort string
	var tags, filters []string
	var limit, offset int

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "搜索概念",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			if len(args) > 0 {
				params.Set("q", args[0])
			}
			if domain != "" {
				params.Set("domain", domain)
			}
			if typeFilter != "" {
				params.Set("type", typeFilter)
			}
			if scenario != "" {
				params.Set("scenario", scenario)
			}
			if sort != "" {
				params.Set("sort", sort)
			}
			for _, t := range tags {
				params.Add("tag", t)
			}
			// --filter sender=kjx → fm[sender]=kjx
			for _, f := range filters {
				parts := strings.SplitN(f, "=", 2)
				if len(parts) == 2 {
					params.Set("fm["+parts[0]+"]", parts[1])
				}
			}
			params.Set("limit", fmt.Sprintf("%d", limit))
			params.Set("offset", fmt.Sprintf("%d", offset))

			resp, err := doRequest("GET", "/api/v1/concepts?"+params.Encode(), nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var results []map[string]any
			if err := readJSON(resp, &results); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "共 %d 条结果\n", len(results))
			prettyPrint(results)
			return nil
		},
	}
	cmd.Flags().StringVarP(&domain, "domain", "d", "", "限定领域")
	cmd.Flags().StringVarP(&typeFilter, "type", "t", "", "限定类型")
	cmd.Flags().StringVar(&scenario, "scenario", "", "限定场景 (frontmatter.scenario)")
	cmd.Flags().StringVar(&sort, "sort", "", "排序: updated_at:desc|asc, date:desc|asc, title:asc")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "限定标签（可重复）")
	cmd.Flags().StringArrayVar(&filters, "filter", nil, "frontmatter 字段过滤，如 --filter sender=kjx --filter group=feishu-worldbuild")
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
				return err
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
			path := fmt.Sprintf("/api/v1/links/%s?limit=%d&offset=%d", escapeConceptID(args[0]), limit, offset)
			resp, err := doRequest("GET", path, nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var result map[string]any
			if err := readJSON(resp, &result); err != nil {
				return err
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
			params := url.Values{"out": {outDir}}
			path := "/api/v1/domains/" + url.PathEscape(args[0]) + "/export?" + params.Encode()
			resp, err := doRequest("GET", path, nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var result map[string]any
			if err := readJSON(resp, &result); err != nil {
				return err
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
区别于 API 的 L1 硬门禁：lint 完全在本地执行，只读不写，供提交前自检使用。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("读取文件失败: %w", err)
			}
			var concept model.Concept
			if err := json.Unmarshal(b, &concept); err != nil {
				return fmt.Errorf("JSON 解析失败: %w", err)
			}

			result := service.LintConcept(&concept)
			prettyPrint(result)
			if len(result.Errors) > 0 {
				return fmt.Errorf("lint 失败: %d 个错误", len(result.Errors))
			}
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
			params := url.Values{}
			params.Set("limit", fmt.Sprintf("%d", limit))
			params.Set("offset", fmt.Sprintf("%d", offset))
			if query != "" {
				params.Set("q", query)
			}
			resp, err := doRequest("GET", "/api/v1/domains?"+params.Encode(), nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var result []map[string]any
			if err := readJSON(resp, &result); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "共 %d 个领域\n", len(result))
			prettyPrint(result)
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 50, "返回数量上限")
	cmd.Flags().IntVar(&offset, "offset", 0, "分页偏移")
	cmd.Flags().StringVarP(&query, "query", "q", "", "搜索领域名")
	return cmd
}

func cmdDomain() *cobra.Command {
	var setFile, visibility string
	var deleteDomain, yes bool
	cmd := &cobra.Command{
		Use:   "domain <domain>",
		Short: "查看或设置 domain README 和 schema",
		Long: `查看或设置 domain 的 README 和 frontmatter schema。

无参数：打印 README（agent 可阅读）
--set readme.md：从文件更新 README 和 schema`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := args[0]
			path := "/api/v1/domains/" + url.PathEscape(domain)

			if deleteDomain {
				if setFile != "" || visibility != "" {
					return fmt.Errorf("--delete 不能与 --set 或 --visibility 同时使用")
				}
				if !yes {
					return fmt.Errorf("删除 domain 需要显式传入 --yes")
				}
				resp, err := doRequest("DELETE", path, nil)
				if err != nil {
					return fmt.Errorf("请求失败: %w", err)
				}
				if _, err := readRaw(resp); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "domain %s 及其全部数据已删除\n", domain)
				return nil
			}

			if setFile != "" {
				b, err := os.ReadFile(setFile)
				if err != nil {
					return fmt.Errorf("读取文件失败: %w", err)
				}
				body := map[string]any{"readme": string(b)}
				if visibility != "" {
					if visibility != "public" && visibility != "private" {
						return fmt.Errorf("--visibility 必须是 public 或 private")
					}
					body["visibility"] = visibility
				}
				resp, err := doRequest("PUT", path, body)
				if err != nil {
					return fmt.Errorf("请求失败: %w", err)
				}
				var result map[string]any
				if err := readJSON(resp, &result); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "✅ domain %s README 已更新\n", domain)
				prettyPrint(result)
				return nil
			}
			if visibility != "" {
				return fmt.Errorf("--visibility 只能与 --set 一起使用")
			}

			// 读取 README——直接打印 markdown，供 agent 阅读
			resp, err := doRequest("GET", path, nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var result map[string]any
			if err := readJSON(resp, &result); err != nil {
				return err
			}
			if readme, ok := result["readme"].(string); ok && readme != "" {
				if v, ok := result["visibility"].(string); ok {
					fmt.Fprintf(os.Stderr, "visibility: %s\n", v)
				}
				fmt.Println(readme)
			} else {
				fmt.Fprintf(os.Stderr, "domain %s 暂无 README\n", domain)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&setFile, "set", "", "从 markdown 文件更新 README")
	cmd.Flags().StringVar(&visibility, "visibility", "", "domain 可见性: public|private（仅与 --set 一起使用）")
	cmd.Flags().BoolVar(&deleteDomain, "delete", false, "永久删除 domain 及其 concepts/links/revisions")
	cmd.Flags().BoolVar(&yes, "yes", false, "确认永久删除")
	return cmd
}

func cmdSample() *cobra.Command {
	var domain, typeFilter string
	var limit int
	cmd := &cobra.Command{
		Use:   "sample",
		Short: "随机采样 concept，适合探索未知 domain 的数据结构",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			if domain != "" {
				params.Set("domain", domain)
			}
			if typeFilter != "" {
				params.Set("type", typeFilter)
			}
			params.Set("limit", fmt.Sprintf("%d", limit))
			resp, err := doRequest("GET", "/api/v1/concepts/sample?"+params.Encode(), nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var results []map[string]any
			if err := readJSON(resp, &results); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "采样 %d 条\n", len(results))
			prettyPrint(results)
			return nil
		},
	}
	cmd.Flags().StringVarP(&domain, "domain", "d", "", "限定领域")
	cmd.Flags().StringVarP(&typeFilter, "type", "t", "", "限定类型")
	cmd.Flags().IntVarP(&limit, "limit", "n", 5, "采样数量")
	return cmd
}

func cmdInvite() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "invite",
		Short: "domain 邀请码管理（host/admin）",
		Long: `管理 domain 邀请码。公开 domain 可邀请 writer；private domain 可邀请 reader 或 writer。

创建后只显示一次明文 code；对方打开固定 Work 链接后输入邀请码。
不依赖 Work 路由。`,
	}

	// okp invite create <domain>
	var expiresHours, maxUses int
	var role string
	createCmd := &cobra.Command{
		Use:   "create <domain>",
		Short: "创建 reader/writer 邀请码",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := args[0]
			body := map[string]any{
				"role":             role,
				"expires_in_hours": expiresHours,
				"max_uses":         maxUses,
			}
			resp, err := doRequest("POST", "/api/v1/domains/"+url.PathEscape(domain)+"/invites", body)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var result map[string]any
			if err := readJSON(resp, &result); err != nil {
				return err
			}
			if code, ok := result["code"].(string); ok {
				fmt.Fprintf(os.Stderr, "邀请码（只显示一次）: %s\n", code)
			}
			if share, ok := result["share_text"].(string); ok && share != "" {
				fmt.Fprintln(os.Stderr, "---")
				fmt.Fprintln(os.Stderr, share)
				fmt.Fprintln(os.Stderr, "---")
			}
			prettyPrint(result)
			return nil
		},
	}
	createCmd.Flags().StringVar(&role, "role", "writer", "邀请角色: writer；private domain 也可用 reader")
	createCmd.Flags().IntVar(&expiresHours, "expires-hours", 72, "过期小时数")
	createCmd.Flags().IntVar(&maxUses, "max-uses", 1, "最大使用次数")
	cmd.AddCommand(createCmd)

	// okp invite list <domain>
	listCmd := &cobra.Command{
		Use:   "list <domain>",
		Short: "列出 domain 邀请码（不含明文 code）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := args[0]
			resp, err := doRequest("GET", "/api/v1/domains/"+url.PathEscape(domain)+"/invites", nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var result []map[string]any
			if err := readJSON(resp, &result); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "共 %d 条邀请\n", len(result))
			prettyPrint(result)
			return nil
		},
	}
	cmd.AddCommand(listCmd)

	// okp invite revoke <domain> <id>
	revokeCmd := &cobra.Command{
		Use:   "revoke <domain> <invite-id>",
		Short: "撤销邀请码",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, id := args[0], args[1]
			path := "/api/v1/domains/" + url.PathEscape(domain) + "/invites/" + url.PathEscape(id)
			resp, err := doRequest("DELETE", path, nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var result map[string]any
			if err := readJSON(resp, &result); err != nil {
				return err
			}
			prettyPrint(result)
			return nil
		},
	}
	cmd.AddCommand(revokeCmd)

	// okp invite accept <code>
	acceptCmd := &cobra.Command{
		Use:   "accept <code>",
		Short: "接受邀请码（当前认证用户）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{"code": args[0]}
			resp, err := doRequest("POST", "/api/v1/invites/accept", body)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var result map[string]any
			if err := readJSON(resp, &result); err != nil {
				return err
			}
			if domain, ok := result["domain"].(string); ok {
				fmt.Fprintf(os.Stderr, "✅ 已加入 domain %s\n", domain)
			}
			prettyPrint(result)
			return nil
		},
	}
	cmd.AddCommand(acceptCmd)

	// okp invite members <domain>
	membersCmd := &cobra.Command{
		Use:   "members <domain>",
		Short: "列出 domain 成员（host/admin）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := args[0]
			resp, err := doRequest("GET", "/api/v1/domains/"+url.PathEscape(domain)+"/members", nil)
			if err != nil {
				return fmt.Errorf("请求失败: %w", err)
			}
			var result []map[string]any
			if err := readJSON(resp, &result); err != nil {
				return err
			}
			prettyPrint(result)
			return nil
		},
	}
	cmd.AddCommand(membersCmd)

	return cmd
}
