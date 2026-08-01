package detect

import (
	"image"
	"math"
	"sort"
)

// floodFill 从指定种子点开始，使用栈进行四连通泛洪填充，返回所有连通像素
func floodFill(binary *image.Gray, x, y int, visited [][]bool) []image.Point {
	bounds := binary.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	if x < 0 || x >= w || y < 0 || y >= h {
		return nil
	}
	if visited[y][x] {
		return nil
	}
	if binary.GrayAt(x, y).Y == 0 {
		return nil
	}

	var component []image.Point
	stack := []image.Point{{X: x, Y: y}}

	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if p.X < 0 || p.X >= w || p.Y < 0 || p.Y >= h {
			continue
		}
		if visited[p.Y][p.X] {
			continue
		}
		if binary.GrayAt(p.X, p.Y).Y == 0 {
			continue
		}

		visited[p.Y][p.X] = true
		component = append(component, p)

		stack = append(stack,
			image.Point{X: p.X + 1, Y: p.Y},
			image.Point{X: p.X - 1, Y: p.Y},
			image.Point{X: p.X, Y: p.Y + 1},
			image.Point{X: p.X, Y: p.Y - 1},
		)
	}

	return component
}

// FindLargestComponent 在二值图像中找到最大的连通分量（白色区域）
func FindLargestComponent(binary *image.Gray) []image.Point {
	bounds := binary.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	visited := make([][]bool, h)
	for i := range visited {
		visited[i] = make([]bool, w)
	}

	var largest []image.Point
	maxSize := 0

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !visited[y][x] && binary.GrayAt(x, y).Y > 0 {
				comp := floodFill(binary, x, y, visited)
				if len(comp) > maxSize {
					maxSize = len(comp)
					largest = comp
				}
			}
		}
	}

	return largest
}

// FindLargestComponentArea 在二值图像中找到面积最大（按凸包面积）的连通分量
// 同时过滤形状太不规则的分量
func FindLargestComponentArea(binary *image.Gray, minAreaRatio float64) ([]image.Point, float64) {
	bounds := binary.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	imgArea := float64(w * h)

	visited := make([][]bool, h)
	for i := range visited {
		visited[i] = make([]bool, w)
	}

	var bestComponent []image.Point
	bestScore := 0.0

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !visited[y][x] && binary.GrayAt(x, y).Y > 0 {
				comp := floodFill(binary, x, y, visited)
				if len(comp) < 100 {
					continue
				}

				// 计算凸包和面积
				hull := convexHull(comp)
				if len(hull) < 4 {
					continue
				}
				hullArea := polygonArea(hull)

				// 过滤太小的区域
				if hullArea < imgArea*minAreaRatio {
					continue
				}

				// 计算矩形度（凸包面积 / 外接矩形面积）
				minX, minY, maxX, maxY := boundsOf(comp)
				bboxArea := float64((maxX-minX+1)*(maxY-minY+1))
				rectangularity := hullArea / bboxArea

				// 矩形度应该接近1（矩形），但考虑到透视可能低一些
				if rectangularity < 0.4 {
					continue
				}

				// 评分：面积越大越好，矩形度越高越好
				score := hullArea * rectangularity
				if score > bestScore {
					bestScore = score
					bestComponent = comp
				}
			}
		}
	}

	return bestComponent, bestScore
}

// boundsOf 计算点集的边界框
func boundsOf(points []image.Point) (minX, minY, maxX, maxY int) {
	if len(points) == 0 {
		return 0, 0, 0, 0
	}
	minX, minY = points[0].X, points[0].Y
	maxX, maxY = minX, minY
	for _, p := range points[1:] {
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	return
}

// cross 计算向量 OA × OB 的叉积（z 分量）
func cross(o, a, b image.Point) int {
	return (a.X-o.X)*(b.Y-o.Y) - (a.Y-o.Y)*(b.X-o.X)
}

// convexHull 使用 Andrew 单调链算法计算点集的凸包
// 返回按逆时针顺序排列的凸包顶点
func convexHull(points []image.Point) []image.Point {
	if len(points) < 3 {
		return points
	}

	sorted := make([]image.Point, len(points))
	copy(sorted, points)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].X != sorted[j].X {
			return sorted[i].X < sorted[j].X
		}
		return sorted[i].Y < sorted[j].Y
	})

	var lower []image.Point
	for _, p := range sorted {
		for len(lower) >= 2 && cross(lower[len(lower)-2], lower[len(lower)-1], p) <= 0 {
			lower = lower[:len(lower)-1]
		}
		lower = append(lower, p)
	}

	var upper []image.Point
	for i := len(sorted) - 1; i >= 0; i-- {
		p := sorted[i]
		for len(upper) >= 2 && cross(upper[len(upper)-2], upper[len(upper)-1], p) <= 0 {
			upper = upper[:len(upper)-1]
		}
		upper = append(upper, p)
	}

	if len(lower) > 0 {
		lower = lower[:len(lower)-1]
	}
	if len(upper) > 0 {
		upper = upper[:len(upper)-1]
	}

	return append(lower, upper...)
}

// pointToLineDistance 计算点 p 到线段 ab 的垂直距离
func pointToLineDistance(p, a, b image.Point) float64 {
	dx := float64(b.X - a.X)
	dy := float64(b.Y - a.Y)
	nominator := math.Abs(dy*float64(p.X-a.X) - dx*float64(p.Y-a.Y))
	if dx == 0 && dy == 0 {
		return math.Sqrt(float64((p.X-a.X)*(p.X-a.X) + (p.Y-a.Y)*(p.Y-a.Y)))
	}
	return nominator / math.Sqrt(dx*dx+dy*dy)
}

// douglasPeucker 递归实现道格拉斯-普克算法简化折线
func douglasPeucker(points []image.Point, epsilon float64) []image.Point {
	if len(points) <= 2 {
		return points
	}

	maxDist := 0.0
	maxIdx := 0
	for i := 1; i < len(points)-1; i++ {
		dist := pointToLineDistance(points[i], points[0], points[len(points)-1])
		if dist > maxDist {
			maxDist = dist
			maxIdx = i
		}
	}

	if maxDist > epsilon {
		left := douglasPeucker(points[:maxIdx+1], epsilon)
		right := douglasPeucker(points[maxIdx:], epsilon)
		return append(left[:len(left)-1], right...)
	}

	return []image.Point{points[0], points[len(points)-1]}
}

// SimplifyContour 使用道格拉斯-普克算法简化轮廓
func SimplifyContour(contour []image.Point, epsilon float64) []image.Point {
	if len(contour) <= 2 {
		return contour
	}
	closed := make([]image.Point, len(contour)+1)
	copy(closed, contour)
	closed[len(contour)] = contour[0]

	result := douglasPeucker(closed, epsilon)
	if len(result) > 1 && result[0] == result[len(result)-1] {
		result = result[:len(result)-1]
	}
	return result
}

// CornerOrder 将四个角点按顺序排列：左上、右上、右下、左下
func CornerOrder(corners [4]image.Point) [4]image.Point {
	cx := 0
	cy := 0
	for _, p := range corners {
		cx += p.X
		cy += p.Y
	}
	cx /= 4
	cy /= 4

	type anglePoint struct {
		p     image.Point
		angle float64
	}
	aps := make([]anglePoint, 4)
	for i, p := range corners {
		angle := math.Atan2(float64(p.Y-cy), float64(p.X-cx))
		aps[i] = anglePoint{p, angle}
	}
	sort.Slice(aps, func(i, j int) bool {
		return aps[i].angle < aps[j].angle
	})

	return [4]image.Point{
		aps[0].p, // 左上
		aps[1].p, // 右上
		aps[2].p, // 右下
		aps[3].p, // 左下
	}
}

// findCornersFromComponent 从连通分量中提取四个角点（改进版）
// 步骤：二分图边界追踪 → 4 段分割 → 直线拟合 → 求交获得精确角点
func findCornersFromComponent(component []image.Point, imgW, imgH int, targetAspect float64) ([4]image.Point, bool) {
	if len(component) < 50 {
		return [4]image.Point{}, false
	}

	// 用凸包 + DP 简化获得初始角点（用于边界分段）
	hull := convexHull(component)
	if len(hull) < 4 {
		return [4]image.Point{}, false
	}

	imgDiagonal := math.Sqrt(float64(imgW*imgW + imgH*imgH))
	epsilon := imgDiagonal * 0.015
	initCorners := SimplifyContour(hull, epsilon)

	// 逐步增大 epsilon 直到得到 4 个角点
	for len(initCorners) > 4 && epsilon < imgDiagonal*0.2 {
		epsilon += imgDiagonal * 0.01
		initCorners = SimplifyContour(hull, epsilon)
	}
	for len(initCorners) < 4 && epsilon < imgDiagonal*0.3 {
		epsilon += imgDiagonal * 0.01
		initCorners = SimplifyContour(hull, epsilon)
	}

	if len(initCorners) < 4 {
		return [4]image.Point{}, false
	}

	// 取前 4 个并按角度排序
	var temp [4]image.Point
	for i := 0; i < 4 && i < len(initCorners); i++ {
		temp[i] = initCorners[i]
	}
	initOrdered := CornerOrder(temp)

	// 用直线拟合 + 求交来精化角点
	refined := refineCornersByLineFitting(component, hull, initOrdered, imgW, imgH)
	if refined == nil {
		if corners, ok := validateCorners(initOrdered[:], imgW, imgH, targetAspect); ok {
			return corners, true
		}
		return [4]image.Point{}, false
	}

	ordered := CornerOrder(*refined)

	// 检查精化后的角点是否在图像边界上
	edgeMargin := min(5, min(imgW, imgH)/100)
	edgeCount := 0
	for _, p := range ordered {
		if p.X <= edgeMargin || p.X >= imgW-1-edgeMargin ||
			p.Y <= edgeMargin || p.Y >= imgH-1-edgeMargin {
			edgeCount++
		}
	}
	if edgeCount >= 3 {
		if corners, ok := validateCorners(initOrdered[:], imgW, imgH, targetAspect); ok {
			return corners, true
		}
		return [4]image.Point{}, false
	}

	// 对精化后的角点先用标准验证，失败则尝试放宽宽高比限制
	if corners, ok := validateCorners(ordered[:], imgW, imgH, targetAspect); ok {
		return corners, true
	}
	if corners, ok := validateCornersRelaxed(ordered[:], imgW, imgH, targetAspect); ok {
		return corners, true
	}

	// 精化后验证失败，退回初始角点
	if corners, ok := validateCorners(initOrdered[:], imgW, imgH, targetAspect); ok {
		return corners, true
	}

	return [4]image.Point{}, false
}



// validateCorners 验证并排序四个角点
func validateCorners(points []image.Point, imgW, imgH int, targetAspect float64) ([4]image.Point, bool) {
	if len(points) < 4 {
		return [4]image.Point{}, false
	}

	var corners [4]image.Point
	copy(corners[:], points[:4])

	ordered := CornerOrder(corners)

	// 验证面积
	polyArea := polygonArea(ordered[:])
	imgArea := float64(imgW * imgH)
	if polyArea < imgArea*0.03 || polyArea > imgArea*0.98 {
		return [4]image.Point{}, false
	}

	// 验证边长
	width := distance(ordered[0], ordered[1])
	height := distance(ordered[0], ordered[3])
	if width < 2 || height < 2 {
		return [4]image.Point{}, false
	}

	// 验证宽高比
	aspect := width / height
	if aspect < targetAspect*0.4 || aspect > targetAspect*1.6 {
		return [4]image.Point{}, false
	}

	return ordered, true
}

// validateCornersRelaxed 与 validateCorners 相同但放宽宽高比限制
// 用于验证经过直线拟合精化的角点，透视变形会导致宽高比偏离标准值
func validateCornersRelaxed(points []image.Point, imgW, imgH int, targetAspect float64) ([4]image.Point, bool) {
	if len(points) < 4 {
		return [4]image.Point{}, false
	}

	var corners [4]image.Point
	copy(corners[:], points[:4])

	ordered := CornerOrder(corners)

	// 验证面积（与标准验证相同）
	polyArea := polygonArea(ordered[:])
	imgArea := float64(imgW * imgH)
	if polyArea < imgArea*0.03 || polyArea > imgArea*0.98 {
		return [4]image.Point{}, false
	}

	// 验证边长
	width := distance(ordered[0], ordered[1])
	height := distance(ordered[0], ordered[3])
	if width < 2 || height < 2 {
		return [4]image.Point{}, false
	}

	// 放宽宽高比验证（0.3x ~ 3.0x，适应透视变形）
	aspect := width / height
	if aspect < targetAspect*0.3 || aspect > targetAspect*3.0 {
		return [4]image.Point{}, false
	}

	// 额外检查：用对边距离交叉验证
	oppositeW := distance(ordered[3], ordered[2])
	oppositeH := distance(ordered[1], ordered[2])
	oppositeAspect := oppositeW / oppositeH
	if oppositeAspect < targetAspect*0.3 || oppositeAspect > targetAspect*3.0 {
		return [4]image.Point{}, false
	}

	return ordered, true
}

// polygonArea 计算多边形面积
func polygonArea(vertices []image.Point) float64 {
	n := len(vertices)
	if n < 3 {
		return 0
	}
	area := 0.0
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += float64(vertices[i].X*vertices[j].Y - vertices[j].X*vertices[i].Y)
	}
	return math.Abs(area) / 2.0
}

// distance 计算两点之间的欧几里得距离
func distance(a, b image.Point) float64 {
	dx := float64(a.X - b.X)
	dy := float64(a.Y - b.Y)
	return math.Sqrt(dx*dx + dy*dy)
}

// refineCornersByLineFitting 通过边界点集分段 → 直线拟合 → 直线求交
// 获得亚像素精度的四个角点，比凸包简化更精确
// imgW, imgH: 原始图像宽高，用于过滤图像硬边界附近的污染点
func refineCornersByLineFitting(component []image.Point, hull []image.Point, initCorners [4]image.Point, imgW, imgH int) *[4]image.Point {
	// 建立点集快速查表
	pointSet := make(map[image.Point]bool, len(component))
	for _, p := range component {
		pointSet[p] = true
	}

	// 提取边界点（至少有一个四邻域不在集合中）
	boundarySet := make(map[image.Point]bool)
	for _, p := range component {
		if !pointSet[image.Point{X: p.X + 1, Y: p.Y}] ||
			!pointSet[image.Point{X: p.X - 1, Y: p.Y}] ||
			!pointSet[image.Point{X: p.X, Y: p.Y + 1}] ||
			!pointSet[image.Point{X: p.X, Y: p.Y - 1}] {
			boundarySet[p] = true
		}
	}

	if len(boundarySet) < 20 {
		return nil
	}

	// 过滤图像硬边界附近的污染点
	// Canny 边缘检测可能检测到图像本身的硬边界（x=0, y=0 等），
	// 这些点会污染直线拟合结果，导致精化后的角点偏离真实位置
	edgeMargin := min(5, min(imgW, imgH)/100)
	filteredBoundary := make(map[image.Point]bool)
	for p := range boundarySet {
		if p.X > edgeMargin && p.X < imgW-1-edgeMargin &&
			p.Y > edgeMargin && p.Y < imgH-1-edgeMargin {
			filteredBoundary[p] = true
		}
	}
	// 如果过滤后点数太少（少于 20 个），退回使用未过滤的集合
	if len(filteredBoundary) >= 20 {
		boundarySet = filteredBoundary
	}

	ordered := CornerOrder(initCorners)

	// 将边界点分段拟合到 4 条边
	// 对每个边界点，计算到 4 条边的距离，归入最近边
	edgePoints := make([][]image.Point, 4)

	for p := range boundarySet {
		bestEdge := 0
		bestDist := math.MaxFloat64
		for i := 0; i < 4; i++ {
			a := ordered[i]
			b := ordered[(i+1)%4]
			d := pointToSegmentDistance(p, a, b)
			if d < bestDist {
				bestDist = d
				bestEdge = i
			}
		}
		edgePoints[bestEdge] = append(edgePoints[bestEdge], p)
	}

	// 每条边至少需要 5 个点才能拟合
	for i := 0; i < 4; i++ {
		if len(edgePoints[i]) < 5 {
			return nil
		}
	}

	// 对每条边做正交距离直线拟合
	// 直线表示为: ax + by + c = 0, 其中 a^2 + b^2 = 1
	type line struct{ a, b, c float64 }
	var lines [4]line

	for i := 0; i < 4; i++ {
		a, b, c := fitLineOrthogonal(edgePoints[i])
		lines[i] = line{a, b, c}
	}

	// 计算相邻直线的交点作为角点
	var refined [4]image.Point
	for i := 0; i < 4; i++ {
		j := (i + 1) % 4
		x, y := lineIntersection(lines[i].a, lines[i].b, lines[i].c,
			lines[j].a, lines[j].b, lines[j].c)
		refined[i] = image.Point{X: int(math.Round(x)), Y: int(math.Round(y))}
	}

	return &refined
}

// pointToSegmentDistance 计算点到线段的最短距离
func pointToSegmentDistance(p, a, b image.Point) float64 {
	dx := float64(b.X - a.X)
	dy := float64(b.Y - a.Y)
	lengthSq := dx*dx + dy*dy
	if lengthSq == 0 {
		return distance(p, a)
	}
	t := (float64(p.X-a.X)*dx + float64(p.Y-a.Y)*dy) / lengthSq
	t = math.Max(0, math.Min(1, t))
	nearX := float64(a.X) + t*dx
	nearY := float64(a.Y) + t*dy
	return math.Sqrt((float64(p.X)-nearX)*(float64(p.X)-nearX) + (float64(p.Y)-nearY)*(float64(p.Y)-nearY))
}

// fitLineOrthogonal 对点集做正交距离直线拟合
// 返回直线 ax + by + c = 0 的系数 (a, b, c)，其中 a^2 + b^2 = 1
func fitLineOrthogonal(points []image.Point) (float64, float64, float64) {
	n := len(points)
	if n < 2 {
		return 1, 0, 0
	}

	// 计算质心
	var cx, cy float64
	for _, p := range points {
		cx += float64(p.X)
		cy += float64(p.Y)
	}
	cx /= float64(n)
	cy /= float64(n)

	// 计算协方差矩阵 [xx xy; xy yy]
	var xx, xy, yy float64
	for _, p := range points {
		dx := float64(p.X) - cx
		dy := float64(p.Y) - cy
		xx += dx * dx
		xy += dx * dy
		yy += dy * dy
	}

	// 最小特征值对应的特征向量是直线的法向量
	// 主特征向量沿直线方向，次特征向量垂直于直线（法向量）
	// atan2(2*xy, xx-yy) 给出主特征向量方向，加 π/2 得法向量方向
	theta := 0.5 * math.Atan2(2*xy, xx-yy)
	// 主特征向量方向 + π/2 = 法向量方向
	theta += math.Pi / 2
	a := math.Cos(theta)
	b := math.Sin(theta)

	// 确保法向量指向同一方向（约定 a > 0 或 a=0 时 b > 0）
	if a < 0 || (a == 0 && b < 0) {
		a = -a
		b = -b
	}

	// c = -(a*cx + b*cy)
	c := -(a*cx + b*cy)

	return a, b, c
}

// lineIntersection 计算两条直线的交点
// 直线1: a1*x + b1*y + c1 = 0
// 直线2: a2*x + b2*y + c2 = 0
func lineIntersection(a1, b1, c1, a2, b2, c2 float64) (float64, float64) {
	denom := a1*b2 - a2*b1
	if math.Abs(denom) < 1e-12 {
		// 平行线，返回原点
		return 0, 0
	}
	x := (b1*c2 - b2*c1) / denom
	y := (c1*a2 - c2*a1) / denom
	return x, y
}

// DetectedCorners 表示检测到的角点结果
type DetectedCorners struct {
	Corners [4]image.Point
	Score   float64
	Method  string
}
