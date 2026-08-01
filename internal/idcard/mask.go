// Copyright 2026 wjhdec
// SPDX-License-Identifier: Apache-2.0

package idcard

import (
	"image"
	"image/color"
	"image/draw"
)

// CreateRoundedCornerMask 创建圆角遮罩，圆角半径为 r，内部为白色（不透明），外部为黑色（透明）
func CreateRoundedCornerMask(w, h, r int) *image.Alpha {
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	// 全部设为不透明
	draw.Draw(mask, mask.Bounds(), &image.Uniform{color.Alpha{A: 255}}, image.Point{}, draw.Src)

	if r <= 0 {
		return mask
	}

	// 如果圆角半径超过短边一半，限制为短边一半
	maxR := min(w, h) / 2
	if r > maxR {
		r = maxR
	}

	// 四个圆角区域：将圆弧外部的像素设为透明，圆心在角区域的内部顶点
	// 左上角 — 圆心在角区域的右下顶点
	clearCorner(mask, r, image.Point{0, 0}, image.Point{r, r}, image.Point{r, r})
	// 右上角 — 圆心在角区域的左下顶点
	clearCorner(mask, r, image.Point{w - r, 0}, image.Point{w, r}, image.Point{w - r, r})
	// 左下角 — 圆心在角区域的右上顶点
	clearCorner(mask, r, image.Point{0, h - r}, image.Point{r, h}, image.Point{r, h - r})
	// 右下角 — 圆心在角区域的左上顶点
	clearCorner(mask, r, image.Point{w - r, h - r}, image.Point{w, h}, image.Point{w - r, h - r})

	return mask
}

// clearCorner 将圆角矩形角部超出圆弧范围的像素设为透明
// topLeft, bottomRight 定义角区域范围，center 为圆弧圆心
func clearCorner(mask *image.Alpha, r int, topLeft, bottomRight, center image.Point) {
	centerX := float64(center.X)
	centerY := float64(center.Y)
	radiusSq := float64(r * r)

	for y := topLeft.Y; y < bottomRight.Y; y++ {
		for x := topLeft.X; x < bottomRight.X; x++ {
			dx := float64(x) - centerX + 0.5
			dy := float64(y) - centerY + 0.5
			distSq := dx*dx + dy*dy
			if distSq > radiusSq {
				mask.SetAlpha(x, y, color.Alpha{0})
			}
		}
	}
}

// ApplyMask 将遮罩应用于图像，遮罩外部区域用 bgColor 填充
func ApplyMask(img *image.RGBA, mask *image.Alpha, bgColor color.Color) *image.RGBA {
	bounds := img.Bounds()
	result := image.NewRGBA(bounds)
	// 先用背景色填充
	draw.Draw(result, bounds, &image.Uniform{bgColor}, image.Point{}, draw.Src)
	// 在遮罩范围内叠加原图
	draw.DrawMask(result, bounds, img, image.Point{}, mask, image.Point{}, draw.Over)
	return result
}

// DrawRoundedCorners 在原图上直接绘制圆角，圆角外设为白色
// 这是简化版本：根据已知圆角半径和边距调整
func DrawRoundedCorners(img *image.RGBA, radius int) *image.RGBA {
	mask := CreateRoundedCornerMask(img.Bounds().Dx(), img.Bounds().Dy(), radius)
	return ApplyMask(img, mask, color.White)
}
