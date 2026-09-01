//go:build !windows

package service

import (
	"fmt"
	"image"
	"image/color"
)

type statusFont struct{}

func statusImageRenderingSupported() bool {
	return false
}

func loadStatusFont(string) (*statusFont, error) {
	return nil, fmt.Errorf("状态图片文字渲染仅支持Windows运行环境")
}

func measureStatusText(*statusFont, string, int) (int, int, error) {
	return 0, 0, fmt.Errorf("状态图片文字渲染仅支持Windows运行环境")
}

func paintStatusText(*image.RGBA, *statusFont, string, int, int, int, color.Color) error {
	return fmt.Errorf("状态图片文字渲染仅支持Windows运行环境")
}
