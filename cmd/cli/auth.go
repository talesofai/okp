package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ── Auth config ────────────────────────────────────────────

type authConfig struct {
	Issuer   string
	ClientID string
	Resource string
	Scope    string
}

func currentAuthConfig() authConfig {
	return authConfig{
		Issuer:   envDefault("OKP_AUTH_ISSUER", "https://auth.neta.art"),
		ClientID: envDefault("OKP_AUTH_CLIENT_ID", "f8d26cdlwx85b0e5l3om2"),
		Resource: envDefault("OKP_AUTH_RESOURCE", "https://api.talesofai"),
		Scope:    envDefault("OKP_AUTH_SCOPE", "openid profile email offline_access"),
	}
}

// ── Token storage ──────────────────────────────────────────

type authSession struct {
	SchemaVersion        int    `json:"schemaVersion"`
	Issuer               string `json:"issuer"`
	ClientID             string `json:"clientId"`
	Resource             string `json:"resource"`
	Scope                string `json:"scope"`
	TokenType            string `json:"tokenType"`
	AccessToken          string `json:"accessToken"`
	RefreshToken         string `json:"refreshToken"`
	IDToken              string `json:"idToken,omitempty"`
	AccessTokenExpiresAt int64  `json:"accessTokenExpiresAt"`
	CreatedAt            int64  `json:"createdAt"`
	UpdatedAt            int64  `json:"updatedAt"`
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "okp")
}

func sessionPath() string {
	return filepath.Join(configDir(), "auth.json")
}

func readSession() *authSession {
	data, err := os.ReadFile(sessionPath())
	if err != nil {
		return nil
	}
	var s authSession
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	return &s
}

func writeSession(s *authSession) {
	os.MkdirAll(configDir(), 0700)
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(sessionPath(), data, 0600)
}

func clearSession() {
	os.Remove(sessionPath())
}

// ── Token helpers ──────────────────────────────────────────

func resolveToken() string {
	if t := os.Getenv("OKP_API_TOKEN"); t != "" {
		return t
	}
	if t := os.Getenv("COHUB_EXECUTION_TOKEN"); t != "" {
		return t
	}
	// Try reading okp auth session
	if s := readSession(); s != nil && s.AccessToken != "" {
		// Check expiry (5 min buffer)
		if time.Now().UnixMilli() < s.AccessTokenExpiresAt-5*60*1000 {
			return s.AccessToken
		}
		// Try refresh
		if tok, err := refreshToken(s); err == nil {
			return tok
		}
	}
	// Fallback: try cohub auth session
	if t := readCohubToken(); t != "" {
		return t
	}
	return ""
}

func readCohubToken() string {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".config", "cohub", "auth.json"))
	if err != nil {
		return ""
	}
	var s struct {
		AccessToken          string `json:"accessToken"`
		AccessTokenExpiresAt int64  `json:"accessTokenExpiresAt"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return ""
	}
	if s.AccessToken != "" && time.Now().UnixMilli() < s.AccessTokenExpiresAt-5*60*1000 {
		return s.AccessToken
	}
	return ""
}

// ── Device flow ────────────────────────────────────────────

type deviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func formPost(u string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequest("POST", u, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return http.DefaultClient.Do(req)
}

func refreshToken(s *authSession) (string, error) {
	cfg := currentAuthConfig()
	resp, err := formPost(cfg.Issuer+"/oidc/token", url.Values{
		"client_id":     {s.ClientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {s.RefreshToken},
		"scope":         {s.Scope},
		"resource":      {s.Resource},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	if tr.Error != "" {
		clearSession()
		return "", fmt.Errorf("%s: %s", tr.Error, tr.ErrorDesc)
	}

	now := time.Now().UnixMilli()
	s.AccessToken = tr.AccessToken
	s.RefreshToken = tr.RefreshToken
	s.AccessTokenExpiresAt = now + int64(tr.ExpiresIn)*1000
	s.UpdatedAt = now
	writeSession(s)
	return s.AccessToken, nil
}

// ── auth commands ──────────────────────────────────────────

func cmdAuth() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "认证管理",
	}
	cmd.AddCommand(cmdAuthLogin())
	cmd.AddCommand(cmdAuthWhoami())
	cmd.AddCommand(cmdAuthLogout())
	return cmd
}

func cmdAuthLogin() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "通过浏览器登录（device flow）",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := currentAuthConfig()

			// 1. Request device code
			resp, err := formPost(cfg.Issuer+"/oidc/device/auth", url.Values{
				"client_id": {cfg.ClientID},
				"scope":     {cfg.Scope},
				"resource":  {cfg.Resource},
			})
			if err != nil {
				return fmt.Errorf("请求 device code 失败: %w", err)
			}
			defer resp.Body.Close()

			var dc deviceCode
			if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
				return fmt.Errorf("解析 device code 失败: %w", err)
			}

			// 2. Show user the URL
			fmt.Printf("\n请打开浏览器访问:\n\n  %s\n\n", dc.VerificationURIComplete)
			fmt.Printf("然后输入验证码: %s\n\n", dc.UserCode)
			fmt.Println("等待授权...")

			// 3. Poll for token
			interval := dc.Interval
			if interval < 5 {
				interval = 5
			}
			deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)

			var lastErr error
			for time.Now().Before(deadline) {
				time.Sleep(time.Duration(interval) * time.Second)

				tokenResp, err := formPost(cfg.Issuer+"/oidc/token", url.Values{
					"client_id":   {cfg.ClientID},
					"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
					"device_code": {dc.DeviceCode},
					"resource":    {cfg.Resource},
				})
				if err != nil {
					lastErr = err
					continue
				}

				var tr tokenResponse
				if err := json.NewDecoder(tokenResp.Body).Decode(&tr); err != nil {
					tokenResp.Body.Close()
					lastErr = err
					continue
				}
				tokenResp.Body.Close()

				if tr.Error == "authorization_pending" {
					continue
				}
				if tr.Error == "slow_down" {
					interval += 5
					continue
				}
				if tr.Error != "" {
					return fmt.Errorf("%s: %s", tr.Error, tr.ErrorDesc)
				}

				// Success!
				now := time.Now().UnixMilli()
				s := authSession{
					SchemaVersion:        1,
					Issuer:               cfg.Issuer,
					ClientID:             cfg.ClientID,
					Resource:             cfg.Resource,
					Scope:                tr.Scope,
					TokenType:            "Bearer",
					AccessToken:          tr.AccessToken,
					RefreshToken:         tr.RefreshToken,
					IDToken:              tr.IDToken,
					AccessTokenExpiresAt: now + int64(tr.ExpiresIn)*1000,
					CreatedAt:            now,
					UpdatedAt:            now,
				}
				writeSession(&s)
				fmt.Println("\n✅ 登录成功！")
				return nil
			}

			if lastErr != nil {
				return fmt.Errorf("登录超时: %w", lastErr)
			}
			return fmt.Errorf("登录超时，请重试")
		},
	}
}

func cmdAuthWhoami() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "显示当前用户信息",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Try execution token first
			if t := os.Getenv("COHUB_EXECUTION_TOKEN"); t != "" {
				fmt.Println("认证方式: execution grant (cohub sandbox)")
				fmt.Printf("user: %s\n", os.Getenv("COHUB_USER_UUID"))
				return nil
			}

			s := readSession()
			if s == nil {
				fmt.Println("未登录。运行 okp auth login 登录。")
				return nil
			}

			// Try to decode ID token
			if s.IDToken != "" {
				parts := strings.Split(s.IDToken, ".")
				if len(parts) >= 2 {
					payload, _ := base64Decode(parts[1])
					var claims map[string]any
					json.Unmarshal(payload, &claims)
					fmt.Printf("认证方式: logto (device flow)\n")
					if name, ok := claims["nick_name"]; ok {
						fmt.Printf("昵称: %v\n", name)
					}
					if email, ok := claims["email"]; ok {
						fmt.Printf("邮箱: %v\n", email)
					}
					if uuid, ok := claims["talesofai_uuid"]; ok {
						fmt.Printf("UUID: %v\n", uuid)
					}
					fmt.Printf("token 过期: %s\n", time.UnixMilli(s.AccessTokenExpiresAt).Format(time.RFC3339))
					return nil
				}
			}
			fmt.Println("认证方式: logto (device flow)")
			fmt.Printf("token 过期: %s\n", time.UnixMilli(s.AccessTokenExpiresAt).Format(time.RFC3339))
			return nil
		},
	}
}

func cmdAuthLogout() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "退出登录",
		RunE: func(cmd *cobra.Command, args []string) error {
			clearSession()
			fmt.Println("已退出登录。")
			return nil
		},
	}
}

func base64Decode(s string) ([]byte, error) {
	// Add padding
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	var result []byte
	_, err := fmt.Sscanf(s, "%s", &result)
	if err != nil {
		// Manual decode
		decode := make([]byte, len(s)*3/4)
		n, err := base64DecodeRaw(s, decode)
		return decode[:n], err
	}
	return result, nil
}

func base64DecodeRaw(s string, dst []byte) (int, error) {
	// Simple base64url decoder
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var buf uint32
	var nbuf, n int
	for _, c := range s {
		if c == '=' {
			break
		}
		idx := strings.IndexRune(alphabet, c)
		if idx < 0 {
			continue
		}
		buf = buf<<6 | uint32(idx)
		nbuf++
		if nbuf == 4 {
			dst[n] = byte(buf >> 16)
			dst[n+1] = byte(buf >> 8)
			dst[n+2] = byte(buf)
			n += 3
			nbuf = 0
		}
	}
	if nbuf == 3 {
		dst[n] = byte(buf >> 10)
		dst[n+1] = byte(buf >> 2)
		n += 2
	}
	if nbuf == 2 {
		dst[n] = byte(buf >> 4)
		n++
	}
	return n, nil
}

func envDefault(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}
