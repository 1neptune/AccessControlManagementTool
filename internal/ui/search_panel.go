package ui

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// SearchPanel 搜索面板组件结构体
// 提供服务器列表的多条件搜索功能
type SearchPanel struct {
	container       *fyne.Container   // 搜索面板容器
	searchEntry     *widget.Entry     // 名称搜索输入框
	ipEntry         *widget.Entry     // IP搜索输入框
	statusSelect    *widget.Select    // 状态下拉选择框（全部/在线/离线）
	typeSelect      *widget.Select    // 类型下拉选择框（全部/Linux/Windows）
	searchBtn       *widget.Button    // 搜索按钮
	onSearch        func(name, ip, status, osType string)  // 搜索回调函数
}

// NewSearchPanel 创建搜索面板组件
// 参数:
//   onSearch - 搜索条件变更时的回调函数
// 返回值:
//   *SearchPanel - 搜索面板实例指针
func NewSearchPanel(onSearch func(name, ip, status, osType string)) *SearchPanel {
	log.Printf("[SearchPanel] 开始创建搜索面板组件")

	sp := &SearchPanel{
		onSearch: onSearch,
	}

	sp.createComponents()
	sp.createLayout()

	log.Printf("[SearchPanel] 搜索面板组件创建完成")
	return sp
}

// createComponents 创建搜索面板的子组件
// 包括名称输入框、IP输入框、状态选择框、类型选择框和搜索按钮
func (sp *SearchPanel) createComponents() {
	sp.searchEntry = widget.NewEntry()
	sp.searchEntry.SetPlaceHolder("输入名称")

	sp.ipEntry = widget.NewEntry()
	sp.ipEntry.SetPlaceHolder("输入IP")

	sp.statusSelect = widget.NewSelect([]string{"全部", "在线", "离线"}, nil)
	sp.statusSelect.SetSelected("全部")
	sp.statusSelect.OnChanged = func(s string) {
		log.Printf("[SearchPanel] 状态筛选: %s", s)
		sp.performSearch()
	}

	sp.typeSelect = widget.NewSelect([]string{"全部", "Linux", "Windows"}, nil)
	sp.typeSelect.SetSelected("全部")
	sp.typeSelect.OnChanged = func(s string) {
		log.Printf("[SearchPanel] 类型筛选: %s", s)
		sp.performSearch()
	}

	sp.searchBtn = widget.NewButtonWithIcon("", theme.SearchIcon(), func() {
		log.Printf("[SearchPanel] 用户点击搜索按钮")
		sp.performSearch()
	})
	sp.searchBtn.Resize(fyne.NewSize(InputHeight, InputHeight))

	sp.searchEntry.OnSubmitted = func(s string) {
		log.Printf("[SearchPanel] 名称输入框按Enter键")
		sp.performSearch()
	}

	sp.ipEntry.OnSubmitted = func(s string) {
		log.Printf("[SearchPanel] IP输入框按Enter键")
		sp.performSearch()
	}
}

// createLayout 创建搜索面板的布局
// 将各子组件按水平方向排列，每个输入框带有边框装饰
func (sp *SearchPanel) createLayout() {
	nameBorder := canvas.NewRectangle(ColorBorder)
	nameBorder.CornerRadius = CornerRadius
	nameBorder.SetMinSize(fyne.NewSize(InputMinWidth, InputHeight))
	nameField := container.NewStack(nameBorder, container.NewPadded(sp.searchEntry))

	ipBorder := canvas.NewRectangle(ColorBorder)
	ipBorder.CornerRadius = CornerRadius
	ipBorder.SetMinSize(fyne.NewSize(InputMinWidth, InputHeight))
	ipField := container.NewStack(ipBorder, container.NewPadded(sp.ipEntry))

	typeBorder := canvas.NewRectangle(ColorBorder)
	typeBorder.CornerRadius = CornerRadius
	typeBorder.SetMinSize(fyne.NewSize(SelectMinWidth, InputHeight))
	typeField := container.NewStack(typeBorder, container.NewPadded(sp.typeSelect))

	statusBorder := canvas.NewRectangle(ColorBorder)
	statusBorder.CornerRadius = CornerRadius
	statusBorder.SetMinSize(fyne.NewSize(SelectMinWidth, InputHeight))
	statusField := container.NewStack(statusBorder, container.NewPadded(sp.statusSelect))

	btnBorder := canvas.NewRectangle(ColorBorder)
	btnBorder.CornerRadius = CornerRadius
	btnBorder.SetMinSize(fyne.NewSize(InputHeight, InputHeight))
	btnField := container.NewStack(btnBorder, container.NewPadded(sp.searchBtn))

	hboxContent := container.NewHBox(
		NewHSpace(SpacingM),
		NewLabel("名称"),
		NewHSpace(SpacingXS),
		nameField,
		NewHSpace(SpacingL),
		NewLabel("IP"),
		NewHSpace(SpacingXS),
		ipField,
		NewHSpace(SpacingL),
		NewLabel("类型"),
		NewHSpace(SpacingXS),
		typeField,
		NewHSpace(SpacingL),
		NewLabel("状态"),
		NewHSpace(SpacingXS),
		statusField,
		NewHSpace(SpacingS),
		btnField,
		NewHSpace(SpacingM),
	)

	bg := canvas.NewRectangle(ColorBackground)
	bg.CornerRadius = CornerRadius

	border := canvas.NewRectangle(ColorBorder)
	border.CornerRadius = CornerRadius

	sp.container = container.NewMax(
		bg,
		container.NewStack(border, hboxContent),
	)
}

// performSearch 执行搜索操作
// 获取当前各搜索条件的值，并调用回调函数触发搜索
func (sp *SearchPanel) performSearch() {
	if sp.onSearch == nil {
		log.Printf("[SearchPanel] 搜索回调未设置，跳过搜索")
		return
	}

	name := sp.searchEntry.Text
	ip := sp.ipEntry.Text
	status := sp.statusSelect.Selected
	osType := sp.typeSelect.Selected

	log.Printf("[SearchPanel] 执行搜索 - 名称: '%s', IP: '%s', 状态: '%s', 类型: '%s'",
		name, ip, status, osType)

	sp.onSearch(name, ip, status, osType)
}

// Container 返回搜索面板容器
// 返回值:
//   *fyne.Container - 搜索面板的根容器
func (sp *SearchPanel) Container() *fyne.Container {
	return sp.container
}

// GetSearchEntry 获取名称搜索输入框
// 返回值:
//   *widget.Entry - 名称搜索输入框组件
func (sp *SearchPanel) GetSearchEntry() *widget.Entry {
	return sp.searchEntry
}

// GetIPEntry 获取IP搜索输入框
// 返回值:
//   *widget.Entry - IP搜索输入框组件
func (sp *SearchPanel) GetIPEntry() *widget.Entry {
	return sp.ipEntry
}

// GetStatusSelect 获取状态下拉选择框
// 返回值:
//   *widget.Select - 状态选择框组件
func (sp *SearchPanel) GetStatusSelect() *widget.Select {
	return sp.statusSelect
}

// GetTypeSelect 获取类型下拉选择框
// 返回值:
//   *widget.Select - 类型选择框组件
func (sp *SearchPanel) GetTypeSelect() *widget.Select {
	return sp.typeSelect
}

// GetSearchBtn 获取搜索按钮
// 返回值:
//   *widget.Button - 搜索按钮组件
func (sp *SearchPanel) GetSearchBtn() *widget.Button {
	return sp.searchBtn
}