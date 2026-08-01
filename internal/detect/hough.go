package detect

import (
	"image"
	"math"
	"sort"
)

// HoughLine 霍夫直线：法线式 x*cosθ + y*sinθ = ρ
// Theta 单位为度（0~180），Rho 单位为像素
type HoughLine struct {
	Theta float64
	Rho   float64
	Votes int
}

// houghLines 标准霍夫变换直线检测
// edges: 二值边缘图；返回按票数降序、经非极大值抑制的直线列表
func houghLines(edges *image.Gray, minVotes int) []HoughLine {
	bounds := edges.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	diag := math.Sqrt(float64(w*w + h*h))

	const thetaStep = 1.0 // 度
	numTheta := int(180 / thetaStep)
	rhoRes := 1.0 // 像素
	numRho := int(2*diag/rhoRes) + 1

	// 预计算 cos/sin 表
	cosT := make([]float64, numTheta)
	sinT := make([]float64, numTheta)
	for t := 0; t < numTheta; t++ {
		rad := float64(t) * thetaStep * math.Pi / 180
		cosT[t] = math.Cos(rad)
		sinT[t] = math.Sin(rad)
	}

	// 投票累积
	acc := make([]int, numTheta*numRho)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if edges.GrayAt(x, y).Y == 0 {
				continue
			}
			for t := 0; t < numTheta; t++ {
				rho := float64(x)*cosT[t] + float64(y)*sinT[t]
				r := int((rho + diag) / rhoRes)
				if r < 0 {
					r = 0
				} else if r >= numRho {
					r = numRho - 1
				}
				acc[t*numRho+r]++
			}
		}
	}

	// 收集达标单元，按票数降序
	type cell struct {
		t, r, votes int
	}
	var cells []cell
	for i, v := range acc {
		if v >= minVotes {
			cells = append(cells, cell{i / numRho, i % numRho, v})
		}
	}
	sort.Slice(cells, func(a, b int) bool { return cells[a].votes > cells[b].votes })

	// 贪心非极大值抑制：抑制 θ±4°、ρ±10px 邻域内的峰
	const supT, supR = 4, 10
	var lines []HoughLine
	for _, c := range cells {
		if len(lines) >= 60 {
			break
		}
		dupe := false
		for _, l := range lines {
			lt := int(math.Round(l.Theta / thetaStep))
			dt := c.t - lt
			if dt > numTheta/2 {
				dt -= numTheta
			} else if dt < -numTheta/2 {
				dt += numTheta
			}
			dr := abs(c.r - int(math.Round((l.Rho+diag)/rhoRes)))
			if abs(dt) <= supT && dr <= supR {
				dupe = true
				break
			}
		}
		if dupe {
			continue
		}
		lines = append(lines, HoughLine{
			Theta: float64(c.t) * thetaStep,
			Rho:   float64(c.r)*rhoRes - diag,
			Votes: c.votes,
		})
	}
	return lines
}

// selectCardLines 从霍夫直线中选择构成身份证边界的 4 条线
// 方式 A：按角度聚类成 2 个近似正交方向，每组取 ρ 极值（适用近平行对边）
// 方式 B（兜底）：穷举高分直线组合评分（适用透视明显、四边角度各异）
func selectCardLines(lines []HoughLine, imgW, imgH int, targetAspect float64) ([4]HoughLine, bool) {
	if quad, ok := selectParallelPairs(lines, imgW, imgH, targetAspect); ok {
		return quad, true
	}
	if quad, ok := selectBestFourLines(lines, imgW, imgH, targetAspect); ok {
		return quad, true
	}
	return [4]HoughLine{}, false
}

// selectParallelPairs 角度聚类：2 个近似正交方向组，每组取 ρ 最小/最大的两条线
func selectParallelPairs(lines []HoughLine, imgW, imgH int, targetAspect float64) ([4]HoughLine, bool) {
	type group struct {
		angle          float64
		votes          int
		minRho, maxRho HoughLine
		hasMin, hasMax bool
	}
	var groups []group
	for _, l := range lines {
		idx := -1
		for i := range groups {
			if angleClose(l.Theta, groups[i].angle) {
				idx = i
				break
			}
		}
		if idx < 0 {
			groups = append(groups, group{angle: l.Theta, votes: l.Votes, minRho: l, maxRho: l, hasMin: true, hasMax: true})
			continue
		}
		g := &groups[idx]
		g.votes += l.Votes
		if l.Rho < g.minRho.Rho {
			g.minRho = l
		}
		if l.Rho > g.maxRho.Rho {
			g.maxRho = l
		}
	}

	// 按票数降序，尝试两两组合，要求角度差接近 90°、每组都有 min/max 两条线
	sort.Slice(groups, func(a, b int) bool { return groups[a].votes > groups[b].votes })
	for i := 0; i < len(groups); i++ {
		for j := i + 1; j < len(groups); j++ {
			diff := math.Abs(cyclicAngleDiffDeg(groups[i].angle, groups[j].angle))
			if diff < 60 || diff > 120 {
				continue
			}
			if !groups[i].hasMin || !groups[i].hasMax || !groups[j].hasMin || !groups[j].hasMax {
				continue
			}
			quad := [4]HoughLine{groups[i].minRho, groups[j].minRho, groups[i].maxRho, groups[j].maxRho}
			if validLineQuad(quad, imgW, imgH, targetAspect) {
				return quad, true
			}
		}
	}
	return [4]HoughLine{}, false
}

// selectBestFourLines 穷举高分直线组合（C(n,4)，n 取前 8 条），
// 按「票数和 × 四边形质量」评分选最优，适用透视明显的四边形
func selectBestFourLines(lines []HoughLine, imgW, imgH int, targetAspect float64) ([4]HoughLine, bool) {
	if len(lines) < 4 {
		return [4]HoughLine{}, false
	}
	n := len(lines)
	if n > 8 {
		n = 8
	}

	imgArea := float64(imgW * imgH)
	best := [4]HoughLine{}
	bestScore := 0.0
	for i := 0; i < n-3; i++ {
		for j := i + 1; j < n-2; j++ {
			for k := j + 1; k < n-1; k++ {
				for l := k + 1; l < n; l++ {
					quad := [4]HoughLine{lines[i], lines[j], lines[k], lines[l]}
					if !validLineQuad(quad, imgW, imgH, targetAspect) {
						continue
					}
					pts := lineQuadCorners(quad)
					votes := float64(quad[0].Votes + quad[1].Votes + quad[2].Votes + quad[3].Votes)
					score := votes * quadScore(pts, imgArea, targetAspect)
					if score > bestScore {
						bestScore = score
						best = quad
					}
				}
			}
		}
	}
	if bestScore <= 0 {
		return [4]HoughLine{}, false
	}
	return best, true
}

// lineQuadCorners 计算 4 条直线两两相邻的交点
func lineQuadCorners(lines [4]HoughLine) [4]image.Point {
	var pts [4]image.Point
	for i := 0; i < 4; i++ {
		pts[i] = intersectHough(lines[i], lines[(i+1)%4])
	}
	return pts
}

// intersectHough 计算两条霍夫直线（法线式）的交点
func intersectHough(a, b HoughLine) image.Point {
	radA := a.Theta * math.Pi / 180
	radB := b.Theta * math.Pi / 180
	x, y := lineIntersection(math.Cos(radA), math.Sin(radA), -a.Rho,
		math.Cos(radB), math.Sin(radB), -b.Rho)
	return image.Point{X: int(math.Round(x)), Y: int(math.Round(y))}
}

// validLineQuad 校验 4 条直线能否构成合理四边形（交点位置、面积、宽高比）
func validLineQuad(lines [4]HoughLine, imgW, imgH int, targetAspect float64) bool {
	pts := lineQuadCorners(lines)

	imgDiagonal := math.Sqrt(float64(imgW*imgW + imgH*imgH))
	margin := int(imgDiagonal * 0.02)
	for _, p := range pts {
		if p.X < -margin || p.X > imgW+margin || p.Y < -margin || p.Y > imgH+margin {
			return false
		}
	}

	ordered := CornerOrder(pts)
	polyArea := polygonArea(ordered[:])
	imgArea := float64(imgW * imgH)
	if polyArea < imgArea*0.03 || polyArea > imgArea*0.98 {
		return false
	}
	width := distance(ordered[0], ordered[1])
	height := distance(ordered[0], ordered[3])
	if width < 2 || height < 2 {
		return false
	}
	aspect := width / height
	return aspect >= targetAspect*0.3 && aspect <= targetAspect*3.0
}

// cyclicAngleDiffDeg 返回两个角度（度）在 0~180 循环域上的差（-90~90）
func cyclicAngleDiffDeg(a, b float64) float64 {
	d := math.Mod(a-b, 180)
	if d < -90 {
		d += 180
	} else if d > 90 {
		d -= 180
	}
	return d
}

// angleClose 判断两个直线角度是否相近（平行容差 12°）
func angleClose(a, b float64) bool {
	return math.Abs(cyclicAngleDiffDeg(a, b)) <= 12
}
