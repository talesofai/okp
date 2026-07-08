package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// version 在构建时通过 -ldflags "-X main.version=x.y.z" 注入
var version = "dev"

func cmdUpdate() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "升级到最新版 okp CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			latestVer, err := fetchLatestVersion()
			if err != nil {
				return fmt.Errorf("检查更新失败: %w", err)
			}

			if latestVer == version {
				fmt.Printf("已是最新版 v%s\n", version)
				return nil
			}

			fmt.Printf("发现新版本: v%s → v%s\n", version, latestVer)
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

func downloadAndInstall(ver string) error {
	platform := runtime.GOOS
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}

	var pkgName, triple string
	switch {
	case platform == "linux" && arch == "x64":
		pkgName, triple = "okp-cli-linux-x64", "x86_64-unknown-linux-musl"
	case platform == "linux" && arch == "arm64":
		pkgName, triple = "okp-cli-linux-arm64", "aarch64-unknown-linux-musl"
	case platform == "darwin" && arch == "arm64":
		pkgName, triple = "okp-cli-darwin-arm64", "aarch64-apple-darwin"
	case platform == "darwin" && arch == "x64":
		pkgName, triple = "okp-cli-darwin-x64", "x86_64-apple-darwin"
	default:
		return fmt.Errorf("unsupported platform: %s/%s", platform, arch)
	}

	// 下载 tarball
	url := fmt.Sprintf("https://registry.npmjs.org/%s/-/%s-%s.tgz", pkgName, pkgName, ver)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	// 提取 tarball 中的二进制
	binInTar := fmt.Sprintf("package/vendor/%s/bin/okp", triple)
	binData, err := extractFromTar(resp.Body, binInTar)
	if err != nil {
		return fmt.Errorf("提取二进制失败: %w", err)
	}

	// 找到当前二进制路径
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}

	// 先写临时文件，再替换
	tmp, err := os.CreateTemp(filepath.Dir(exe), "okp-update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(binData); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	if err := os.Chmod(tmpName, 0755); err != nil {
		return err
	}

	return os.Rename(tmpName, exe)
}

// extractFromTar 从 gzip tarball 中提取指定路径的文件内容
func extractFromTar(r io.Reader, target string) ([]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if strings.TrimPrefix(hdr.Name, "./") == target {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("binary not found in tarball: %s", target)
}
