package handler

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// 公网访问管理后台时，未压缩的 JS/CSS 体积是主要卡顿来源。
// 对文本类响应做实时 gzip（图片/字体已压缩，跳过），压缩率通常在 65% 以上。

var gzipWriterPool = sync.Pool{
	New: func() any { return gzip.NewWriter(io.Discard) },
}

func gzipEligible(contentType string) bool {
	ct := strings.ToLower(strings.Split(contentType, ";")[0])
	switch {
	case strings.HasPrefix(ct, "text/"):
		return true
	case strings.HasSuffix(ct, "javascript"),
		strings.HasSuffix(ct, "json"),
		strings.HasSuffix(ct, "xml"),
		strings.HasSuffix(ct, "+json"),
		strings.HasSuffix(ct, "+xml"):
		return true
	}
	return false
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer *gzip.Writer
	wrote  bool
}

func (g *gzipResponseWriter) WriteHeader(status int) {
	if g.wrote {
		return
	}
	g.wrote = true
	ct := g.Header().Get("Content-Type")
	if gzipEligible(ct) {
		g.Header().Del("Content-Length")
		g.Header().Set("Content-Encoding", "gzip")
		g.Header().Add("Vary", "Accept-Encoding")
		g.writer.Reset(g.ResponseWriter)
	} else {
		g.writer = nil
	}
	g.ResponseWriter.WriteHeader(status)
}

func (g *gzipResponseWriter) Write(data []byte) (int, error) {
	if !g.wrote {
		if g.Header().Get("Content-Type") == "" {
			g.Header().Set("Content-Type", http.DetectContentType(data))
		}
		g.WriteHeader(http.StatusOK)
	}
	if g.writer != nil {
		return g.writer.Write(data)
	}
	return g.ResponseWriter.Write(data)
}

func (g *gzipResponseWriter) Flush() {
	if g.writer != nil {
		_ = g.writer.Flush()
	}
	if flusher, ok := g.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func compressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		writer := gzipWriterPool.Get().(*gzip.Writer)
		defer gzipWriterPool.Put(writer)
		gw := &gzipResponseWriter{ResponseWriter: w, writer: writer}
		defer func() {
			if gw.writer != nil {
				_ = gw.writer.Close()
			}
		}()
		next.ServeHTTP(gw, r)
	})
}
