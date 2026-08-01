// Copyright 2026 wjhdec
// SPDX-License-Identifier: Apache-2.0

package detect

import (
	"image"
	"image/color"
	"math"
)

// ToGrayscale 将任意图像转换为灰度图
func ToGrayscale(src image.Image) *image.Gray {
	bounds := src.Bounds()
	dst := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}
	return dst
}

// GaussianBlur 对灰度图进行高斯模糊
// sigma 为标准差，核大小自动计算为 2*ceil(2*sigma) + 1
func GaussianBlur(src *image.Gray, sigma float64) *image.Gray {
	if sigma <= 0 {
		return src
	}

	ks := int(math.Ceil(2*sigma))*2 + 1
	if ks < 3 {
		ks = 3
	}
	halfK := ks / 2

	kernel := make([][]float64, ks)
	sum := 0.0
	for i := 0; i < ks; i++ {
		kernel[i] = make([]float64, ks)
		for j := 0; j < ks; j++ {
			x := float64(i - halfK)
			y := float64(j - halfK)
			val := math.Exp(-(x*x+y*y)/(2*sigma*sigma)) / (2 * math.Pi * sigma * sigma)
			kernel[i][j] = val
			sum += val
		}
	}
	for i := 0; i < ks; i++ {
		for j := 0; j < ks; j++ {
			kernel[i][j] /= sum
		}
	}

	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	dst := image.NewGray(bounds)
	tmp := image.NewGray(bounds)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sumVal float64
			for kx := -halfK; kx <= halfK; kx++ {
				sx := x + kx
				if sx < 0 {
					sx = 0
				} else if sx >= w {
					sx = w - 1
				}
				sumVal += float64(src.GrayAt(sx, y).Y) * kernel[kx+halfK][halfK]
			}
			tmp.SetGray(x, y, color.Gray{Y: uint8(math.Round(sumVal))})
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sumVal float64
			for ky := -halfK; ky <= halfK; ky++ {
				sy := y + ky
				if sy < 0 {
					sy = 0
				} else if sy >= h {
					sy = h - 1
				}
				sumVal += float64(tmp.GrayAt(x, sy).Y) * kernel[halfK][ky+halfK]
			}
			dst.SetGray(x, y, color.Gray{Y: uint8(math.Round(sumVal))})
		}
	}

	return dst
}

// gradient 同时计算 Sobel 梯度幅值和量化方向
// magnitude: 原始幅值（未归一化）, direction: 量化到 {0,1,2,3} 对应 0°,45°,90°,135°
// 同时返回最大幅值用于后续阈值计算
func gradient(src *image.Gray) (magnitude []float64, direction []uint8, maxMag float64, h, w int) {
	bounds := src.Bounds()
	w = bounds.Dx()
	h = bounds.Dy()

	magnitude = make([]float64, h*w)
	direction = make([]uint8, h*w)
	maxMag = 0

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x == 0 || x == w-1 || y == 0 || y == h-1 {
				magnitude[y*w+x] = 0
				direction[y*w+x] = 0
				continue
			}

			gx := float64(src.GrayAt(x+1, y-1).Y) +
				2*float64(src.GrayAt(x+1, y).Y) +
				float64(src.GrayAt(x+1, y+1).Y) -
				float64(src.GrayAt(x-1, y-1).Y) -
				2*float64(src.GrayAt(x-1, y).Y) -
				float64(src.GrayAt(x-1, y+1).Y)

			gy := float64(src.GrayAt(x-1, y+1).Y) +
				2*float64(src.GrayAt(x, y+1).Y) +
				float64(src.GrayAt(x+1, y+1).Y) -
				float64(src.GrayAt(x-1, y-1).Y) -
				2*float64(src.GrayAt(x, y-1).Y) -
				float64(src.GrayAt(x+1, y-1).Y)

			mag := math.Sqrt(gx*gx + gy*gy)
			magnitude[y*w+x] = mag
			if mag > maxMag {
				maxMag = mag
			}

			// 量化方向：0=水平(0°)，1=对角线(45°)，2=垂直(90°)，3=另一对角线(135°)
			angle := math.Atan2(gy, gx) // -π ~ π
			// 转换到 0~π
			if angle < 0 {
				angle += math.Pi
			}
			// 量化到 4 个扇区
			angle = angle * 180 / math.Pi // 0~180
			switch {
			case angle < 22.5 || angle >= 157.5:
				direction[y*w+x] = 0 // 水平
			case angle < 67.5:
				direction[y*w+x] = 1 // 45° 对角线
			case angle < 112.5:
				direction[y*w+x] = 2 // 垂直
			default:
				direction[y*w+x] = 3 // 135° 对角线
			}
		}
	}

	return magnitude, direction, maxMag, h, w
}

// nonMaxSuppression 非极大值抑制，保留梯度方向上的局部极大值
func nonMaxSuppression(mag []float64, dir []uint8, h, w int) []float64 {
	result := make([]float64, h*w)
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			idx := y*w + x
			d := dir[idx]
			var m1, m2 float64
			switch d {
			case 0: // 水平方向 — 比较左右
				m1 = mag[y*w+(x-1)]
				m2 = mag[y*w+(x+1)]
			case 1: // 45° 对角线 — 比较右上和左下
				m1 = mag[(y-1)*w+(x+1)]
				m2 = mag[(y+1)*w+(x-1)]
			case 2: // 垂直方向 — 比较上下
				m1 = mag[(y-1)*w+x]
				m2 = mag[(y+1)*w+x]
			case 3: // 135° 对角线 — 比较左上和右下
				m1 = mag[(y-1)*w+(x-1)]
				m2 = mag[(y+1)*w+(x+1)]
			}
			if mag[idx] >= m1 && mag[idx] >= m2 {
				result[idx] = mag[idx]
			}
		}
	}
	return result
}

// computeOtsuThreshold 计算 Otsu 最佳阈值（直接返回浮点阈值）
func computeOtsuThreshold(values []float64, maxVal float64) float64 {
	const bins = 256
	hist := make([]int, bins)
	count := 0
	for _, v := range values {
		bin := int(v / maxVal * (bins - 1))
		if bin >= bins {
			bin = bins - 1
		}
		hist[bin]++
		count++
	}
	if count == 0 {
		return maxVal * 0.5
	}

	var sum1 float64
	for i := 0; i < bins; i++ {
		sum1 += float64(i) * float64(hist[i])
	}

	var wB, wF float64
	var maxVariance, threshold float64
	var sumBF float64

	for i := 0; i < bins; i++ {
		wB += float64(hist[i])
		if wB == 0 {
			continue
		}
		wF = float64(count) - wB
		if wF == 0 {
			break
		}
		sumBF += float64(i) * float64(hist[i])
		mB := sumBF / wB
		mF := (sum1 - sumBF) / wF
		variance := wB * wF * (mB - mF) * (mB - mF)
		if variance > maxVariance {
			maxVariance = variance
			threshold = float64(i)
		}
	}

	// 将 bin 阈值映射回原始值范围
	return threshold / (bins - 1) * maxVal
}

// hysteresisThreshold 滞后阈值处理
// 强边缘: magnitude > high, 弱边缘: low < magnitude <= high
// 仅保留与强边缘连通的弱边缘
func hysteresisThreshold(mag []float64, h, w int, low, high float64) *image.Gray {
	// 标记像素类型: 0=非边缘, 1=弱边缘, 2=强边缘
	pixels := make([]uint8, h*w)

	for i := 0; i < h*w; i++ {
		if mag[i] >= high {
			pixels[i] = 2
		} else if mag[i] >= low {
			pixels[i] = 1
		}
	}

	// BFS 从强边缘出发，连接弱边缘
	visited := make([]bool, h*w)
	var queue []int

	for i := 0; i < h*w; i++ {
		if pixels[i] == 2 {
			queue = append(queue, i)
			visited[i] = true
		}
	}

	for len(queue) > 0 {
		idx := queue[0]
		queue = queue[1:]
		y := idx / w
		x := idx % w

		// 检查 8 邻域
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				nx, ny := x+dx, y+dy
				if nx < 0 || nx >= w || ny < 0 || ny >= h {
					continue
				}
				nidx := ny*w + nx
				if !visited[nidx] && pixels[nidx] == 1 {
					visited[nidx] = true
					pixels[nidx] = 2
					queue = append(queue, nidx)
				}
			}
		}
	}

	dst := image.NewGray(image.Rect(0, 0, w, h))
	for i := 0; i < h*w; i++ {
		if pixels[i] == 2 {
			dst.Pix[i] = 255
		}
	}

	return dst
}

// CannyEdges 完整 Canny 边缘检测
// sigma: 高斯模糊参数, lowRatio: 低阈值相对高阈值比例（默认 0.5）
// 如果 highThreshold <= 0，使用 Otsu 自动计算
func CannyEdges(src *image.Gray, sigma float64, lowRatio float64) *image.Gray {
	// 1. 高斯模糊
	blurred := GaussianBlur(src, sigma)

	// 2. 计算梯度
	mag, dir, maxMag, h, w := gradient(blurred)

	// 3. 非极大值抑制
	nms := nonMaxSuppression(mag, dir, h, w)

	// 4. 自动计算阈值
	highThresh := computeOtsuThreshold(nms, maxMag)
	if highThresh < 1 {
		highThresh = maxMag * 0.3
	}
	lowThresh := highThresh * lowRatio

	// 5. 滞后阈值
	return hysteresisThreshold(nms, h, w, lowThresh, highThresh)
}

// OtsuThreshold 使用 Otsu 算法对灰度图进行二值化（灰度值较高为白色）
func OtsuThreshold(src *image.Gray) *image.Gray {
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	total := w * h

	hist := make([]int, 256)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			hist[src.GrayAt(x, y).Y]++
		}
	}

	var sum1 float64
	for i := 0; i < 256; i++ {
		sum1 += float64(i) * float64(hist[i])
	}

	var wB, wF float64
	var maxVariance, threshold float64
	var sumBF float64

	for i := 0; i < 256; i++ {
		wB += float64(hist[i])
		if wB == 0 {
			continue
		}
		wF = float64(total) - wB
		if wF == 0 {
			break
		}
		sumBF += float64(i) * float64(hist[i])
		mB := sumBF / wB
		mF := (sum1 - sumBF) / wF
		variance := wB * wF * (mB - mF) * (mB - mF)
		if variance > maxVariance {
			maxVariance = variance
			threshold = float64(i)
		}
	}

	dst := image.NewGray(bounds)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if float64(src.GrayAt(x, y).Y) > threshold {
				dst.SetGray(x, y, color.Gray{Y: 255})
			} else {
				dst.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}

	return dst
}

// InvertThreshold 对灰度图进行反向 Otsu 二值化（灰度值较低为白色）
// 用于处理卡片比背景暗的情况
func InvertThreshold(src *image.Gray) *image.Gray {
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	total := w * h

	hist := make([]int, 256)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			hist[src.GrayAt(x, y).Y]++
		}
	}

	var sum1 float64
	for i := 0; i < 256; i++ {
		sum1 += float64(i) * float64(hist[i])
	}

	var wB, wF float64
	var maxVariance, threshold float64
	var sumBF float64

	for i := 0; i < 256; i++ {
		wB += float64(hist[i])
		if wB == 0 {
			continue
		}
		wF = float64(total) - wB
		if wF == 0 {
			break
		}
		sumBF += float64(i) * float64(hist[i])
		mB := sumBF / wB
		mF := (sum1 - sumBF) / wF
		variance := wB * wF * (mB - mF) * (mB - mF)
		if variance > maxVariance {
			maxVariance = variance
			threshold = float64(i)
		}
	}

	dst := image.NewGray(bounds)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if float64(src.GrayAt(x, y).Y) <= threshold {
				dst.SetGray(x, y, color.Gray{Y: 255})
			} else {
				dst.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}

	return dst
}

// MorphologicalClose 形态学闭运算：先膨胀后腐蚀
func MorphologicalClose(src *image.Gray, kernelSize int) *image.Gray {
	if kernelSize < 3 {
		kernelSize = 3
	}
	return erode(dilate(src, kernelSize), kernelSize)
}

func dilate(src *image.Gray, ksize int) *image.Gray {
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	half := ksize / 2
	dst := image.NewGray(bounds)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			maxVal := 0
			for ky := -half; ky <= half; ky++ {
				for kx := -half; kx <= half; kx++ {
					sx := x + kx
					sy := y + ky
					if sx < 0 || sx >= w || sy < 0 || sy >= h {
						continue
					}
					val := int(src.GrayAt(sx, sy).Y)
					if val > maxVal {
						maxVal = val
					}
				}
			}
			dst.SetGray(x, y, color.Gray{Y: uint8(maxVal)})
		}
	}
	return dst
}

func erode(src *image.Gray, ksize int) *image.Gray {
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	half := ksize / 2
	dst := image.NewGray(bounds)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			minVal := 255
			for ky := -half; ky <= half; ky++ {
				for kx := -half; kx <= half; kx++ {
					sx := x + kx
					sy := y + ky
					if sx < 0 || sx >= w || sy < 0 || sy >= h {
						continue
					}
					val := int(src.GrayAt(sx, sy).Y)
					if val < minVal {
						minVal = val
					}
				}
			}
			dst.SetGray(x, y, color.Gray{Y: uint8(minVal)})
		}
	}
	return dst
}

// clearImageBorder 将二值图像的四周边界像素清零
// 防止 Canny 检测到的图像硬边界（x=0, y=0 等）污染连通分量分析
func clearImageBorder(img *image.Gray) {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	black := color.Gray{Y: 0}

	for x := 0; x < w; x++ {
		img.SetGray(x, 0, black)
		img.SetGray(x, h-1, black)
	}
	for y := 0; y < h; y++ {
		img.SetGray(0, y, black)
		img.SetGray(w-1, y, black)
	}
}
