package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const (
	SpacingXS = 4
	SpacingS  = 8
	SpacingM  = 16
	SpacingL  = 24
	SpacingXL = 32

	InputHeight       = 36
	ButtonHeight      = 36
	ButtonMinWidth    = 80
	InputMinWidth     = 280
	SelectMinWidth    = 150
	CornerRadius      = 4
	BorderWidth       = 1
	RowHeight         = 40
)

var (
	ColorBackground    = color.NRGBA{0xFF, 0xFF, 0xFF, 0xFF}
	ColorSurface       = color.NRGBA{0xFA, 0xFA, 0xFA, 0xFF}
	ColorBorder        = color.NRGBA{0xE5, 0xE7, 0xEB, 0xFF}
	ColorBorderHover   = color.NRGBA{0xD1, 0xD5, 0xDB, 0xFF}
	ColorText          = color.NRGBA{0x1F, 0x29, 0x37, 0xFF}
	ColorTextSecondary = color.NRGBA{0x6B, 0x72, 0x80, 0xFF}
	ColorPlaceholder   = color.NRGBA{0x9C, 0xA3, 0xAF, 0xFF}
	ColorHover         = color.NRGBA{0xF9, 0xFA, 0xFB, 0xFF}
	ColorPressed       = color.NRGBA{0xF3, 0xF4, 0xF6, 0xFF}
	ColorSelected      = color.NRGBA{0xF3, 0xF4, 0xF6, 0xFF}
	ColorSelectedBg    = color.NRGBA{0xF3, 0xF4, 0xF6, 0xFF}
	ColorFocus         = color.NRGBA{0xE8, 0xF0, 0xFE, 0xFF}
	ColorDivider       = color.NRGBA{0xF3, 0xF4, 0xF6, 0xFF}
)

// NewButton 创建标准按钮
// 参数:
//   label - 按钮显示文本
//   onTapped - 点击回调函数
// 返回值:
//   *widget.Button - Fyne按钮组件
func NewButton(label string, onTapped func()) *widget.Button {
	btn := widget.NewButton(label, onTapped)
	btn.Importance = widget.MediumImportance
	btn.Resize(fyne.NewSize(ButtonMinWidth, ButtonHeight))
	return btn
}

// NewButtonWithIcon 创建带图标的按钮
// 参数:
//   label - 按钮显示文本
//   icon - 图标资源
//   onTapped - 点击回调函数
// 返回值:
//   *widget.Button - 带图标的Fyne按钮组件
func NewButtonWithIcon(label string, icon fyne.Resource, onTapped func()) *widget.Button {
	btn := widget.NewButtonWithIcon(label, icon, onTapped)
	btn.Importance = widget.MediumImportance
	btn.Resize(fyne.NewSize(ButtonMinWidth, ButtonHeight))
	return btn
}

// NewButtonSmall 创建小型按钮
// 参数:
//   label - 按钮显示文本
//   onTapped - 点击回调函数
// 返回值:
//   *widget.Button - 宽度为60px的小型按钮
func NewButtonSmall(label string, onTapped func()) *widget.Button {
	btn := widget.NewButton(label, onTapped)
	btn.Importance = widget.MediumImportance
	btn.Resize(fyne.NewSize(60, ButtonHeight))
	return btn
}

// NewInputField 创建输入框
// 参数:
//   hint - 占位提示文本
// 返回值:
//   *widget.Entry - Fyne输入框组件
func NewInputField(hint string) *widget.Entry {
	entry := widget.NewEntry()
	entry.SetPlaceHolder(hint)
	return entry
}

// NewPasswordField 创建密码输入框
// 参数:
//   hint - 占位提示文本
// 返回值:
//   *widget.Entry - 密码输入框（内容会被隐藏）
func NewPasswordField(hint string) *widget.Entry {
	entry := widget.NewPasswordEntry()
	entry.SetPlaceHolder(hint)
	return entry
}

// NewInputFieldWithBorder 创建带边框的输入框
// 参数:
//   hint - 占位提示文本
// 返回值:
//   *fyne.Container - 包含边框和输入框的容器
func NewInputFieldWithBorder(hint string) *fyne.Container {
	entry := widget.NewEntry()
	entry.SetPlaceHolder(hint)
	entry.Resize(fyne.NewSize(InputMinWidth, InputHeight))
	
	border := canvas.NewRectangle(ColorBorder)
	border.CornerRadius = CornerRadius
	
	return container.NewStack(border, container.NewPadded(entry))
}

// NewSelectWithBorder 创建带边框的下拉选择框
// 参数:
//   options - 选项列表
//   onChanged - 选项变更回调
// 返回值:
//   *fyne.Container - 包含边框和选择框的容器
func NewSelectWithBorder(options []string, onChanged func(string)) *fyne.Container {
	sel := widget.NewSelect(options, onChanged)
	sel.Resize(fyne.NewSize(SelectMinWidth, InputHeight))
	
	border := canvas.NewRectangle(ColorBorder)
	border.CornerRadius = CornerRadius
	
	return container.NewStack(border, container.NewPadded(sel))
}

// NewTextArea 创建多行文本输入框
// 参数:
//   hint - 占位提示文本
// 返回值:
//   *widget.Entry - 多行文本输入框
func NewTextArea(hint string) *widget.Entry {
	entry := widget.NewMultiLineEntry()
	entry.SetPlaceHolder(hint)
	return entry
}

// NewSelect 创建下拉选择框
// 参数:
//   options - 选项列表
//   onChanged - 选项变更回调
// 返回值:
//   *widget.Select - Fyne下拉选择框组件
func NewSelect(options []string, onChanged func(string)) *widget.Select {
	sel := widget.NewSelect(options, onChanged)
	sel.Resize(fyne.NewSize(SelectMinWidth, InputHeight))
	return sel
}

// NewLabel 创建文本标签
// 参数:
//   text - 显示文本
// 返回值:
//   *canvas.Text - Canvas文本组件
func NewLabel(text string) *canvas.Text {
	return canvas.NewText(text, ColorText)
}

// NewLabelBold 创建粗体文本标签
// 参数:
//   text - 显示文本
// 返回值:
//   *widget.Label - 居中对齐的粗体标签
func NewLabelBold(text string) *widget.Label {
	return widget.NewLabelWithStyle(text, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
}

// NewLabelRight 创建右对齐文本标签
// 参数:
//   text - 显示文本
// 返回值:
//   *widget.Label - 右对齐的文本标签
func NewLabelRight(text string) *widget.Label {
	return widget.NewLabelWithStyle(text, fyne.TextAlignTrailing, fyne.TextStyle{})
}

// NewLabelWithHeight 创建指定高度的文本标签
// 参数:
//   text - 显示文本
//   height - 标签高度
// 返回值:
//   *fyne.Container - 包含文本的居中容器
func NewLabelWithHeight(text string, height float32) *fyne.Container {
	textCanvas := canvas.NewText(text, ColorText)
	textCanvas.TextSize = 14
	return container.NewCenter(textCanvas)
}

// NewCard 创建卡片容器
// 参数:
//   content - 卡片内容
// 返回值:
//   *fyne.Container - 带圆角边框的卡片容器
func NewCard(content fyne.CanvasObject) *fyne.Container {
	bg := canvas.NewRectangle(ColorBackground)
	bg.CornerRadius = CornerRadius

	border := canvas.NewRectangle(ColorBorder)
	border.CornerRadius = CornerRadius

	paddedContent := container.NewPadded(content)

	inner := container.NewStack(border, paddedContent)
	result := container.NewBorder(nil, nil, nil, nil, container.NewStack(bg, inner))

	return result
}

// NewCardWithPadding 创建带内边距的卡片容器
// 参数:
//   content - 卡片内容
// 返回值:
//   *fyne.Container - 带16px内边距的卡片容器
func NewCardWithPadding(content fyne.CanvasObject) *fyne.Container {
	padded := container.NewVBox(
		NewVSpace(SpacingM),
		container.NewHBox(
			NewHSpace(SpacingM),
			content,
			NewHSpace(SpacingM),
		),
		NewVSpace(SpacingM),
	)
	return NewCard(padded)
}

// NewForm 创建表单组件
// 返回值:
//   *widget.Form - Fyne表单组件
func NewForm() *widget.Form {
	form := widget.NewForm()
	return form
}

// NewFormItem 创建表单项
// 参数:
//   label - 标签文本
//   content - 表单控件
// 返回值:
//   *widget.FormItem - 表单项组件
func NewFormItem(label string, content fyne.CanvasObject) *widget.FormItem {
	return widget.NewFormItem(label, content)
}

// NewOutputPanel 创建输出面板（只读多行输入框）
// 返回值:
//   *widget.Entry - 禁用状态的多行输入框，用于显示输出信息
func NewOutputPanel() *widget.Entry {
	entry := widget.NewMultiLineEntry()
	entry.Disable()
	entry.SetPlaceHolder("配置输出将显示在这里...")
	return entry
}

// NewScrollContainer 创建滚动容器
// 参数:
//   content - 滚动内容
// 返回值:
//   *container.Scroll - 最小高度300px的滚动容器
func NewScrollContainer(content fyne.CanvasObject) *container.Scroll {
	scroll := container.NewScroll(content)
	scroll.SetMinSize(fyne.NewSize(0, 300))
	return scroll
}

// NewSeparator 创建分隔线
// 返回值:
//   *widget.Separator - Fyne分隔线组件
func NewSeparator() *widget.Separator {
	return widget.NewSeparator()
}

// NewHSpace 创建水平间距
// 参数:
//   width - 间距宽度
// 返回值:
//   fyne.CanvasObject - 透明矩形占位符
func NewHSpace(width float32) fyne.CanvasObject {
	space := canvas.NewRectangle(color.Transparent)
	space.SetMinSize(fyne.NewSize(width, 0))
	return space
}

// NewVSpace 创建垂直间距
// 参数:
//   height - 间距高度
// 返回值:
//   fyne.CanvasObject - 透明矩形占位符
func NewVSpace(height float32) fyne.CanvasObject {
	space := canvas.NewRectangle(color.Transparent)
	space.SetMinSize(fyne.NewSize(0, height))
	return space
}

// NewSpacer 创建弹性间距
// 返回值:
//   fyne.CanvasObject - 弹性扩展的间距组件
func NewSpacer() fyne.CanvasObject {
	return layout.NewSpacer()
}

// NewFlexRow 创建水平弹性布局容器
// 参数:
//   items - 子组件列表
// 返回值:
//   *fyne.Container - HBox容器
func NewFlexRow(items ...fyne.CanvasObject) *fyne.Container {
	return container.NewHBox(items...)
}

// NewFlexCol 创建垂直弹性布局容器
// 参数:
//   items - 子组件列表
// 返回值:
//   *fyne.Container - VBox容器
func NewFlexCol(items ...fyne.CanvasObject) *fyne.Container {
	return container.NewVBox(items...)
}

// NewBorderContainer 创建边框布局容器
// 参数:
//   top - 顶部组件
//   bottom - 底部组件
//   left - 左侧组件
//   right - 右侧组件
//   content - 中心内容
// 返回值:
//   *fyne.Container - Border布局容器
func NewBorderContainer(top, bottom, left, right, content fyne.CanvasObject) *fyne.Container {
	return container.NewBorder(top, bottom, left, right, content)
}

// NewPageContainer 创建页面容器
// 参数:
//   content - 页面内容
// 返回值:
//   *fyne.Container - 带上下边距的页面容器
func NewPageContainer(content fyne.CanvasObject) *fyne.Container {
	return container.NewVBox(
		NewVSpace(SpacingM),
		content,
		NewVSpace(SpacingM),
	)
}