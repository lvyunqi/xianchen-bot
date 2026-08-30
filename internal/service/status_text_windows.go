//go:build windows

package service

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"syscall"
	"unsafe"
)

type statusFont struct {
	Family string
}

type gdiBitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type gdiRGBQuad struct {
	Blue     byte
	Green    byte
	Red      byte
	Reserved byte
}

type gdiBitmapInfo struct {
	Header gdiBitmapInfoHeader
	Colors [1]gdiRGBQuad
}

type gdiSize struct {
	CX int32
	CY int32
}

var (
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procCreateFontW        = gdi32.NewProc("CreateFontW")
	procSetTextColor       = gdi32.NewProc("SetTextColor")
	procSetBkColor         = gdi32.NewProc("SetBkColor")
	procSetBkMode          = gdi32.NewProc("SetBkMode")
	procTextOutW           = gdi32.NewProc("TextOutW")
	procGetTextExtentW     = gdi32.NewProc("GetTextExtentPoint32W")
)

const (
	gdiRGBColors        = 0
	gdiBI_RGB           = 0
	gdiOpaque           = 2
	gdiNormalWeight     = 400
	gdiDefaultCharset   = 1
	gdiOutDefaultPrecis = 0
	gdiClipDefault      = 0
	gdiAntialiased      = 4
	gdiDefaultPitch     = 0
)

func loadStatusFont(configured string) (*statusFont, error) {
	family := strings.TrimSpace(configured)
	if family == "" {
		family = "Microsoft YaHei UI"
	}
	selected := &statusFont{Family: family}
	if _, _, err := measureStatusText(selected, "仙尘", 28); err != nil {
		return nil, fmt.Errorf("初始化状态图中文字体: %w", err)
	}
	return selected, nil
}

func measureStatusText(selected *statusFont, text string, size int) (int, int, error) {
	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return 0, 0, fmt.Errorf("CreateCompatibleDC失败")
	}
	defer procDeleteDC.Call(dc)
	fontHandle, err := createStatusGDIFont(selected, size)
	if err != nil {
		return 0, 0, err
	}
	defer procDeleteObject.Call(fontHandle)
	previous, _, _ := procSelectObject.Call(dc, fontHandle)
	defer procSelectObject.Call(dc, previous)

	utf16Value, err := syscall.UTF16FromString(text)
	if err != nil {
		return 0, 0, err
	}
	var measured gdiSize
	result, _, callErr := procGetTextExtentW.Call(dc, uintptr(unsafe.Pointer(&utf16Value[0])), uintptr(len(utf16Value)-1), uintptr(unsafe.Pointer(&measured)))
	if result == 0 {
		return 0, 0, fmt.Errorf("GetTextExtentPoint32W失败: %v", callErr)
	}
	return int(measured.CX), int(measured.CY), nil
}

func paintStatusText(destination *image.RGBA, selected *statusFont, text string, left, top, size int, textColor color.Color) error {
	width, height, err := measureStatusText(selected, text, size)
	if err != nil || width <= 0 || height <= 0 {
		return err
	}
	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return fmt.Errorf("CreateCompatibleDC失败")
	}
	defer procDeleteDC.Call(dc)

	info := gdiBitmapInfo{Header: gdiBitmapInfoHeader{
		Size: uint32(unsafe.Sizeof(gdiBitmapInfoHeader{})), Width: int32(width), Height: -int32(height),
		Planes: 1, BitCount: 32, Compression: gdiBI_RGB, SizeImage: uint32(width * height * 4),
	}}
	var bits unsafe.Pointer
	bitmap, _, callErr := procCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&info)), gdiRGBColors, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 || bits == nil {
		return fmt.Errorf("CreateDIBSection失败: %v", callErr)
	}
	defer procDeleteObject.Call(bitmap)
	previousBitmap, _, _ := procSelectObject.Call(dc, bitmap)
	defer procSelectObject.Call(dc, previousBitmap)

	fontHandle, err := createStatusGDIFont(selected, size)
	if err != nil {
		return err
	}
	defer procDeleteObject.Call(fontHandle)
	previousFont, _, _ := procSelectObject.Call(dc, fontHandle)
	defer procSelectObject.Call(dc, previousFont)

	pixels := unsafe.Slice((*byte)(bits), width*height*4)
	clear(pixels)
	procSetTextColor.Call(dc, 0x00FFFFFF)
	procSetBkColor.Call(dc, 0x00000000)
	procSetBkMode.Call(dc, gdiOpaque)
	utf16Value, err := syscall.UTF16FromString(text)
	if err != nil {
		return err
	}
	result, _, callErr := procTextOutW.Call(dc, 0, 0, uintptr(unsafe.Pointer(&utf16Value[0])), uintptr(len(utf16Value)-1))
	if result == 0 {
		return fmt.Errorf("TextOutW失败: %v", callErr)
	}

	foreground := color.RGBAModel.Convert(textColor).(color.RGBA)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			index := (y*width + x) * 4
			coverage := maxByte(pixels[index], pixels[index+1], pixels[index+2])
			if coverage == 0 {
				continue
			}
			destinationX, destinationY := left+x, top+y
			if !image.Pt(destinationX, destinationY).In(destination.Bounds()) {
				continue
			}
			base := color.RGBAModel.Convert(destination.At(destinationX, destinationY)).(color.RGBA)
			alpha := uint32(coverage) * uint32(foreground.A) / 255
			inverse := uint32(255) - alpha
			destination.SetRGBA(destinationX, destinationY, color.RGBA{
				R: uint8((uint32(foreground.R)*alpha + uint32(base.R)*inverse) / 255),
				G: uint8((uint32(foreground.G)*alpha + uint32(base.G)*inverse) / 255),
				B: uint8((uint32(foreground.B)*alpha + uint32(base.B)*inverse) / 255),
				A: 255,
			})
		}
	}
	return nil
}

func createStatusGDIFont(selected *statusFont, size int) (uintptr, error) {
	family, err := syscall.UTF16PtrFromString(selected.Family)
	if err != nil {
		return 0, err
	}
	height := uintptr(uint32(int32(-maxInt(size, 1))))
	handle, _, callErr := procCreateFontW.Call(
		height, 0, 0, 0, gdiNormalWeight, 0, 0, 0,
		gdiDefaultCharset, gdiOutDefaultPrecis, gdiClipDefault, gdiAntialiased,
		gdiDefaultPitch, uintptr(unsafe.Pointer(family)),
	)
	if handle == 0 {
		return 0, fmt.Errorf("CreateFontW失败: %v", callErr)
	}
	return handle, nil
}

func maxByte(values ...byte) byte {
	var maximum byte
	for _, value := range values {
		if value > maximum {
			maximum = value
		}
	}
	return maximum
}
