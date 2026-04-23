// Package sync 提供 Pilotty 配置备份到云端的 WebDAV 最小实现 (Phase 10-E-E)。
//
// 为什么不用 gowebdav 库: 依赖重 (~2MB), 我们只需 GET / PUT / PROPFIND 三个动词,
// HTTP Basic Auth, 手写 100 行足够, 也避免 gomobile 绑定时把 gowebdav 的辅助类型
// 都带进 Kotlin 命名空间。 坚果云 / AList / Seafile / Nextcloud 的 WebDAV 端点
// 都支持 HTTP Basic + PUT / GET, 本实现能覆盖。
package sync

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebDAVClient 最小 WebDAV 客户端: 支持 HTTP Basic Auth + Test / Push / Pull。
//
// 不做的事:
//   - MKCOL 创建目录 (用户需在云端先建好目标路径, 或用根目录)
//   - PROPFIND 遍历 (用 HEAD 代替探测文件存在)
//   - 锁 / token / 冲突合并 (直接覆盖 push, pull 无签名校验)
//   - 客户端证书 (Basic Auth 走 HTTPS 即够)
type WebDAVClient struct {
	BaseURL  string // 如 https://dav.jianguoyun.com/dav/
	Username string
	Password string
	client   *http.Client
}

// NewWebDAVClient 构造。 timeout 15s (坚果云海外可能慢, 留宽余)
func NewWebDAVClient(baseURL, user, pass string) *WebDAVClient {
	return &WebDAVClient{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		Username: user,
		Password: pass,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// TestConnection 用 HEAD BaseURL 测试 Basic Auth 是否通过。
// 200 / 207 / 401 区分: 401 = 认证失败, 其它 2xx = OK。
//
// 注意: 部分 WebDAV 服务对 HEAD 根路径返回 404 (坚果云), 也视为 "URL 可达 + 认证通过"。
func (c *WebDAVClient) TestConnection() error {
	req, err := http.NewRequest("HEAD", c.BaseURL+"/", nil)
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	req.SetBasicAuth(c.Username, c.Password)
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200, 204, 207, 404, 405:
		// 404/405 = URL 可达但根路径无 HEAD 响应, 认证已通过 (401 会在这之前挡住)
		return nil
	case 401, 403:
		return fmt.Errorf("认证失败 (HTTP %d): 用户名或密码错误", resp.StatusCode)
	default:
		return fmt.Errorf("服务器返回 HTTP %d", resp.StatusCode)
	}
}

// Push 上传 data 到 BaseURL/path. path 示例: "pilotty-backup.json"
func (c *WebDAVClient) Push(path string, data []byte) error {
	fullURL, err := joinPath(c.BaseURL, path)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", fullURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	req.SetBasicAuth(c.Username, c.Password)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(data))

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("上传失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("上传失败 (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

// Pull 从 BaseURL/path 下载. 返回文件原文.
func (c *WebDAVClient) Pull(path string) ([]byte, error) {
	fullURL, err := joinPath(c.BaseURL, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	req.SetBasicAuth(c.Username, c.Password)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("文件不存在: %s", path)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("下载失败 (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// 8 MB 上限 — 备份 JSON 不可能这么大, 超了说明拉到错文件
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	return data, nil
}

// joinPath 把 base + path 拼成合法 URL, 对 path 做最小转义 (空格 / 中文 / 斜杠保留)
func joinPath(base, path string) (string, error) {
	if base == "" {
		return "", fmt.Errorf("baseURL 为空")
	}
	if path == "" {
		return "", fmt.Errorf("path 为空")
	}
	p := strings.TrimLeft(path, "/")
	// url.PathEscape 会把 "/" 也转掉, 不符合 WebDAV 语义 (子目录必须保留 /)
	// 按 segment 逐段转义
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		parts[i] = url.PathEscape(seg)
	}
	return base + "/" + strings.Join(parts, "/"), nil
}
