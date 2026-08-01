package transform

import (
	"image"
	"image/color"
	"log"
	"math"
)

// solveLinear 使用高斯消元法（全选主元）求解 8x8 线性方程组
func solveLinear(A [][]float64, b []float64) ([]float64, error) {
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

	return result, nil
}

// computePerspectiveCoeffs 计算透视变换系数（第一参数 → 第二参数）
// 对于第一参数的每个点 (x,y)，计算其在第二参数中的对应点 (u,v):
//
//	u = (a*x + b*y + c) / (g*x + h*y + 1)
//	v = (d*x + e*y + f) / (g*x + h*y + 1)
//
// 返回系数 [a, b, c, d, e, f, g, h]
func computePerspectiveCoeffs(from, to [4]image.Point) ([]float64, error) {
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
		A[i*2][3] = 0
		A[i*2][4] = 0
		A[i*2][5] = 0
		A[i*2][6] = -u * x
		A[i*2][7] = -u * y
		b[i*2] = u

		// v = (d*x + e*y + f) / (g*x + h*y + 1)
		// => d*x + e*y + f - v*g*x - v*h*y = v
		A[i*2+1][0] = 0
		A[i*2+1][1] = 0
		A[i*2+1][2] = 0
		A[i*2+1][3] = x
		A[i*2+1][4] = y
		A[i*2+1][5] = 1
		A[i*2+1][6] = -v * x
		A[i*2+1][7] = -v * y
		b[i*2+1] = v
	}

	return solveLinear(A, b)
}

// bilinearInterpolate 在源图像中进行双线性插值
func bilinearInterpolate(src *image.RGBA, x, y float64) color.Color {
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	if x < 0 || x >= float64(w) || y < 0 || y >= float64(h) {
		return color.White
	}

	ix := int(math.Floor(x))
	iy := int(math.Floor(y))
	fx := x - float64(ix)
	fy := y - float64(iy)

	if ix < 0 {
		ix = 0
	}
	if iy < 0 {
		iy = 0
	}
	if ix >= w-1 {
		ix = w - 2
	}
	if iy >= h-1 {
		iy = h - 2
	}

	c00 := src.RGBAAt(ix, iy)
	c10 := src.RGBAAt(ix+1, iy)
	c01 := src.RGBAAt(ix, iy+1)
	c11 := src.RGBAAt(ix+1, iy+1)

	r := float64(c00.R)*(1-fx)*(1-fy) + float64(c10.R)*fx*(1-fy) +
		float64(c01.R)*(1-fx)*fy + float64(c11.R)*fx*fy
	g := float64(c00.G)*(1-fx)*(1-fy) + float64(c10.G)*fx*(1-fy) +
		float64(c01.G)*(1-fx)*fy + float64(c11.G)*fx*fy
	bl := float64(c00.B)*(1-fx)*(1-fy) + float64(c10.B)*fx*(1-fy) +
		float64(c01.B)*(1-fx)*fy + float64(c11.B)*fx*fy
	a := float64(c00.A)*(1-fx)*(1-fy) + float64(c10.A)*fx*(1-fy) +
		float64(c01.A)*(1-fx)*fy + float64(c11.A)*fx*fy

	return color.RGBA{
		R: uint8(math.Round(r)),
		G: uint8(math.Round(g)),
		B: uint8(math.Round(bl)),
		A: uint8(math.Round(a)),
	}
}

// PerspectiveWarp 对图像进行透视变换
// srcCorners: 源图像中身份证的四个角（左上、右上、右下、左下）
// 变换过程：将源图像中 srcCorners 围成的四边形映射到目标矩形
func PerspectiveWarp(src image.Image, srcCorners [4]image.Point, dstW, dstH int) *image.RGBA {
	srcRGBA, ok := src.(*image.RGBA)
	if !ok {
		bounds := src.Bounds()
		srcRGBA = image.NewRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				srcRGBA.Set(x, y, src.At(x, y))
			}
		}
	}

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
	coeffs, err := computePerspectiveCoeffs(dstCorners, srcCorners)
	if err != nil {
		log.Printf("透视变换系数求解失败，回退仿射变换: %v", err)
		return affineWarp(srcRGBA, srcCorners, dstW, dstH)
	}

	a, b, c := coeffs[0], coeffs[1], coeffs[2]
	d, e, f := coeffs[3], coeffs[4], coeffs[5]
	g, h := coeffs[6], coeffs[7]

	log.Printf("透视系数: a=%.6f b=%.6f c=%.6f d=%.6f e=%.6f f=%.6f g=%.6f h=%.6f",
		a, b, c, d, e, f, g, h)

	result := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	for dy := 0; dy < dstH; dy++ {
		for dx := 0; dx < dstW; dx++ {
			x := float64(dx)
			y := float64(dy)
			denom := g*x + h*y + 1
			if math.Abs(denom) < 1e-12 {
				continue
			}
			sx := (a*x + b*y + c) / denom
			sy := (d*x + e*y + f) / denom
			result.Set(dx, dy, bilinearInterpolate(srcRGBA, sx, sy))
		}
	}

	return result
}

// affineWarp 仿射变换回退方案
func affineWarp(src *image.RGBA, srcCorners [4]image.Point, dstW, dstH int) *image.RGBA {
	srcPts := []image.Point{srcCorners[0], srcCorners[1], srcCorners[3]}
	dstPts := []image.Point{
		{0, 0},
		{dstW - 1, 0},
		{0, dstH - 1},
	}

	n := 6
	A := make([][]float64, n)
	b := make([]float64, n)
	for i := range A {
		A[i] = make([]float64, n)
	}

	for i := 0; i < 3; i++ {
		dx := float64(dstPts[i].X)
		dy := float64(dstPts[i].Y)
		sx := float64(srcPts[i].X)
		sy := float64(srcPts[i].Y)

		A[i*2][0] = dx
		A[i*2][1] = dy
		A[i*2][2] = 1
		A[i*2][3] = 0
		A[i*2][4] = 0
		A[i*2][5] = 0
		b[i*2] = sx

		A[i*2+1][0] = 0
		A[i*2+1][1] = 0
		A[i*2+1][2] = 0
		A[i*2+1][3] = dx
		A[i*2+1][4] = dy
		A[i*2+1][5] = 1
		b[i*2+1] = sy
	}

	x := make([]float64, n)
	// 高斯消元（6x6）
	for col := 0; col < n; col++ {
		pivot := col
		for row := col + 1; row < n; row++ {
			if math.Abs(A[row][col]) > math.Abs(A[pivot][col]) {
				pivot = row
			}
		}
		A[col], A[pivot] = A[pivot], A[col]
		b[col], b[pivot] = b[pivot], b[col]
		pivotVal := A[col][col]
		if math.Abs(pivotVal) < 1e-12 {
			continue
		}
		for row := col + 1; row < n; row++ {
			factor := A[row][col] / pivotVal
			for k := col; k < n; k++ {
				A[row][k] -= factor * A[col][k]
			}
			b[row] -= factor * b[col]
		}
	}
	for i := n - 1; i >= 0; i-- {
		if math.Abs(A[i][i]) < 1e-12 {
			continue
		}
		sum := b[i]
		for j := i + 1; j < n; j++ {
			sum -= A[i][j] * x[j]
		}
		x[i] = sum / A[i][i]
	}

	result := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for dy := 0; dy < dstH; dy++ {
		for dx := 0; dx < dstW; dx++ {
			sx := x[0]*float64(dx) + x[1]*float64(dy) + x[2]
			sy := x[3]*float64(dx) + x[4]*float64(dy) + x[5]
			result.Set(dx, dy, bilinearInterpolate(src, sx, sy))
		}
	}
	return result
}
