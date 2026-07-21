package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// LinearTheme 自定义线性主题结构体
// 实现 fyne.Theme 接口，用于统一管理应用的颜色、字体、图标和尺寸
type LinearTheme struct{}

// NewLinearTheme 创建自定义主题实例
// 参数:
//   isDark - 是否为深色模式（当前未使用）
// 返回值:
//   *LinearTheme - 主题实例指针
func NewLinearTheme(isDark bool) *LinearTheme {
	return &LinearTheme{}
}

// Color 返回指定名称和变体的颜色
// 参数:
//   name - 颜色名称，如背景色、按钮色、文本色等
//   variant - 主题变体（深色/浅色）
// 返回值:
//   color.Color - 对应的颜色值
func (l *LinearTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return ColorBackground
	case theme.ColorNameButton:
		return ColorHover
	case theme.ColorNameDisabledButton:
		return ColorSurface
	case theme.ColorNameHover:
		return ColorHover
	case theme.ColorNamePressed:
		return ColorPressed
	case theme.ColorNameFocus:
		return ColorHover
	case theme.ColorNameSelection:
		return ColorSelectedBg
	case theme.ColorNameSeparator:
		return ColorDivider
	case theme.ColorNameInputBackground:
		return ColorBackground
	case theme.ColorNameInputBorder:
		return ColorBorder
	case theme.ColorNamePlaceHolder:
		return ColorPlaceholder
	case theme.ColorNameForeground:
		return ColorText
	case theme.ColorNameDisabled:
		return ColorTextSecondary
	case theme.ColorNamePrimary:
		return ColorText
	case theme.ColorNameError:
		return color.NRGBA{0xEF, 0x44, 0x44, 0xFF}
	case theme.ColorNameSuccess:
		return color.NRGBA{0x22, 0xC5, 0x5E, 0xFF}
	case theme.ColorNameWarning:
		return color.NRGBA{0xF5, 0x9E, 0x0B, 0xFF}
	case theme.ColorNameScrollBar:
		return ColorBorder
	case theme.ColorNameHeaderBackground:
		return ColorSurface
	case theme.ColorNameMenuBackground:
		return ColorBackground
	case theme.ColorNameOverlayBackground:
		return ColorBackground
	default:
		return ColorText
	}
}

// Font 返回指定样式的字体资源
// 参数:
//   name - 字体样式（普通、粗体、斜体等）
// 返回值:
//   fyne.Resource - 字体资源
func (l *LinearTheme) Font(name fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(name)
}

// Icon 返回指定名称的图标资源
// 参数:
//   name - 图标名称
// 返回值:
//   fyne.Resource - 图标资源
func (l *LinearTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// Size 返回指定名称的尺寸值
// 参数:
//   name - 尺寸名称，如内边距、图标大小、文本大小等
// 返回值:
//   float32 - 尺寸值（像素）
func (l *LinearTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return SpacingM
	case theme.SizeNameInlineIcon:
		return 20
	case theme.SizeNameScrollBar:
		return 8
	case theme.SizeNameScrollBarSmall:
		return 4
	case theme.SizeNameText:
		return 16
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameSubHeadingText:
		return 18
	case theme.SizeNameCaptionText:
		return 14
	default:
		return theme.DefaultTheme().Size(name)
	}
}