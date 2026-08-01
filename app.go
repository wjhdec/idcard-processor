// Copyright 2026 wjhdec
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"idcard-processor/internal/detect"
	"idcard-processor/internal/idcard"
	"idcard-processor/internal/transform"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context
}

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type ImageInfo struct {
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Base64   string `json:"base64"`
	FilePath string `json:"filePath"`
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SelectInputFile 打开文件选择对话框，返回图片信息（宽高+base64预览）
func (a *App) SelectInputFile() (*ImageInfo, error) {
	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择身份证照片",
		Filters: []runtime.FileFilter{
			{DisplayName: "图片文件 (*.jpg, *.jpeg, *.png)", Pattern: "*.jpg;*.jpeg;*.png"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("打开文件对话框失败: %w", err)
	}
	if filePath == "" {
		return nil, nil
	}

	return a.loadImageInfo(filePath)
}

// LoadImage 加载指定路径的图片并返回信息
func (a *App) LoadImage(filePath string) (*ImageInfo, error) {
	return a.loadImageInfo(filePath)
}

func (a *App) loadImageInfo(filePath string) (*ImageInfo, error) {
	src, err := decodeImage(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取图片失败: %w", err)
	}

	bounds := src.Bounds()
	base64Str, err := imageToBase64(src)
	if err != nil {
		return nil, fmt.Errorf("编码图片失败: %w", err)
	}

	return &ImageInfo{
		Width:    bounds.Dx(),
		Height:   bounds.Dy(),
		Base64:   base64Str,
		FilePath: filePath,
	}, nil
}

// DetectCorners 自动检测图片中身份证的四个角点（初始估计）
func (a *App) DetectCorners(filePath string) ([4]Point, error) {
	src, err := decodeImage(filePath)
	if err != nil {
		return [4]Point{}, err
	}
	corners, err := detect.DetectCorners(src)
	if err != nil {
		return [4]Point{}, err
	}
	return [4]Point{
		{X: corners[0].X, Y: corners[0].Y},
		{X: corners[1].X, Y: corners[1].Y},
		{X: corners[2].X, Y: corners[2].Y},
		{X: corners[3].X, Y: corners[3].Y},
	}, nil
}

// ProcessImage 用用户选择的四个角点处理图片
// 接收角点后自动按位置排序为：左上、右上、右下、左下
func (a *App) ProcessImage(filePath string, corners [4]Point, dpi int, outputPath string) error {
	src, err := decodeImage(filePath)
	if err != nil {
		return fmt.Errorf("读取图片失败: %w", err)
	}

	// 转换为 image.Point 并自动按位置排序
	pts := [4]image.Point{
		{X: corners[0].X, Y: corners[0].Y},
		{X: corners[1].X, Y: corners[1].Y},
		{X: corners[2].X, Y: corners[2].Y},
		{X: corners[3].X, Y: corners[3].Y},
	}
	srcCorners := detect.CornerOrder(pts)

	// 计算目标尺寸
	cardW := int(idcard.CardWidthMM / 25.4 * float64(dpi))
	cardH := int(idcard.CardHeightMM / 25.4 * float64(dpi))

	// 透视变换
	warped := transform.PerspectiveWarp(src, srcCorners, cardW, cardH)

	// 圆角遮罩
	radius := int(idcard.CornerRadiusMM / 25.4 * float64(dpi))
	result := idcard.DrawRoundedCorners(warped, radius)

	// 保存
	if err := saveImage(outputPath, result); err != nil {
		return fmt.Errorf("保存图片失败: %w", err)
	}

	return nil
}

// SelectOutputFile 打开保存文件对话框
func (a *App) SelectOutputFile(defaultName string) (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "保存处理结果",
		DefaultFilename: defaultName,
		Filters: []runtime.FileFilter{
			{DisplayName: "JPEG (*.jpg)", Pattern: "*.jpg"},
			{DisplayName: "PNG (*.png)", Pattern: "*.png"},
		},
	})
}

// GetDefaultDPI 返回默认 DPI
func (a *App) GetDefaultDPI() int {
	return idcard.OutputDPI
}

// decodeImage 读取图像文件
func decodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return jpeg.Decode(f)
	case ".png":
		return png.Decode(f)
	default:
		return nil, fmt.Errorf("不支持的图片格式: %s", ext)
	}
}

// saveImage 保存图像文件
func saveImage(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return jpeg.Encode(f, img, &jpeg.Options{Quality: 95})
	case ".png":
		return png.Encode(f, img)
	default:
		return fmt.Errorf("不支持的输出格式: %s", ext)
	}
}

// imageToBase64 将图片编码为 base64 JPEG 预览（最长边先降采样到 1600px，降低内存与编码耗时）
func imageToBase64(img image.Image) (string, error) {
	preview := downscaleForPreview(img, 1600)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, preview, &jpeg.Options{Quality: 85}); err != nil {
		return "", err
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// downscaleForPreview 将图像最长边缩放到 maxDim 以内（双线性插值），保持宽高比
// 尺寸未超限时原样返回，不做额外拷贝
func downscaleForPreview(img image.Image, maxDim int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxDim && h <= maxDim {
		return img
	}

	scale := float64(maxDim) / float64(max(w, h))
	dstW := int(math.Round(float64(w) * scale))
	dstH := int(math.Round(float64(h) * scale))
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	// 双线性采样（映射到源图中心对齐坐标）
	for dy := 0; dy < dstH; dy++ {
		sy := (float64(dy)+0.5)*float64(h)/float64(dstH) - 0.5
		if sy < 0 {
			sy = 0
		}
		iy := int(sy)
		fy := sy - float64(iy)
		if iy >= h-1 {
			iy = h - 2
		}
		for dx := 0; dx < dstW; dx++ {
			sx := (float64(dx)+0.5)*float64(w)/float64(dstW) - 0.5
			if sx < 0 {
				sx = 0
			}
			ix := int(sx)
			fx := sx - float64(ix)
			if ix >= w-1 {
				ix = w - 2
			}

			c00 := toRGBA(img.At(b.Min.X+ix, b.Min.Y+iy))
			c10 := toRGBA(img.At(b.Min.X+ix+1, b.Min.Y+iy))
			c01 := toRGBA(img.At(b.Min.X+ix, b.Min.Y+iy+1))
			c11 := toRGBA(img.At(b.Min.X+ix+1, b.Min.Y+iy+1))

			w00 := (1 - fx) * (1 - fy)
			w10 := fx * (1 - fy)
			w01 := (1 - fx) * fy
			w11 := fx * fy

			dst.SetRGBA(dx, dy, color.RGBA{
				R: uint8(math.Round(float64(c00.R)*w00 + float64(c10.R)*w10 + float64(c01.R)*w01 + float64(c11.R)*w11)),
				G: uint8(math.Round(float64(c00.G)*w00 + float64(c10.G)*w10 + float64(c01.G)*w01 + float64(c11.G)*w11)),
				B: uint8(math.Round(float64(c00.B)*w00 + float64(c10.B)*w10 + float64(c01.B)*w01 + float64(c11.B)*w11)),
				A: uint8(math.Round(float64(c00.A)*w00 + float64(c10.A)*w10 + float64(c01.A)*w01 + float64(c11.A)*w11)),
			})
		}
	}
	return dst
}

// toRGBA 将任意 color.Color 归一化为 color.RGBA（非 8bit 通道会损失精度，预览可接受）
func toRGBA(c color.Color) color.RGBA {
	if r, ok := c.(color.RGBA); ok {
		return r
	}
	r, g, b, a := c.RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}
