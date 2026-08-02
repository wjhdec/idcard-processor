// Copyright 2026 wjhdec
// SPDX-License-Identifier: Apache-2.0

package idcard

// 身份证标准尺寸（单位：毫米）
const (
	CardWidthMM    = 85.6
	CardHeightMM   = 54.0
	CornerRadiusMM = 3.18
)

// 输出分辨率（DPI），350DPI 满足打印需求
const OutputDPI = 350

// DocConfig 描述一种输出画幅配置
type DocConfig struct {
	Key            string  // 唯一标识（前端通过 key 选择）
	Name           string  // 显示名称
	WidthMM        float64 // 画幅宽（毫米）；CustomSize 模式下不适用
	HeightMM       float64 // 画幅高（毫米）
	CornerRadiusMM float64 // 圆角半径（毫米）；0 表示直角
	CustomSize     bool    // true = 自定义：宽高由前端输入（毫米），不适用预设尺寸
}

// DocConfigs 常见证件与照片画幅配置表（索引 0 作为未知 key 的默认回退）
var DocConfigs = []DocConfig{
	{Key: "idcard", Name: "身份证", WidthMM: 85.6, HeightMM: 54.0, CornerRadiusMM: 3.18},
	{Key: "driver", Name: "驾驶证", WidthMM: 88.0, HeightMM: 60.0, CornerRadiusMM: 3.0},
	{Key: "bankcard", Name: "银行卡", WidthMM: 85.6, HeightMM: 54.0, CornerRadiusMM: 3.18},
	{Key: "social", Name: "社保卡", WidthMM: 85.6, HeightMM: 54.0, CornerRadiusMM: 3.18},
	{Key: "pass", Name: "港澳通行证", WidthMM: 85.6, HeightMM: 54.0, CornerRadiusMM: 3.18},
	{Key: "oneinch", Name: "一寸照", WidthMM: 25.0, HeightMM: 35.0},
	{Key: "smallone", Name: "小一寸", WidthMM: 22.0, HeightMM: 32.0},
	{Key: "bigone", Name: "大一寸", WidthMM: 33.0, HeightMM: 48.0},
	{Key: "twoinch", Name: "二寸照", WidthMM: 35.0, HeightMM: 49.0},
	{Key: "smalltwo", Name: "小二寸", WidthMM: 35.0, HeightMM: 45.0},
	{Key: "bigtwo", Name: "大二寸", WidthMM: 35.0, HeightMM: 53.0},
	{Key: "custom", Name: "自定义", CustomSize: true},
}

// GetDocConfig 按 key 查询证件配置
func GetDocConfig(key string) (DocConfig, bool) {
	for _, c := range DocConfigs {
		if c.Key == key {
			return c, true
		}
	}
	return DocConfig{}, false
}
