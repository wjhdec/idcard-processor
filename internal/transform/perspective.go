package transform

import (
	"image"
	"image/draw"
	"log"
	"math"
)

// solveLinear 使用高斯消元法（全选主元）求解线性方程组，返回解的切片
// 处理 6x6（仿射）与 8x8（透视）两种情况；矩阵奇异时对应系数取 0
func solveLinear(A [][]float64, b []float64) []float64 {
	n := len(A)
	colIdx := make([]int, n)
	for i := range colIdx {
		colIdx[i] = i
	}

	// 增广矩阵
	m := make([][]float64, n)
	for i := range m {
		m[i] = make([]float64, n+1)
		copy(m[i], A[i])
		m[i][n] = b[i]
	}

	// 前向消元
	for k := 0; k < n; k++ {
		// 寻找主元（k行及以下、k列及右侧的最大值）
		maxVal := 0.0
		maxRow, maxCol := k, k
		for i := k; i < n; i++ {
			for j := k; j < n; j++ {
				v := math.Abs(m[i][j])
				if v > maxVal {
					maxVal = v
					maxRow, maxCol = i, j
				}
			}
		}

		if maxVal < 1e-12 {
			continue // 矩阵奇异，剩余系数设为 0
		}

		// 交换行
		m[k], m[maxRow] = m[maxRow], m[k]

		// 交换列（记录列交换）
		for i := 0; i < n; i++ {
			m[i][k], m[i][maxCol] = m[i][maxCol], m[i][k]
		}
		colIdx[k], colIdx[maxCol] = colIdx[maxCol], colIdx[k]

		pivotVal := m[k][k]
		// 消去 k 行以下的所有行
		for i := k + 1; i < n; i++ {
			factor := m[i][k] / pivotVal
			for j := k; j <= n; j++ {
				m[i][j] -= factor * m[k][j]
			}
		}
	}

	// 回代求解
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		if math.Abs(m[i][i]) < 1e-12 {
			x[i] = 0
			continue
		}
		sum := m[i][n]
		for j := i + 1; j < n; j++ {
			sum -= m[i][j] * x[j]
		}
		x[i] = sum / m[i][i]
	}

	// 按照列交换恢复系数顺序
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[colIdx[i]] = x[i]
	}

	return result
}

// computePerspectiveCoeffs 计算透视变换系数（第一参数 → 第二参数）
// 对于第一参数的每个点 (x,y)，计算其在第二参数中的对应点 (u,v):
//
//	u = (a*x + b*y + c) / (g*x + h*y + 1)
//	v = (d*x + e*y + f) / (g*x + h*y + 1)
//
// 返回系数 [a, b, c, d, e, f, g, h]
func computePerspectiveCoeffs(from, to [4]image.Point) []float64 {
	A := make([][]float64, 8)
	for i := range A {
		A[i] = make([]float64, 8)
	}
	b := make([]float64, 8)

	for i := 0; i < 4; i++ {
		x := float64(from[i].X)
		y := float64(from[i].Y)
		u := float64(to[i].X)
		v := float64(to[i].Y)

		// u = (a*x + b*y + c) / (g*x + h*y + 1)
		// => a*x + b*y + c - u*g*x - u*h*y = u
		A[i*2][0] = x
		A[i*2][1] = y
		A[i*2][2] = 1
		A[i*2][6] = -u * x
		A[i*2][7] = -u * y
		b[i*2] = u

		// v = (d*x + e*y + f) / (g*x + h*y + 1)
		// => d*x + e*y + f - v*g*x - v*h*y = v
		A[i*2+1][3] = x
		A[i*2+1][4] = y
		A[i*2+1][5] = 1
		A[i*2+1][6] = -v * x
		A[i*2+1][7] = -v * y
		b[i*2+1] = v
	}

	return solveLinear(A, b)
}

// normalizedRGBA 将任意图像转换为原点在 (0,0) 的 *image.RGBA
// （*image.RGBA 且原点已是 (0,0) 时直接复用，避免拷贝）
func normalizedRGBA(src image.Image) *image.RGBA {
	if r, ok := src.(*image.RGBA); ok && r.Rect.Min == (image.Point{}) {
		return r
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

// PerspectiveWarp 对图像进行透视变换
// srcCorners: 源图像中身份证的四个角（左上、右上、右下、左下）
// 变换过程：将源图像中 srcCorners 围成的四边形映射到目标矩形
// 内层循环直接读写 Pix 数组（避免 Set/RGBAAt 的接口分发与边界检查），
// 双线性插值采样
func PerspectiveWarp(src image.Image, srcCorners [4]image.Point, dstW, dstH int) *image.RGBA {
	srcRGBA := normalizedRGBA(src)

	// 目标矩形角点（输出图像的四角）
	dstCorners := [4]image.Point{
		{0, 0},               // 左上
		{dstW - 1, 0},        // 右上
		{dstW - 1, dstH - 1}, // 右下
		{0, dstH - 1},        // 左下
	}

	// 计算逆变换系数：对于目标像素 (dx,dy)，找到源图像中的 (sx,sy)
	// computePerspectiveCoeffs(from, to) = from → to 的映射
	// 我们需要 dst → src，所以 from=dstCorners, to=srcCorners
	coeffs := computePerspectiveCoeffs(dstCorners, srcCorners)

	a, b, c := coeffs[0], coeffs[1], coeffs[2]
	d, e, f := coeffs[3], coeffs[4], coeffs[5]
	g, h := coeffs[6], coeffs[7]

	result := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	// 快速路径：源与目标都是原点在 (0,0) 的 RGBA，直接访问 Pix
	applyWarp(result, srcRGBA, func(x, y float64) (float64, float64) {
		denom := g*x + h*y + 1
		return (a*x + b*y + c) / denom, (d*x + e*y + f) / denom
	})

	log.Printf("透视系数: a=%.6f b=%.6f c=%.6f d=%.6f e=%.6f f=%.6f g=%.6f h=%.6f",
		a, b, c, d, e, f, g, h)

	return result
}

// applyWarp 遍历目标图像每个像素，通过 mapFunc 求源坐标并双线性采样
func applyWarp(dst, src *image.RGBA, mapFunc func(x, y float64) (float64, float64)) {
	dstW, dstH := dst.Rect.Dx(), dst.Rect.Dy()
	srcW, srcH := src.Rect.Dx(), src.Rect.Dy()
	srcPix, srcStride := src.Pix, src.Stride
	dstPix, dstStride := dst.Pix, dst.Stride

	for dy := 0; dy < dstH; dy++ {
		y := float64(dy)
		for dx := 0; dx < dstW; dx++ {
			out := dy*dstStride + dx*4
			sx, sy := mapFunc(float64(dx), y)

			if sx < 0 || sx >= float64(srcW) || sy < 0 || sy >= float64(srcH) {
				dstPix[out], dstPix[out+1], dstPix[out+2], dstPix[out+3] = 255, 255, 255, 255
				continue
			}

			ix := int(math.Floor(sx))
			iy := int(math.Floor(sy))
			fx := sx - float64(ix)
			fy := sy - float64(iy)

			// 双线性插值（四像素权重）
			w00 := (1 - fx) * (1 - fy)
			w10 := fx * (1 - fy)
			w01 := (1 - fx) * fy
			w11 := fx * fy

			// 四个采样点均在界内；越界方向的邻居退化用自身像素补齐
			// （该方向插值权重为 0，读自身像素结果等价）
			i00 := iy*srcStride + ix*4
			i10 := i00 + 4
			i01 := i00 + srcStride
			i11 := i00 + srcStride + 4
			if ix+1 >= srcW {
				i10, i11 = i00, i00
			}
			if iy+1 >= srcH {
				i01, i11 = i00, i00
			}

			dstPix[out] = uint8(math.Round(float64(srcPix[i00])*w00 + float64(srcPix[i10])*w10 + float64(srcPix[i01])*w01 + float64(srcPix[i11])*w11))
			dstPix[out+1] = uint8(math.Round(float64(srcPix[i00+1])*w00 + float64(srcPix[i10+1])*w10 + float64(srcPix[i01+1])*w01 + float64(srcPix[i11+1])*w11))
			dstPix[out+2] = uint8(math.Round(float64(srcPix[i00+2])*w00 + float64(srcPix[i10+2])*w10 + float64(srcPix[i01+2])*w01 + float64(srcPix[i11+2])*w11))
			dstPix[out+3] = uint8(math.Round(float64(srcPix[i00+3])*w00 + float64(srcPix[i10+3])*w10 + float64(srcPix[i01+3])*w01 + float64(srcPix[i11+3])*w11))
		}
	}
}

// affineWarp 仿射变换回退方案（透视系数求解失败时使用）
// 复用 solveLinear 求解 6x6 方程组
func affineWarp(src *image.RGBA, srcCorners [4]image.Point, dstW, dstH int) *image.RGBA {
	srcPts := []image.Point{srcCorners[0], srcCorners[1], srcCorners[3]}
	dstPts := []image.Point{
		{0, 0},
		{dstW - 1, 0},
		{0, dstH - 1},
	}

	A := make([][]float64, 6)
	for i := range A {
		A[i] = make([]float64, 6)
	}
	b := make([]float64, 6)

	for i := 0; i < 3; i++ {
		dx := float64(dstPts[i].X)
		dy := float64(dstPts[i].Y)
		sx := float64(srcPts[i].X)
		sy := float64(srcPts[i].Y)

		A[i*2][0], A[i*2][1], A[i*2][2] = dx, dy, 1
		b[i*2] = sx

		A[i*2+1][3], A[i*2+1][4], A[i*2+1][5] = dx, dy, 1
		b[i*2+1] = sy
	}

	coeff := solveLinear(A, b)

	srcRGBA := normalizedRGBA(src)
	result := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	applyWarp(result, srcRGBA, func(x, y float64) (float64, float64) {
		return coeff[0]*x + coeff[1]*y + coeff[2], coeff[3]*x + coeff[4]*y + coeff[5]
	})

	return result
}
