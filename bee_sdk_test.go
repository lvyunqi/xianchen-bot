package main

import "testing"

func TestImageDownloadURL(t *testing.T) {
	want := "https://example.com/panel.png"
	for _, input := range []string{
		"[CQ:image,file=panel.png,url=https://example.com/panel.png]",
		"[图片, url=https://example.com/panel.png]",
		`{"type":"image","url":"https://example.com/panel.png"}`,
		"https://example.com/panel.png",
	} {
		if got := ImageDownloadURL(input); got != want {
			t.Errorf("ImageDownloadURL(%q) = %q, want %q", input, got, want)
		}
	}
	if got := ImageDownloadURL("上传图片 菜单"); got != "" {
		t.Fatalf("ordinary text unexpectedly parsed as image URL: %q", got)
	}
}
