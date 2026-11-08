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

// gzipTestBody 超过 minGzipLength（4096 字节），确保会触发压缩。
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

func TestGzipSkipsSmallResponse(t *testing.T) {
	w := serveGzip(t, newGzipTestRouter(), "/api/small", "gzip")

	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("小于 %d 字节的响应不应压缩，实际 Content-Encoding=%q", minGzipLength, got)
	}
	if got := w.Body.String(); !strings.Contains(got, `"ok":true`) {
		t.Fatalf("小响应内容异常：%q", got)
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
