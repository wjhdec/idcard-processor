package detect

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
)

// DetectCorners 从输入图像中检测身份证的四个角
// 使用多策略降级检测：边缘检测优先，模板定位兜底
func DetectCorners(src image.Image) ([4]image.Point, error) {
	bounds := src.Bounds()
	imgW, imgH := bounds.Dx(), bounds.Dy()
	log.Printf("输入图像大小: %dx%d", imgW, imgH)

	aspectRatio := 85.6 / 54.0 // 身份证宽高比

	// 转换为灰度图
	gray := ToGrayscale(src)

	// ==================== 策略 1: Canny 边缘检测 ====================
	log.Println("策略1: Canny 边缘检测...")
	if corners, found := tryCannyDetection(gray, imgW, imgH, aspectRatio); found {
		log.Printf("策略1 成功，角点: %v", corners)
		return corners, nil
	}
	log.Println("策略1 失败，尝试亮度分割...")

	// ==================== 策略 2: 亮度分割（卡片比背景亮） ====================
	log.Println("策略2: 亮度分割（亮目标）...")
	if corners, found := tryIntensityDetection(gray, imgW, imgH, aspectRatio, false); found {
		log.Printf("策略2 成功，角点: %v", corners)
		return corners, nil
	}
	log.Println("策略2 失败，尝试反向亮度分割...")

	// ==================== 策略 3: 反向亮度分割（卡片比背景暗） ====================
	log.Println("策略3: 反向亮度分割（暗目标）...")
	if corners, found := tryIntensityDetection(gray, imgW, imgH, aspectRatio, true); found {
		log.Printf("策略3 成功，角点: %v", corners)
		return corners, nil
	}
	log.Println("策略3 失败，尝试模板定位...")

	// ==================== 策略 4: 颜色模板定位 ====================
	if corners, found := tryTemplateDetection(src, imgW, imgH, aspectRatio); found {
		log.Printf("策略4 成功，角点: %v", corners)
		return corners, nil
	}

	return [4]image.Point{}, fmt.Errorf("未能检测到身份证轮廓，请确保照片中身份证清晰可见、光线充足")
}

// tryTemplateDetection 利用身份证正面白色背景特征定位卡片
// 正面身份证背景为白色，通过亮度+色度检测白色区域，找到卡片大致范围
func tryTemplateDetection(src image.Image, imgW, imgH int, aspectRatio float64) ([4]image.Point, bool) {
	var rgba *image.RGBA
	if r, ok := src.(*image.RGBA); ok {
		rgba = r
	} else {
		bounds := src.Bounds()
		rgba = image.NewRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				rgba.Set(x, y, src.At(x, y))
			}
		}
	}

	// 检测浅色区域（身份证正面背景为白色/浅色）
	whiteMask := detectLightRegions(rgba)

	// 大核闭运算连接碎片化的浅色区域
	whiteClosed := MorphologicalClose(whiteMask, 15)
	// 再用闭运算进一步合并
	whiteClosed = MorphologicalClose(whiteClosed, 15)

	// 找到最大浅色连通区域
	whiteComponent := FindLargestComponent(whiteClosed)
	if len(whiteComponent) < 200 {
		log.Println("  模板定位: 未找到足够大的白色区域")
		return [4]image.Point{}, false
	}

	// 使用凸包+DP获得初始角点
	hull := convexHull(whiteComponent)
	if len(hull) < 4 {
		return [4]image.Point{}, false
	}

	imgDiagonal := math.Sqrt(float64(imgW*imgW + imgH*imgH))
	epsilon := imgDiagonal * 0.02
	corners := SimplifyContour(hull, epsilon)

	for len(corners) > 4 && epsilon < imgDiagonal*0.3 {
		epsilon += imgDiagonal * 0.02
		corners = SimplifyContour(hull, epsilon)
	}

	if len(corners) < 4 {
		return [4]image.Point{}, false
	}

	var temp [4]image.Point
	for i := 0; i < 4 && i < len(corners); i++ {
		temp[i] = corners[i]
	}
	ordered := CornerOrder(temp)

	log.Printf("  模板定位: 白色区域=%dpx 角点=%v", len(whiteComponent), ordered)

	if corners, ok := validateCorners(ordered[:], imgW, imgH, aspectRatio); ok {
		return corners, true
	}
	if corners, ok := validateCornersRelaxed(ordered[:], imgW, imgH, aspectRatio); ok {
		return corners, true
	}

	return [4]image.Point{}, false
}

// detectLightRegions 检测图像中的浅色区域
// 身份证正面以白色/米白为主，放宽阈值并通过形态学合并
func detectLightRegions(rgba *image.RGBA) *image.Gray {
	bounds := rgba.Bounds()
	dst := image.NewGray(bounds)

	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			c := rgba.RGBAAt(x, y)
			r := int(c.R)
			g := int(c.G)
			b := int(c.B)

			// 浅色判定：三通道都 > 150，总亮度 > 480，通道差异 < 80
			if r > 150 && g > 150 && b > 150 {
				maxDiff := max(abs(r-g), max(abs(g-b), abs(r-b)))
				if maxDiff < 80 && r+g+b > 480 {
					dst.SetGray(x, y, color.Gray{Y: 255})
				}
			}
		}
	}

	return dst
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// tryCannyDetection 使用 Canny 边缘检测定位身份证
func tryCannyDetection(gray *image.Gray, imgW, imgH int, aspectRatio float64) ([4]image.Point, bool) {
	// Canny 边缘检测（使用多种 sigma 参数尝试）
	sigmas := []float64{1.0, 1.5, 2.0, 0.8}
	for _, sigma := range sigmas {
		edges := CannyEdges(gray, sigma, 0.5)

		// 闭运算连接边缘
		closed := MorphologicalClose(edges, 5)

		// 清除图像边界像素，防止 Canny 检测到的图像硬边界
		// 污染连通分量（图像边框会与身份证边缘连通，影响角点检测）
		clearImageBorder(closed)

		// 找最佳连通分量
		component, score := FindLargestComponentArea(closed, 0.02)
		if len(component) < 100 {
			continue
		}

		if corners, found := findCornersFromComponent(component, imgW, imgH, aspectRatio); found {
			log.Printf("  Canny(sigma=%.1f) 得分=%.0f, 找到角点", sigma, score)
			return corners, true
		}
		log.Printf("  Canny(sigma=%.1f) 得分=%.0f, 未通过角点验证", sigma, score)
	}

	return [4]image.Point{}, false
}

// tryIntensityDetection 使用灰度亮度分割定位身份证
// invert=true 表示卡片比背景暗（取反）
func tryIntensityDetection(gray *image.Gray, imgW, imgH int, aspectRatio float64, invert bool) ([4]image.Point, bool) {
	// 使用不同的模糊强度尝试
	sigmas := []float64{3.0, 5.0, 7.0, 9.0, 2.0}
	for _, sigma := range sigmas {
		blurred := GaussianBlur(gray, sigma)

		var binary *image.Gray
		if invert {
			binary = InvertThreshold(blurred)
		} else {
			binary = OtsuThreshold(blurred)
		}

		// 闭运算填充小孔
		closed := MorphologicalClose(binary, 5)
		// 开运算去除噪点
		// 先腐蚀再膨胀
		opened := dilate(erode(closed, 3), 3)

		// 找最佳连通分量
		component, score := FindLargestComponentArea(opened, 0.05)
		if len(component) < 100 {
			continue
		}

		if corners, found := findCornersFromComponent(component, imgW, imgH, aspectRatio); found {
			mode := "亮目标"
			if invert {
				mode = "暗目标"
			}
			log.Printf("  亮度分割(sigma=%.1f, %s) 得分=%.0f, 找到角点", sigma, mode, score)
			return corners, true
		}
		mode := "亮目标"
		if invert {
			mode = "暗目标"
		}
		log.Printf("  亮度分割(sigma=%.1f, %s) 得分=%.0f, 未通过角点验证", sigma, mode, score)
	}

	return [4]image.Point{}, false
}
