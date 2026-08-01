package idcard

// 身份证标准尺寸（单位：毫米）
const (
	CardWidthMM    = 85.6
	CardHeightMM   = 54.0
	CornerRadiusMM = 3.18
)

// 输出分辨率（DPI），350DPI 满足打印需求
const OutputDPI = 350

// 输出像素尺寸（由 DPI 和毫米换算得出）
var (
	CardWidthPx    = roundMMToPixels(CardWidthMM)
	CardHeightPx   = roundMMToPixels(CardHeightMM)
	CornerRadiusPx = roundMMToPixels(CornerRadiusMM)
)

// roundMMToPixels 将毫米值按全局 DPI 转换为像素并取整
func roundMMToPixels(mm float64) int {
	return int(mm / 25.4 * float64(OutputDPI))
}
