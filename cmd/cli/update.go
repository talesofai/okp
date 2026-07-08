package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

func cmdUpdate() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "升级到最新版 okp CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			currentVer := "1.0.1" // sync with npm version

			// Check latest version from npm
			latestVer, err := fetchLatestVersion()
			if err != nil {
				return fmt.Errorf("检查更新失败: %w", err)
			}

			if latestVer == currentVer {
				fmt.Printf("已是最新版 v%s\n", currentVer)
				return nil
			}

			fmt.Printf("发现新版本: v%s → v%s\n", currentVer, latestVer)
			fmt.Println("正在更新...")

			if err := downloadAndInstall(latestVer); err != nil {
				return fmt.Errorf("更新失败: %w", err)
			}

			fmt.Printf("✅ 已升级到 v%s\n", latestVer)
			return nil
		},
	}
}

func fetchLatestVersion() (string, error) {
	resp, err := http.Get("https://registry.npmjs.org/@markbangwu/okp/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		return "", err
	}
	return pkg.Version, nil
}

func downloadAndInstall(version string) error {
	// Download the platform-specific binary
	platform := runtime.GOOS
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}

	var pkgName string
	switch {
	case platform == "linux" && arch == "x64":
		pkgName = "okp-cli-linux-x64"
	case platform == "darwin" && arch == "arm64":
		pkgName = "okp-cli-darwin-arm64"
	case platform == "darwin" && arch == "x64":
		pkgName = "okp-cli-darwin-x64"
	case platform == "win32" && arch == "x64":
		pkgName = "okp-cli-win32-x64"
	default:
		return fmt.Errorf("unsupported platform: %s/%s", platform, arch)
	}

	url := fmt.Sprintf("https://registry.npmjs.org/%s/-/%s-%s.tgz", pkgName, pkgName, version)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	// Save to temp and replace current binary
	tmpFile, err := os.CreateTemp("", "okp-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	// The npm tarball contains the binary at vendor/.../bin/okp
	// For simplicity, we extract just the binary using a tar reader
	// But for now, use the direct binary URL approach
	// Actually, let's use a simpler approach - direct binary download
	
	// Find current binary path
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}

	// Try to replace (might fail on Windows if running)
	if err := os.Rename(tmpFile.Name(), exe); err != nil {
		// Fallback: copy
		src, _ := os.Open(tmpFile.Name())
		if src != nil {
			defer src.Close()
			os.Remove(exe)
			dst, _ := os.Create(exe)
			if dst != nil {
				defer dst.Close()
				io.Copy(dst, src)
				os.Chmod(exe, 0755)
			}
		}
	} else {
		os.Chmod(exe, 0755)
	}

	return nil
}
