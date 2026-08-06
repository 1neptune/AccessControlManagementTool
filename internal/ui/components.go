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

// ComboBox 可编辑下拉框组件
// 结合了文本输入框和下拉列表的功能，允许用户：
//   - 从下拉列表中选择预设值
//   - 直接输入任意数字或文本
// 实现原理:
//   - Entry组件作为文本输入主体，支持直接键入
//   - 下拉按钮触发弹出选项列表（使用widget.PopUp悬浮面板）
//   - 选中选项后自动填充Entry并触发OnChanged回调
//   - 提供与widget.Select兼容的API（Selected, SetOptions, SetSelected, Disable, Enable）
type ComboBox struct {
	Entry       *widget.Entry  // 文本输入框组件，用户可直接输入文本
	options     []string       // 下拉选项列表
	Selected    string         // 当前选中的值，与Entry文本同步
	OnChanged   func(string)   // 选项变更回调，选中或输入时触发
	disabled    bool           // 是否禁用
	dropdownBtn *widget.Button // 下拉触发按钮
	field       *fyne.Container // 完整字段容器（边框+内部布局），用于获取画布绝对位置
	popup       *widget.PopUp  // 下拉列表弹窗
	popupWidth  float32        // 弹窗宽度（与字段宽度一致）
	itemHeight  float32        // 每个选项的高度
	canvas      fyne.Canvas   // 画布引用，用于创建PopUp
}

// NewComboBox 创建可编辑下拉框
// 参数:
//   fyneCanvas - Fyne画布（从Window.Canvas()获取），用于创建PopUp弹窗
//   options - 初始选项列表
//   onChanged - 选项变更回调函数，当用户选择或输入文本时触发
// 返回:
//   *ComboBox - 可编辑下拉框实例
func NewComboBox(fyneCanvas fyne.Canvas, options []string, onChanged func(string)) *ComboBox {
	cb := &ComboBox{
		Entry:      widget.NewEntry(),
		options:    options,
		Selected:   "",
		OnChanged:  onChanged,
		itemHeight: 32,
		canvas:     fyneCanvas,
	}

	// 当用户在输入框中键入时，同步更新Selected并触发回调
	cb.Entry.OnChanged = func(s string) {
		cb.Selected = s
		if cb.OnChanged != nil {
			cb.OnChanged(s)
		}
	}

	return cb
}

// SetOptions 设置下拉选项列表
// 参数:
//   options - 新的选项列表
func (c *ComboBox) SetOptions(options []string) {
	c.options = options
}

// SetSelected 设置选中值
// 更新Entry文本并触发OnChanged回调
// 参数:
//   s - 要选中的值
func (c *ComboBox) SetSelected(s string) {
	c.Selected = s
	c.Entry.SetText(s)
}

// Disable 禁用组件（同时禁用输入框和下拉功能）
func (c *ComboBox) Disable() {
	c.disabled = true
	c.Entry.Disable()
	if c.dropdownBtn != nil {
		c.dropdownBtn.Disable()
	}
}

// Enable 启用组件（同时启用输入框和下拉功能）
func (c *ComboBox) Enable() {
	c.disabled = false
	c.Entry.Enable()
	if c.dropdownBtn != nil {
		c.dropdownBtn.Enable()
	}
}

// SetText 设置输入框文本
// 与SetSelected功能相同，用于兼容widget.Select的SetText调用
// 参数:
//   s - 要设置的文本
func (c *ComboBox) SetText(s string) {
	c.SetSelected(s)
}

// ToggleDropdown 切换下拉列表的显示/隐藏
// 点击下拉按钮时调用
// 行为:
//   - 如果弹窗已显示，则隐藏弹窗
//   - 如果弹窗未显示，则显示选项列表并定位到字段正下方
//   - 禁用状态或无选项时不响应
// 定位逻辑:
//   - 通过field容器获取在画布上的绝对位置
//   - 弹窗宽度与字段宽度一致
//   - 弹窗紧贴字段下方显示
func (c *ComboBox) ToggleDropdown() {
	if c.disabled || len(c.options) == 0 {
		return
	}

	// 如果弹窗已存在，隐藏它
	if c.popup != nil {
		c.popup.Hide()
		c.popup = nil
		return
	}

	// 创建选项列表作为弹窗内容
	// 每个选项为一个按钮，点击后自动选中并关闭弹窗
	var items []fyne.CanvasObject
	for _, opt := range c.options {
		opt := opt // 捕获循环变量，避免闭包问题
		btn := widget.NewButton(opt, func() {
			c.SetSelected(opt)
			if c.popup != nil {
				c.popup.Hide()
				c.popup = nil
			}
		})
		btn.Resize(fyne.NewSize(c.popupWidth, c.itemHeight))
		items = append(items, btn)
	}

	// 弹窗内容：VBox包裹选项按钮，带内边距
	content := container.NewPadded(container.NewVBox(items...))
	c.popup = widget.NewPopUp(content, c.canvas)

	// 计算弹窗尺寸
	popupHeight := float32(len(c.options))*c.itemHeight + 16 // 16px for padding

	// 保存引用避免闭包问题
	popup := c.popup
	field := c.field
	popupW := c.popupWidth
	popupH := popupHeight

	// 第一次Do：显示弹窗
	// 第二次Do：等待布局完成后，用field容器位置定位弹窗到字段正下方
	fyne.Do(func() {
		popup.Show()

		fyne.Do(func() {
			if field == nil {
				return
			}
			// 获取字段容器在画布上的绝对位置和尺寸
			fieldPos := field.Position()
			fieldSize := field.Size()

			// 设置弹窗尺寸与字段一致，位置紧贴字段下方
			popup.Resize(fyne.NewSize(popupW, popupH))
			popup.Move(fyne.NewPos(fieldPos.X, fieldPos.Y+fieldSize.Height))
		})
	})
}

// NewComboBoxField 创建带边框的可编辑下拉框字段
// 组合了Entry输入框、下拉按钮和边框容器，返回完整的UI组件
// 参数:
//   fyneCanvas - Fyne画布（从Window.Canvas()获取），用于创建PopUp弹窗
//   options - 初始选项列表
//   onChanged - 选项变更回调
//   width - 字段总宽度
//   height - 字段总高度
// 返回:
//   *ComboBox - 可编辑下拉框实例（用于编程式控制）
//   *fyne.Container - 完整的视觉组件（边框+输入框+下拉按钮）
func NewComboBoxField(fyneCanvas fyne.Canvas, options []string, onChanged func(string), width, height float32) (*ComboBox, *fyne.Container) {
	cb := NewComboBox(fyneCanvas, options, onChanged)

	// 下拉按钮宽度30px，在输入框左侧（前边），Entry占据剩余空间
	btnWidth := float32(30)
	entryWidth := width - btnWidth

	// 创建下拉按钮（▾图标），位于输入框前边（左侧）
	dropdownBtn := widget.NewButton("▾", func() { cb.ToggleDropdown() })
	dropdownBtn.Resize(fyne.NewSize(btnWidth, height))
	cb.dropdownBtn = dropdownBtn

	// 设置Entry尺寸
	cb.Entry.Resize(fyne.NewSize(entryWidth, height))

	// 设置弹窗宽度与总宽度一致
	cb.popupWidth = width

	// 创建边框容器
	border := canvas.NewRectangle(ColorBorder)
	border.CornerRadius = CornerRadius
	border.SetMinSize(fyne.NewSize(width, height))

	// 创建内部布局：左侧下拉按钮 + 右侧Entry
	// 下拉按钮在输入框前边（左侧），选择选项后值填入输入框
	inner := container.NewBorder(nil, nil, dropdownBtn, nil, cb.Entry)

	// 创建完整组件：边框 + 内部布局
	field := container.NewStack(border, inner)
	cb.field = field

	return cb, field
}