package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
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
	srcCorners := sortCorners(pts)

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

// sortCorners 将四个点按逆时针排序：左上、右上、右下、左下
func sortCorners(corners [4]image.Point) [4]image.Point {
	// 计算质心
	var cx, cy float64
	for _, p := range corners {
		cx += float64(p.X)
		cy += float64(p.Y)
	}
	cx /= 4
	cy /= 4

	// 按角度排序
	type ap struct {
		p     image.Point
		angle float64
	}
	aps := make([]ap, 4)
	for i, p := range corners {
		aps[i] = ap{p, math.Atan2(float64(p.Y)-cy, float64(p.X)-cx)}
	}
	sort.Slice(aps, func(i, j int) bool { return aps[i].angle < aps[j].angle })

	return [4]image.Point{aps[0].p, aps[1].p, aps[2].p, aps[3].p}
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

// imageToBase64 将图片编码为 base64 JPEG
func imageToBase64(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return "", err
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
