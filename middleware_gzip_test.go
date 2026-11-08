package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// gzipTestBody 是较大的响应体（4096 字节），用于验证压缩。
var gzipTestBody = strings.Repeat("gzip middleware ", 256)

// newGzipTestRouter 构造只挂 gzip 中间件的测试路由，不依赖数据库。
func newGzipTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(gzipCompression())

	r.GET("/api/large", func(c *gin.Context) {
		Response{}.Ok(c, http.StatusOK, gin.H{"content": gzipTestBody})
	})
	r.GET("/api/small", func(c *gin.Context) {
		Response{}.Ok(c, http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/api/avatar", func(c *gin.Context) {
		// 头像接口返回已压缩的二进制（这里用 PNG 魔数模拟）
		c.Data(http.StatusOK, "image/png", bytes.Repeat([]byte{0x89, 'P', 'N', 'G'}, 512))
	})
	r.GET("/api/error", func(c *gin.Context) {
		c.Data(http.StatusInternalServerError, "text/plain; charset=utf-8", []byte(gzipTestBody))
	})
	r.GET("/api/no-content", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	return r
}

// serveGzip 发送一次 GET 请求；acceptEncoding 为空表示不带 Accept-Encoding。
func serveGzip(t *testing.T, r *gin.Engine, path, acceptEncoding string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGzipCompressesLargeResponse(t *testing.T) {
	w := serveGzip(t, newGzipTestRouter(), "/api/large", "gzip, deflate")

	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("大响应应使用 gzip 压缩，实际 Content-Encoding=%q", got)
	}
	if vary := w.Header().Get("Vary"); !strings.Contains(vary, "Accept-Encoding") {
		t.Fatalf("应返回 Vary: Accept-Encoding，实际 %q", vary)
	}

	reader, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("响应不是合法的 gzip 数据：%v", err)
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("解压响应失败：%v", err)
	}
	if !strings.Contains(string(body), gzipTestBody) {
		t.Fatalf("解压后内容不完整：%q", string(body))
	}
}

func TestGzipSkippedWithoutAcceptEncoding(t *testing.T) {
	w := serveGzip(t, newGzipTestRouter(), "/api/large", "")

	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("未声明 Accept-Encoding 不应压缩，实际 Content-Encoding=%q", got)
	}
	if !strings.Contains(w.Body.String(), gzipTestBody) {
		t.Fatal("未压缩响应内容不完整")
	}
}

// 小响应同样要压缩：一旦使用 gzip.WithMinLength，gin 的 c.Data（静态文件）
// 会因为 Content-Length 触发库的明文直写分支，导致「声明 gzip、body 明文」。
func TestGzipCompressesSmallResponse(t *testing.T) {
	w := serveGzip(t, newGzipTestRouter(), "/api/small", "gzip")

	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("小响应也应使用 gzip 压缩，实际 Content-Encoding=%q", got)
	}

	reader, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("响应不是合法的 gzip 数据：%v", err)
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("解压响应失败：%v", err)
	}
	if !strings.Contains(string(body), `"ok":true`) {
		t.Fatalf("解压后的小响应内容异常：%q", string(body))
	}
}

// TestGzipCompressesSmallDataOnRealServer 覆盖真实 net/http 行为：
// gin 的 c.Data 会设置 Content-Length，曾经配合 WithMinLength 会产生
// 「Content-Encoding: gzip + 明文 body」的错误响应（Web UI 白屏）。
func TestGzipCompressesSmallDataOnRealServer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	payload := strings.Repeat("a", 793) // 与 index.html 体积接近
	r := gin.New()
	r.Use(gzipCompression())
	r.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(payload))
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
	if err != nil {
		t.Fatalf("构造请求失败：%v", err)
	}
	// 手动设置 Accept-Encoding：DefaultTransport 不会自动解压
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatalf("请求失败：%v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("应返回 gzip 压缩，实际 Content-Encoding=%q", got)
	}

	reader, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("响应体不是合法的 gzip 数据：%v", err)
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("解压响应失败：%v", err)
	}
	if string(body) != payload {
		t.Fatalf("解压后内容不一致：len=%d，期望 %d", len(body), len(payload))
	}
}

func TestGzipSkipsAvatarPath(t *testing.T) {
	w := serveGzip(t, newGzipTestRouter(), "/api/avatar?name=alice", "gzip")

	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("头像接口不应压缩，实际 Content-Encoding=%q", got)
	}
	if got := w.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("头像 Content-Type 应为 image/png，实际 %q", got)
	}
	if w.Body.Len() != 2048 {
		t.Fatalf("头像响应长度应为 2048，实际 %d", w.Body.Len())
	}
}

func TestGzipSkipsErrorResponse(t *testing.T) {
	w := serveGzip(t, newGzipTestRouter(), "/api/error", "gzip")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("状态码应为 500，实际 %d", w.Code)
	}
	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("错误响应不应压缩，实际 Content-Encoding=%q", got)
	}
	if w.Body.Len() != len(gzipTestBody) {
		t.Fatalf("错误响应长度应为 %d，实际 %d", len(gzipTestBody), w.Body.Len())
	}
}

func TestGzipSkipsEmptyResponse(t *testing.T) {
	w := serveGzip(t, newGzipTestRouter(), "/api/no-content", "gzip")

	if w.Code != http.StatusNoContent {
		t.Fatalf("状态码应为 204，实际 %d", w.Code)
	}
	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("204 响应不应压缩，实际 Content-Encoding=%q", got)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("204 响应不应有 body，实际 %q", w.Body.String())
	}
}
