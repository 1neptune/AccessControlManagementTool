package ui

import (
	"access-control-tool/internal/firewall"
	"access-control-tool/internal/models"
	"access-control-tool/internal/portscanner"
	"access-control-tool/internal/smb"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ConfigWindow 配置窗口结构体，用于配置服务器访问控制策略
type ConfigWindow struct {
	app         fyne.App                    // Fyne应用实例
	server      *models.Server              // 当前配置的服务器信息
	parent      fyne.Window                 // 父窗口引用
	portInfoMap map[int]portscanner.PortInfo // 端口信息映射表，key为端口号，value为端口详细信息
	configuring bool                        // 配置状态标志，防止重复点击按钮
}

// NewConfigWindow 创建配置窗口实例
// 参数:
//   app - Fyne应用实例
//   parent - 父窗口引用
// 返回:
//   ConfigWindow指针
func NewConfigWindow(app fyne.App, parent fyne.Window) *ConfigWindow {
	log.Printf("[ConfigWindow] 创建配置窗口组件")
	return &ConfigWindow{
		app:         app,
		parent:      parent,
		portInfoMap: make(map[int]portscanner.PortInfo),
	}
}

// showMessageWindow 显示消息弹窗
// 参数:
//   title - 弹窗标题
//   message - 弹窗内容
// 特性:
//   - 固定尺寸480x160，居中显示
//   - 白色背景，灰色边框，黑色文字
//   - 文本水平居中，不换行
func (cw *ConfigWindow) showMessageWindow(title, message string) {
	fyne.Do(func() {
		msgWin := cw.app.NewWindow(title)
		msgWin.SetFixedSize(true)

		msgLabel := widget.NewLabel(message)
		msgLabel.Wrapping = fyne.TextWrapOff
		msgLabel.Alignment = fyne.TextAlignCenter

		okBtn := widget.NewButton("确定", func() {
			msgWin.Close()
		})
		okBtn.Resize(fyne.NewSize(120, InputHeight))

		okBorder := canvas.NewRectangle(ColorBorder)
		okBorder.CornerRadius = CornerRadius
		okBorder.SetMinSize(fyne.NewSize(120, InputHeight))
		okField := container.NewStack(okBorder, container.NewPadded(okBtn))

		content := container.NewVBox(
			NewVSpace(SpacingL),
			container.NewCenter(msgLabel),
			NewVSpace(SpacingL),
			container.NewCenter(okField),
			NewVSpace(SpacingL),
		)

		bg := canvas.NewRectangle(ColorBackground)
		msgWin.SetContent(container.NewStack(bg, content))

		msgWin.Resize(fyne.NewSize(480, 160))
		msgWin.CenterOnScreen()
		msgWin.Show()
	})
}

// newLabelWithAlign 创建带对齐的标签容器
// 参数:
//   text - 标签文本内容
//   width - 标签宽度
//   height - 标签高度
//   bold - 是否加粗
// 返回:
//   包含文本的容器，文本垂直居中
// 说明:
//   使用GridWrap固定尺寸，通过计算paddingTop实现文本垂直居中
func newLabelWithAlign(text string, width, height float32, bold bool) *fyne.Container {
	log.Printf("[ConfigWindow] 创建标签: text=%s, width=%.2f, height=%.2f, bold=%v", text, width, height, bold)
	textCanvas := canvas.NewText(text, ColorText)
	textCanvas.TextSize = 14
	if bold {
		textCanvas.TextStyle = fyne.TextStyle{Bold: true}
	}
	
	// 创建固定宽度的标签容器
	// 使用GridWrap强制容器使用指定的尺寸
	return container.NewGridWrap(fyne.NewSize(width, height), textCanvas)
}

// Show 显示配置窗口
// 参数:
//   server - 要配置的服务器信息
//   onClosed - 窗口关闭时的回调函数（用于通知主窗口释放服务器锁）
// 功能:
//   - 创建配置窗口界面，包含端口选择、服务名称、关联业务IP、自定义白名单输入
//   - 输出面板用于显示配置过程日志
//   - 异步加载服务器端口信息
// 布局结构:
//   - 配置卡片: 端口选择 + 服务名称 + 关联业务 + 白名单 + 配置按钮
//   - 输出卡片: 标题 + 输出文本区域
func (cw *ConfigWindow) Show(server models.Server, onClosed func()) {
	log.Printf("[ConfigWindow] 显示配置窗口，服务器: %s (%s:%d)", server.Name, server.Host, server.SSHPort)

	// 保存服务器引用
	cw.server = &server

	// 创建窗口
	windowTitle := fmt.Sprintf("配置访问控制 - %s", server.Host)
	configWin := cw.app.NewWindow(windowTitle)

	// ===== 创建表单控件 =====

	// 统一输入框宽度配置
	// labelWidth: 标签固定宽度（80px），所有标签统一此宽度
	// labelHeight: 标签高度，与输入框高度一致
	// portFieldWidth: 端口下拉框和端口输入框的宽度（各120px，两者相同
	// serviceInputWidth: 服务名称输入框宽度 = 端口输入框 × 2 = 240px
	// contentInputWidth: 关联业务/白名单输入框宽度（750px）
	// formTotalWidth: 表单区域总宽度（838px），所有行统一此宽度
	//   计算公式: labelWidth(80) + 间距(8) + contentInputWidth(750) = 838px
	// 端口行: 80 + 8 + 120(下拉) + 8 + 120(输入) + 16 + 80 + 8 + 240(服务名) = 838px？算一遍
	//   实际: 80+8+120+8+120+16+80+8+240 = 680px，剩余158px，加大端口下拉框/输入框调整
	//   调整为：portFieldWidth=160，serviceInputWidth=320，则 80+8+160+8+160+16+80+8+320 = 840px 近似838px OK
	labelWidth := float32(80)
	labelHeight := float32(InputHeight)
	portFieldWidth := float32(160)
	serviceInputWidth := portFieldWidth * 2 // 服务名称输入框 = 端口输入框 × 2 = 320px
	contentInputWidth := float32(750)
	formTotalWidth := labelWidth + 8 + contentInputWidth

	log.Printf("[ConfigWindow] 布局参数 - labelWidth=%.2f, labelHeight=%.2f, portFieldWidth=%.2f, serviceInputWidth=%.2f, contentInputWidth=%.2f, formTotalWidth=%.2f",
		labelWidth, labelHeight, portFieldWidth, serviceInputWidth, contentInputWidth, formTotalWidth)

	// 服务名称输入框（只读，由端口选择自动填充，不带滚动）
	serviceNameEntry := widget.NewEntry()
	serviceNameEntry.Disable()
	serviceNameEntry.SetPlaceHolder("服务名称")
	serviceNameEntry.Resize(fyne.NewSize(serviceInputWidth, InputHeight))
	serviceNameBorder := canvas.NewRectangle(ColorBorder)
	serviceNameBorder.CornerRadius = CornerRadius
	serviceNameBorder.SetMinSize(fyne.NewSize(serviceInputWidth, InputHeight))
	serviceNameField := container.NewStack(serviceNameBorder, serviceNameEntry)
	log.Printf("[ConfigWindow] 服务名称输入框创建完成，宽度=%.2f, 高度=%.2f", serviceInputWidth, InputHeight)

	// 关联业务IP输入框（只读，多行，超过3行显示滚动）
	// 默认高度70px（约3行，每行约20px），超过时自动显示滚动条
	// 直接使用MultiLineEntry，不包装Scroll容器，这样只有内容超出时才显示滚动条
	businessIPEntry := widget.NewMultiLineEntry()
	businessIPEntry.Disable()
	businessIPEntry.Wrapping = fyne.TextWrapWord
	businessIPEntry.Resize(fyne.NewSize(contentInputWidth, 70))
	businessIPBorder := canvas.NewRectangle(ColorBorder)
	businessIPBorder.CornerRadius = CornerRadius
	businessIPBorder.SetMinSize(fyne.NewSize(contentInputWidth, 70))
	businessIPField := container.NewStack(businessIPBorder, businessIPEntry)
	log.Printf("[ConfigWindow] 关联业务输入框创建完成，宽度=%.2f, 高度=%.2f", contentInputWidth, float32(70))

	// 自定义白名单输入框（必填，支持逗号分隔多个IP，不带滚动）
	ipEntry := widget.NewEntry()
	ipEntry.SetPlaceHolder("多个IP用逗号分隔，支持IP段如 192.168.1.0/24")
	ipEntry.Resize(fyne.NewSize(contentInputWidth, InputHeight))
	ipBorder := canvas.NewRectangle(ColorBorder)
	ipBorder.CornerRadius = CornerRadius
	ipBorder.SetMinSize(fyne.NewSize(contentInputWidth, InputHeight))
	ipField := container.NewStack(ipBorder, ipEntry)
	log.Printf("[ConfigWindow] 自定义白名单输入框创建完成，宽度=%.2f, 高度=%.2f", contentInputWidth, InputHeight)

	// ===== 端口控件创建 =====
	// 端口区域由两个独立控件组成：端口下拉框 + 端口输入框
	// - 端口下拉框(portSelect)：显示服务器扫描到的开放端口列表，供用户选择预设端口
	// - 端口输入框(portEntry)：标 * 必填，显示下拉框选中值，也支持用户直接输入自定义端口号
	//   配置时端口号以该输入框的值为准（非下拉框）
	// 两者宽度相同(portFieldWidth)，布局为：[下拉框][输入框]

	// 端口下拉框（widget.Select，纯选择，用于显示预设端口列表）
	portSelect := widget.NewSelect([]string{}, nil)
	portSelectBorder := canvas.NewRectangle(ColorBorder)
	portSelectBorder.CornerRadius = CornerRadius
	portSelectBorder.SetMinSize(fyne.NewSize(portFieldWidth, InputHeight))
	portSelectField := container.NewStack(portSelectBorder, portSelect)

	// 端口输入框（widget.Entry，* 必填，可编辑）
	// placeholder为"自定义端口"，提示用户可直接输入端口号
	// 配置时端口号以此输入框的值为准
	portEntry := widget.NewEntry()
	portEntry.SetPlaceHolder("自定义端口")
	portEntry.Resize(fyne.NewSize(portFieldWidth, InputHeight))
	portEntryBorder := canvas.NewRectangle(ColorBorder)
	portEntryBorder.CornerRadius = CornerRadius
	portEntryBorder.SetMinSize(fyne.NewSize(portFieldWidth, InputHeight))
	portEntryField := container.NewStack(portEntryBorder, portEntry)

	// 下拉框选择同步逻辑：
	// 当用户从下拉框选择端口时，自动将选中值填入端口输入框，
	// 并触发handlePortSelect填充服务名称和关联业务IP。
	// 这样下拉框和输入框保持同步，且配置时以输入框值为准。
	portSelect.OnChanged = func(s string) {
		portEntry.SetText(s)
		cw.handlePortSelect(s, serviceNameEntry, businessIPEntry, ipEntry)
	}
	log.Printf("[ConfigWindow] 端口下拉框和输入框创建完成，下拉框宽度=%.2f, 输入框宽度=%.2f", portFieldWidth, portFieldWidth)

	// 输出面板（多行文本框，只读）
	// 宽度使用formTotalWidth(880px)，确保右边界与表单行对齐
	outputText := widget.NewMultiLineEntry()
	outputText.Disable()
	outputBorder := canvas.NewRectangle(ColorBorder)
	outputBorder.CornerRadius = CornerRadius
	outputBorder.SetMinSize(fyne.NewSize(formTotalWidth, 200))
	outputField := container.NewStack(outputBorder, outputText)
	log.Printf("[ConfigWindow] 输出面板创建完成，宽度=%.2f, 高度=%.2f", formTotalWidth, float32(200))

	// 配置按钮（开始配置、取消配置）
	startConfigBtn := widget.NewButton("开始配置", nil)
	cancelConfigBtn := widget.NewButton("取消配置", nil)

	// 设置按钮点击事件
	startConfigBtn.OnTapped = func() {
		log.Printf("[ConfigWindow] 用户点击开始配置按钮")
		cw.configure(portSelect, portEntry, ipEntry, outputText, startConfigBtn, cancelConfigBtn)
	}

	cancelConfigBtn.OnTapped = func() {
		log.Printf("[ConfigWindow] 用户点击取消配置按钮")
		cw.deconfigure(portSelect, portEntry, ipEntry, outputText, startConfigBtn, cancelConfigBtn)
	}
	// ===== 组装表单行 =====

	// 刷新按钮（点击后重新扫描端口信息，样式与其他按钮一致）
	var refreshBtn *widget.Button
	refreshBtn = widget.NewButton("刷新端口", nil)
	// 刷新按钮回调：同时禁用端口下拉框和输入框，同步提示
	refreshBtn.OnTapped = func() {
		log.Printf("[ConfigWindow] 用户点击刷新按钮")
		fyne.Do(func() {
			portSelect.Disable()
			portEntry.Disable()
			refreshBtn.Disable()
			portEntry.SetText("正在获取...")
			serviceNameEntry.SetText("正在获取...")
			businessIPEntry.SetText("正在获取...")
			ipEntry.SetText("")
		})
		go cw.loadPorts(portSelect, portEntry, serviceNameEntry, businessIPEntry, ipEntry, refreshBtn)
	}

	// OpenSSH按钮（仅Windows服务器显示，位于开始配置按钮左侧）
	// 必须在refreshBtn声明之后创建，因为回调中需要引用refreshBtn
	var opensshBtn *widget.Button
	if server.OSType == "windows" {
		opensshBtn = widget.NewButton("OpenSSH", nil)
		opensshBtn.OnTapped = func() {
			log.Printf("[ConfigWindow] 用户点击OpenSSH按钮")
			cw.configureOpenSSH(outputText, opensshBtn, startConfigBtn, cancelConfigBtn, refreshBtn)
		}
	}

	// 重新设计布局：使用简单的HBox布局，每行独立，各组件均匀排列
	// 端口行: [端口标签(80)] + [8] + [端口下拉框(200)] + [16] + [服务名称标签(80)] + [8] + [服务名称输入框(446)]
	// 关联业务行: [关联业务标签(80)] + [8] + [关联业务输入框(750)]
	// 白名单行: [自定义白名单标签(80)] + [8] + [白名单输入框(750)]
	// 所有行总宽度统一为 838px，端口和服务名称在同一行均匀排列
	
	log.Printf("[ConfigWindow] 开始组装布局...")
	
	// 创建端口行
	// 端口行：端口*标签(必填) + 端口下拉框 + 端口输入框 + 服务名称标签 + 服务名称输入框
	// 端口下拉框和端口输入框宽度相同（各160px）
	// 服务名称输入框宽度 = 端口输入框 × 2 = 320px
	// 端口* 标签中的 * 表示必填项
	portRow := container.NewHBox(
		newLabelWithAlign("端口*", labelWidth, labelHeight, true),
		NewHSpace(8),
		portSelectField,
		NewHSpace(8),
		portEntryField,
		NewHSpace(16),
		newLabelWithAlign("服务名称", labelWidth, labelHeight, true),
		NewHSpace(8),
		serviceNameField,
	)
	log.Printf("[ConfigWindow] 端口行创建完成")
	
	// 创建关联业务行
	businessRow := container.NewHBox(
		newLabelWithAlign("关联业务", labelWidth, labelHeight, true),
		NewHSpace(8),
		businessIPField,
	)
	log.Printf("[ConfigWindow] 关联业务行创建完成")
	
	// 创建白名单行
	whitelistRow := container.NewHBox(
		newLabelWithAlign("自定义白名单", labelWidth, labelHeight, true),
		NewHSpace(8),
		ipField,
	)
	log.Printf("[ConfigWindow] 白名单行创建完成")
	
	// 将所有行放入VBox，每行独立，间距10px
	formGrid := container.NewVBox(
		portRow,
		NewVSpace(10),
		businessRow,
		NewVSpace(10),
		whitelistRow,
	)
	log.Printf("[ConfigWindow] 表单网格创建完成")

	// 按钮行（居中对齐，间距均匀）
	// Windows服务器: OpenSSH + 开始配置 + 取消配置 + 刷新端口（4个按钮均匀分布）
	// Linux服务器: 开始配置 + 取消配置 + 刷新端口（3个按钮均匀分布，保持不变）
	var configBtnBox *fyne.Container
	if server.OSType == "windows" {
		// 4个按钮: 100*4 + 20*3(间距) = 460px
		// 两侧空白: (formTotalWidth - 400) / 2
		configBtnBox = container.NewHBox(
			NewHSpace((formTotalWidth-100-100-100-100)/2),
			opensshBtn,
			NewHSpace(20),
			startConfigBtn,
			NewHSpace(20),
			cancelConfigBtn,
			NewHSpace(20),
			refreshBtn,
			NewHSpace((formTotalWidth-100-100-100-100)/2),
		)
	} else {
		// 3个按钮: 100*3 + 20*2(间距) = 340px
		// 两侧空白: (formTotalWidth - 300) / 2
		configBtnBox = container.NewHBox(
			NewHSpace((formTotalWidth-100-100-100)/2),
			startConfigBtn,
			NewHSpace(20),
			cancelConfigBtn,
			NewHSpace(20),
			refreshBtn,
			NewHSpace((formTotalWidth-100-100-100)/2),
		)
	}

	// ===== 组装卡片 =====

	// 配置卡片
	configCard := NewCard(
		container.NewBorder(
			nil, nil, nil, nil,
			container.NewVBox(
				NewVSpace(12),
				formGrid,
				NewVSpace(12),
				configBtnBox,
				NewVSpace(12),
			),
		),
	)

	// 输出卡片（输出框宽度与输入框对齐）
	outputCard := NewCard(
		container.NewBorder(
			container.NewVBox(
				NewLabelBold("输出面板"),
				NewSeparator(),
				NewVSpace(8),
			),
			nil, nil, nil,
			container.NewVBox(outputField),
		),
	)

	// ===== 组装窗口内容 =====
	content := container.NewVBox(
		configCard,
		NewVSpace(8),
		outputCard,
	)

	// 设置窗口关闭回调（通知主窗口释放服务器锁）
	configWin.SetOnClosed(func() {
		cw.configuring = false
		if onClosed != nil {
			onClosed()
		}
	})

	// 设置窗口内容和属性
	configWin.SetContent(content)
	log.Printf("[ConfigWindow] 配置窗口设置内容完成")
	
	// 获取内容实际尺寸，用于调试
	contentSize := configWin.Content().Size()
	log.Printf("[ConfigWindow] 内容实际尺寸: %v", contentSize)
	
	configWin.SetFixedSize(true)
	// 窗口宽度计算:
	//   表单行总宽度 = formTotalWidth(880px)
	//   卡片内边距(左右各8px) = 16px
	//   窗口最小宽度 = 880 + 16 = 896px，取950px保证充足空间
	//   窗口高度保持560px不变
	windowWidth := float32(950)
	windowHeight := float32(560)
	log.Printf("[ConfigWindow] 设置窗口尺寸: %.2fx%.2f", windowWidth, windowHeight)
	configWin.Resize(fyne.NewSize(windowWidth, windowHeight))
	
	// 设置后再次获取内容尺寸，用于调试
	contentSizeAfter := configWin.Content().Size()
	log.Printf("[ConfigWindow] 设置窗口尺寸后内容尺寸: %v", contentSizeAfter)
	configWin.CenterOnScreen()
	configWin.Show()
	log.Printf("[ConfigWindow] 配置窗口居中并显示完成")

	// 端口下拉框和输入框先显示"正在获取..."提示，同时禁用刷新按钮
	fyne.Do(func() {
		portSelect.Disable()
		portEntry.Disable()
		refreshBtn.Disable()
		portEntry.SetText("正在获取...")
		serviceNameEntry.SetText("正在获取...")
		businessIPEntry.SetText("正在获取...")
		ipEntry.SetText("")
	})

	// 异步加载端口信息
	go cw.loadPorts(portSelect, portEntry, serviceNameEntry, businessIPEntry, ipEntry, refreshBtn)
}

// handlePortSelect 处理端口选择事件
// 参数:
//   portStr - 选中的端口字符串
//   serviceNameEntry - 服务名称输入框
//   businessIPEntry - 关联业务IP输入框
//   ipEntry - 自定义白名单输入框
// 功能:
//   - 当用户选择端口时，先显示"正在获取..."提示
//   - 异步获取服务名称和关联业务IP
//   - 获取到数据后更新显示
//   - 端口为空时清空所有输入框
func (cw *ConfigWindow) handlePortSelect(portStr string, serviceNameEntry, businessIPEntry, ipEntry *widget.Entry) {
	// 端口为空时清空所有输入框
	if portStr == "" {
		serviceNameEntry.SetText("")
		businessIPEntry.SetText("")
		ipEntry.SetText("")
		return
	}

	// 解析端口号
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return
	}

	// 先显示"正在获取..."提示
	fyne.Do(func() {
		serviceNameEntry.SetText("正在获取...")
		businessIPEntry.SetText("正在获取...")
		ipEntry.SetText("")
	})

	// 异步获取服务名称和关联业务IP
	go cw.refreshPortInfo(port, serviceNameEntry, businessIPEntry, ipEntry)
}

// refreshPortInfo 异步刷新端口的服务名称和关联业务IP
// 参数:
//   port - 端口号
//   serviceNameEntry - 服务名称输入框
//   businessIPEntry - 关联业务IP输入框
//   ipEntry - 自定义白名单输入框
// 功能:
//   - 调用ScanPorts重新扫描获取最新的端口信息（包括服务名称和连接IP）
//   - 更新portInfoMap中的数据
//   - 更新UI显示最新的服务名称和关联业务IP
func (cw *ConfigWindow) refreshPortInfo(port int, serviceNameEntry, businessIPEntry, ipEntry *widget.Entry) {
	log.Printf("[ConfigWindow] 刷新端口 %d 的服务名称和关联业务IP", port)

	portInfoList, err := portscanner.ScanPorts(cw.server)
	if err != nil {
		log.Printf("[ConfigWindow] 刷新端口信息失败: %v", err)
		fyne.Do(func() {
			serviceNameEntry.SetText("获取失败")
			businessIPEntry.SetText("获取失败")
		})
		return
	}

	// 查找当前端口的最新信息
	var latestInfo *portscanner.PortInfo
	for _, info := range portInfoList {
		if info.Port == port {
			latestInfo = &info
			break
		}
	}

	if latestInfo == nil {
		fyne.Do(func() {
			serviceNameEntry.SetText("未找到")
			businessIPEntry.SetText("未找到")
		})
		return
	}

	// 更新portInfoMap中的数据
	cw.portInfoMap[port] = *latestInfo

	// 更新UI显示最新数据
	fyne.Do(func() {
		if latestInfo.ServiceName != "" {
			serviceNameEntry.SetText(latestInfo.ServiceName)
		} else {
			serviceNameEntry.SetText("未知")
		}

		if len(latestInfo.ConnectedIPs) > 0 {
			businessIPEntry.SetText(strings.Join(latestInfo.ConnectedIPs, "\n"))
			ipEntry.SetText(strings.Join(latestInfo.ConnectedIPs, ", "))
		} else {
			businessIPEntry.SetText("无")
			ipEntry.SetText("")
		}
		log.Printf("[ConfigWindow] 端口 %d 刷新完成，服务名称: %s, 业务IP: %v", port, latestInfo.ServiceName, latestInfo.ConnectedIPs)
	})
}

// loadPorts 异步加载服务器端口信息
// 参数:
//   portSelect - 端口选择下拉框
//   serviceNameEntry - 服务名称输入框
//   businessIPEntry - 关联业务IP输入框
//   ipEntry - 自定义白名单输入框
//   refreshBtn - 刷新按钮（加载完成后启用）
// 功能:
//   - 调用端口扫描器获取服务器开放端口列表
//   - 将端口信息存入portInfoMap供后续使用
//   - 更新端口下拉框选项和端口输入框（加载完成后启用两者和刷新按钮）
//   - 默认选中第一个端口，通过OnChanged自动同步到端口输入框
//   - 加载失败时显示错误提示并启用刷新按钮
// 注意:
//   - 在goroutine中执行，UI更新需通过fyne.Do()
//   - 调用前应先禁用refreshBtn，加载完成后自动启用
//   - 端口值同步链路：portSelect.SetSelected → OnChanged → portEntry.SetText
func (cw *ConfigWindow) loadPorts(portSelect *widget.Select, portEntry *widget.Entry, serviceNameEntry, businessIPEntry, ipEntry *widget.Entry, refreshBtn *widget.Button) {
	log.Printf("[ConfigWindow] 开始扫描服务器端口: %s:%d", cw.server.Host, cw.server.SSHPort)

	// 调用端口扫描器获取端口列表
	portInfoList, err := portscanner.ScanPorts(cw.server)
	if err != nil {
		// 扫描失败：启用控件，清空下拉框选项，输入框显示"获取失败"
		log.Printf("[ConfigWindow] 端口扫描失败: %v", err)
		fyne.Do(func() {
			portSelect.Enable()
			portEntry.Enable()
			refreshBtn.Enable()
			portSelect.SetOptions([]string{})  // 清空下拉框选项
			portEntry.SetText("获取失败")        // 输入框显示失败提示
			serviceNameEntry.SetText("获取失败")
			businessIPEntry.SetText("获取失败")
			cw.showMessageWindow("错误", "端口扫描失败")
		})
		return
	}

	log.Printf("[ConfigWindow] 端口扫描成功，发现 %d 个端口", len(portInfoList))

	// 将端口信息存入映射表（portInfoMap），供handlePortSelect使用
	// key为端口号(int)，value为端口详细信息(服务名、连接IP等)
	cw.portInfoMap = make(map[int]portscanner.PortInfo)
	portStrings := make([]string, 0, len(portInfoList))
	for _, info := range portInfoList {
		portStrings = append(portStrings, strconv.Itoa(info.Port))
		cw.portInfoMap[info.Port] = info
	}

	// 更新UI：启用端口下拉框、端口输入框和刷新按钮
	// 设置下拉框选项，默认选中第一个端口
	// SetSelected会触发OnChanged回调，自动将选中值同步到portEntry输入框
	fyne.Do(func() {
		portSelect.Enable()
		portEntry.Enable()
		refreshBtn.Enable()
		portSelect.SetOptions(portStrings)
		if len(portStrings) > 0 {
			portSelect.SetSelected(portStrings[0])
			log.Printf("[ConfigWindow] 默认选中端口: %s", portStrings[0])
			// OnChanged会自动调handlePortSelect，无需再次调用
		} else {
			portSelect.Selected = "无端口"
			portEntry.SetText("无端口")
			serviceNameEntry.SetText("无")
			businessIPEntry.SetText("无")
		}
	})
}

// configure 执行访问控制策略配置
// 参数:
//   portSelect - 端口下拉框（用于显示预设端口，不直接取值）
//   portEntry - 端口输入框（* 必填，配置时端口号以此输入框的值为准）
//   ipEntry - 自定义白名单输入框
//   outputText - 输出面板文本框
//   startBtn - 开始配置按钮
//   cancelBtn - 取消配置按钮
// 流程:
//   1. 检查配置状态，防止重复点击
//   2. 校验端口输入框（非空、数字、1-65535范围）和白名单输入
//   3. 设置配置状态，禁用按钮，清空输出面板
//   4. 在goroutine中调用firewall.Configure()执行配置
//   5. 配置完成后恢复按钮状态，显示结果弹窗
// 安全:
//   - 使用defer + recover捕获panic，确保按钮状态恢复
//   - UI更新通过fyne.Do()确保在主线程执行
func (cw *ConfigWindow) configure(portSelect *widget.Select, portEntry *widget.Entry, ipEntry *widget.Entry, outputText *widget.Entry, startBtn, cancelBtn *widget.Button) {
	log.Printf("[ConfigWindow] 开始执行配置")

	// 1. 检查配置状态，防止重复点击
	if cw.configuring {
		log.Printf("[ConfigWindow] 配置正在进行中，拒绝重复点击")
		cw.showMessageWindow("提示", "配置正在进行中，请等待完成")
		return
	}

	// 2. 端口校验（三项校验，任一项不符合则弹出警告弹窗并中止操作）
	// 端口值以端口号输入框(portEntry.Text)为准，而非下拉框
	// 校验顺序：非空 → 数字 → 端口范围(1-65535)
	portStr := strings.TrimSpace(portEntry.Text) // 去除首尾空格
	if portStr == "" {
		// 校验1：输入框不能为空
		log.Printf("[ConfigWindow] 端口输入框为空")
		cw.showMessageWindow("警告", "端口不能为空，请从下拉列表选择或输入端口号")
		return
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		// 校验2：必须是有效数字
		log.Printf("[ConfigWindow] 端口不是有效数字: %s", portStr)
		cw.showMessageWindow("警告", "端口必须是数字，请输入有效的端口号")
		return
	}

	if port < 1 || port > 65535 {
		// 校验3：端口号必须在1-65535范围内
		log.Printf("[ConfigWindow] 端口号超出范围: %d", port)
		cw.showMessageWindow("警告", "端口号必须在 1-65535 范围内")
		return
	}

	// 3. 校验白名单输入
	ipWhitelist := strings.TrimSpace(ipEntry.Text)
	if ipWhitelist == "" {
		log.Printf("[ConfigWindow] %s服务器白名单为空，将只允许127.0.0.1访问", cw.server.OSType)
		cw.configuring = true
		fyne.Do(func() {
			startBtn.Disable()
			cancelBtn.Disable()
		})
		cw.showConfirmConfigDialog(port, ipEntry, outputText, startBtn, cancelBtn)
		return
	}

	cw.doConfigure(port, ipWhitelist, outputText, startBtn, cancelBtn)
}

// doConfigure 实际执行访问控制策略配置
// 参数:
//   port - 目标端口
//   ipWhitelist - 白名单IP列表（空表示只允许127.0.0.1）
//   outputText - 输出面板文本框
//   startBtn - 开始配置按钮
//   cancelBtn - 取消配置按钮
// 流程:
//   1. 设置配置状态，禁用按钮，清空输出面板
//   2. 创建滚动防抖器和进度回调函数
//   3. 在goroutine中调用firewall.Configure()执行配置
//   4. 配置完成后恢复按钮状态，显示结果弹窗
// 安全:
//   - 使用defer + recover捕获panic，确保按钮状态恢复
//   - UI更新通过fyne.Do()确保在主线程执行
func (cw *ConfigWindow) doConfigure(port int, ipWhitelist string, outputText *widget.Entry, startBtn, cancelBtn *widget.Button) {
	log.Printf("[ConfigWindow] 开始执行配置，端口: %d, IP白名单: '%s'", port, ipWhitelist)

	cw.configuring = true
	fyne.Do(func() {
		startBtn.Disable()
		cancelBtn.Disable()
		outputText.SetText("")
	})
	log.Printf("[ConfigWindow] 配置参数 - 端口: %d, IP白名单: '%s'", port, ipWhitelist)

	debouncer := newScrollDebouncer(outputText)

	progressCallback := func(message string) {
		fyne.Do(func() {
			outputText.SetText(outputText.Text + message)
			log.Printf("[ConfigWindow] 配置输出: %s", message)
			debouncer.scrollToBottom()
		})
	}

	go func() {
		log.Printf("[ConfigWindow] 开始调用防火墙配置")

		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ConfigWindow] 配置发生panic: %v", r)
				fyne.Do(func() {
					cw.configuring = false
					startBtn.Enable()
					cancelBtn.Enable()
					outputText.SetText(outputText.Text + fmt.Sprintf("\n❌ 配置发生异常: %v\n", r))
					debouncer.scrollToBottom()
					cw.showMessageWindow("错误", "配置发生异常")
				})
			}
		}()

		result, err := firewall.Configure(cw.server, port, ipWhitelist, progressCallback)

		fyne.Do(func() {
			cw.configuring = false
			startBtn.Enable()
			cancelBtn.Enable()

			if err != nil {
				log.Printf("[ConfigWindow] 配置失败: %v", err)
				outputText.SetText(outputText.Text + fmt.Sprintf("\n❌ 配置失败: %v\n", err))
				debouncer.scrollToBottom()
				cw.showMessageWindow("错误", "配置失败")
				return
			}

			log.Printf("[ConfigWindow] 配置完成，结果: %v", result.Success)

			if result.Success {
				cw.showMessageWindow("成功", "配置成功")
			} else {
				cw.showMessageWindow("错误", "配置失败")
			}
		})
	}()
}

// showConfirmConfigDialog 显示白名单为空时的确认对话框
// 参数:
//   port - 要配置的端口号
//   ipEntry - 自定义白名单输入框
//   outputText - 输出面板文本框
//   startBtn - 开始配置按钮
//   cancelBtn - 取消配置按钮
// 功能:
//   - 当白名单为空时，弹出确认对话框确认是否配置
//   - 用户点击"确定"后执行配置（仅拒绝规则，只允许127.0.0.1访问）
//   - 用户点击"取消"后恢复按钮状态
func (cw *ConfigWindow) showConfirmConfigDialog(port int, ipEntry *widget.Entry, outputText *widget.Entry, startBtn, cancelBtn *widget.Button) {
	log.Printf("[ConfigWindow] 显示配置确认对话框，端口: %d", port)

	fyne.Do(func() {
		confirmWin := cw.app.NewWindow("确认配置")

		msgLabel := widget.NewLabel(fmt.Sprintf("只允许127.0.0.1访问\n\n确定要配置端口 %d 的访问控制策略吗？", port))
		msgLabel.Wrapping = fyne.TextWrapWord
		msgLabel.Alignment = fyne.TextAlignCenter

		confirmBtn := widget.NewButton("确定", func() {
			log.Printf("[ConfigWindow] 用户确认配置")
			confirmWin.Close()
			cw.doConfigure(port, "", outputText, startBtn, cancelBtn)
		})
		confirmBtn.Resize(fyne.NewSize(100, InputHeight))

		cancelBtnInternal := widget.NewButton("取消", func() {
			log.Printf("[ConfigWindow] 用户取消配置")
			cw.configuring = false
			startBtn.Enable()
			cancelBtn.Enable()
			confirmWin.Close()
		})
		cancelBtnInternal.Resize(fyne.NewSize(100, InputHeight))

		btnBox := container.NewHBox(
			confirmBtn,
			NewHSpace(20),
			cancelBtnInternal,
		)

		content := container.NewVBox(
			NewVSpace(24),
			container.NewCenter(msgLabel),
			NewVSpace(24),
			container.NewCenter(btnBox),
			NewVSpace(24),
		)

		bg := canvas.NewRectangle(ColorBackground)
		confirmWin.SetContent(container.NewStack(bg, content))

		confirmWin.SetOnClosed(func() {
			cw.configuring = false
			startBtn.Enable()
			cancelBtn.Enable()
		})

		confirmWin.SetFixedSize(true)
		confirmWin.Resize(fyne.NewSize(480, 200))
		confirmWin.CenterOnScreen()
		confirmWin.Show()
	})
}

// deconfigure 执行取消访问控制策略配置
// 参数:
//   portSelect - 端口下拉框（用于显示预设端口，不直接取值）
//   portEntry - 端口输入框（* 必填，取消配置时端口号以此输入框的值为准）
//   ipEntry - 自定义白名单输入框
//   outputText - 输出面板文本框
//   startBtn - 开始配置按钮
//   cancelBtn - 取消配置按钮
// 流程:
//   1. 检查配置状态，防止重复点击
//   2. 校验端口输入框（非空、数字、1-65535范围）
//   3. 如果白名单为空，弹出确认对话框（完整取消）
//   4. 如果白名单不为空，直接执行取消配置（仅删除白名单IP）
// 注意:
//   - 白名单为空时表示完整取消所有规则
//   - 白名单不为空时仅删除指定IP的访问规则
func (cw *ConfigWindow) deconfigure(portSelect *widget.Select, portEntry *widget.Entry, ipEntry *widget.Entry, outputText *widget.Entry, startBtn, cancelBtn *widget.Button) {
	log.Printf("[ConfigWindow] 开始执行取消配置")

	// 1. 检查配置状态，防止重复点击
	if cw.configuring {
		log.Printf("[ConfigWindow] 配置正在进行中，拒绝重复点击")
		cw.showMessageWindow("提示", "配置正在进行中，请等待完成")
		return
	}

	// 2. 端口校验（三项校验，任一项不符合则弹出警告弹窗并中止操作）
	// 端口值以端口号输入框(portEntry.Text)为准，而非下拉框
	// 校验顺序：非空 → 数字 → 端口范围(1-65535)
	portStr := strings.TrimSpace(portEntry.Text) // 去除首尾空格
	if portStr == "" {
		// 校验1：输入框不能为空
		log.Printf("[ConfigWindow] 端口输入框为空")
		cw.showMessageWindow("警告", "端口不能为空，请从下拉列表选择或输入端口号")
		return
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		// 校验2：必须是有效数字
		log.Printf("[ConfigWindow] 端口不是有效数字: %s", portStr)
		cw.showMessageWindow("警告", "端口必须是数字，请输入有效的端口号")
		return
	}

	if port < 1 || port > 65535 {
		// 校验3：端口号必须在1-65535范围内
		log.Printf("[ConfigWindow] 端口号超出范围: %d", port)
		cw.showMessageWindow("警告", "端口号必须在 1-65535 范围内")
		return
	}

	// 3. 获取白名单输入
	ipWhitelist := strings.TrimSpace(ipEntry.Text)

	// 4. 白名单为空时，弹出确认对话框（完整取消）
	if ipWhitelist == "" {
		log.Printf("[ConfigWindow] 未填写白名单，弹出确认对话框")
		cw.configuring = true
		fyne.Do(func() {
			startBtn.Disable()
			cancelBtn.Disable()
		})
		cw.showConfirmDialog(port, ipEntry, outputText, startBtn, cancelBtn)
		return
	}

	// 5. 白名单不为空，直接执行取消配置（仅删除指定IP）
	cw.doDeconfigure(port, ipWhitelist, outputText, startBtn, cancelBtn)
}

// showConfirmDialog 显示取消配置确认对话框
// 参数:
//   port - 要取消配置的端口号
//   ipEntry - 自定义白名单输入框
//   outputText - 输出面板文本框
//   startBtn - 开始配置按钮
//   cancelBtn - 取消配置按钮
// 功能:
//   - 当白名单为空时，弹出确认对话框确认是否完整取消所有规则
//   - 用户点击"确定"后执行完整取消配置
//   - 用户点击"取消"后恢复按钮状态
// 注意:
//   - 调用前已设置configuring=true并禁用按钮
//   - 用户取消时需恢复按钮状态和configuring=false
func (cw *ConfigWindow) showConfirmDialog(port int, ipEntry *widget.Entry, outputText *widget.Entry, startBtn, cancelBtn *widget.Button) {
	log.Printf("[ConfigWindow] 显示取消配置确认对话框，端口: %d", port)

	fyne.Do(func() {
		confirmWin := cw.app.NewWindow("确认取消配置")

		msgLabel := widget.NewLabel(fmt.Sprintf("确定要取消端口 %d 的访问控制策略吗\n\n这将删除该端口的所有规则配置。", port))
		msgLabel.Wrapping = fyne.TextWrapWord
		msgLabel.Alignment = fyne.TextAlignCenter

		confirmBtn := widget.NewButton("确定", func() {
			log.Printf("[ConfigWindow] 用户确认取消配置")
			confirmWin.Close()
			cw.doDeconfigure(port, "", outputText, startBtn, cancelBtn)
		})
		confirmBtn.Resize(fyne.NewSize(100, InputHeight))

		cancelBtnInternal := widget.NewButton("取消", func() {
			log.Printf("[ConfigWindow] 用户取消取消配置")
			cw.configuring = false
			startBtn.Enable()
			cancelBtn.Enable()
			confirmWin.Close()
		})
		cancelBtnInternal.Resize(fyne.NewSize(100, InputHeight))

		btnBox := container.NewHBox(
			confirmBtn,
			NewHSpace(20),
			cancelBtnInternal,
		)

		content := container.NewVBox(
			NewVSpace(24),
			container.NewCenter(msgLabel),
			NewVSpace(24),
			container.NewCenter(btnBox),
			NewVSpace(24),
		)

		bg := canvas.NewRectangle(ColorBackground)
		confirmWin.SetContent(container.NewStack(bg, content))

		confirmWin.SetOnClosed(func() {
			cw.configuring = false
			startBtn.Enable()
			cancelBtn.Enable()
		})

		confirmWin.SetFixedSize(true)
		confirmWin.Resize(fyne.NewSize(480, 200))
		confirmWin.CenterOnScreen()
		confirmWin.Show()
	})
}

// doDeconfigure 实际执行取消配置操作
// 参数:
//   port - 要取消配置的端口号
//   ipWhitelist - 要删除的白名单IP（空表示完整取消）
//   outputText - 输出面板文本框
//   startBtn - 开始配置按钮
//   cancelBtn - 取消配置按钮
// 流程:
//   1. 设置配置状态，禁用按钮，清空输出面板
//   2. 在goroutine中调用firewall.Deconfigure()执行取消配置
//   3. 取消配置完成后恢复按钮状态，显示结果弹窗
// 安全:
//   - 使用defer + recover捕获panic，确保按钮状态恢复
//   - UI更新通过fyne.Do()确保在主线程执行
func (cw *ConfigWindow) doDeconfigure(port int, ipWhitelist string, outputText *widget.Entry, startBtn, cancelBtn *widget.Button) {
	log.Printf("[ConfigWindow] 执行取消配置，端口: %d, IP白名单: '%s'", port, ipWhitelist)

	// 1. 设置配置状态，禁用按钮，清空输出面板
	cw.configuring = true
	fyne.Do(func() {
		startBtn.Disable()
		cancelBtn.Disable()
		outputText.SetText("")
	})

	// 2. 创建滚动防抖器（避免频繁滚动导致闪烁）
	debouncer := newScrollDebouncer(outputText)

	// 3. 创建进度回调函数（输出取消配置日志到面板）
	progressCallback := func(message string) {
		fyne.Do(func() {
			outputText.SetText(outputText.Text + message)
			log.Printf("[ConfigWindow] 取消配置输出: %s", message)
			debouncer.scrollToBottom()
		})
	}

	// 4. 在goroutine中执行取消配置（避免阻塞UI）
	go func() {
		log.Printf("[ConfigWindow] 开始调用防火墙取消配置")

		// defer + recover：确保panic时也能恢复按钮状态
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ConfigWindow] 取消配置发生panic: %v", r)
				fyne.Do(func() {
					cw.configuring = false
					startBtn.Enable()
					cancelBtn.Enable()
					outputText.SetText(outputText.Text + fmt.Sprintf("\n❌ 取消配置发生异常: %v\n", r))
					debouncer.scrollToBottom()
					cw.showMessageWindow("错误", "取消配置发生异常")
				})
			}
		}()

		// 调用防火墙取消配置函数
		err := firewall.Deconfigure(cw.server, port, ipWhitelist, progressCallback)

		// 5. 取消配置完成，恢复UI状态
		fyne.Do(func() {
			cw.configuring = false
			startBtn.Enable()
			cancelBtn.Enable()

			// 处理取消配置错误
			if err != nil {
				log.Printf("[ConfigWindow] 取消配置失败: %v", err)
				outputText.SetText(outputText.Text + fmt.Sprintf("\n❌ 取消配置失败: %v\n", err))
				debouncer.scrollToBottom()
				cw.showMessageWindow("错误", "取消配置失败")
				return
			}

			log.Printf("[ConfigWindow] 取消配置完成")
			cw.showMessageWindow("成功", "取消配置成功")
		})
	}()
}

// centerWindow 将窗口居中显示（未使用，保留兼容）
// 参数:
//   win - 要居中的窗口
//   width - 窗口宽度
//   height - 窗口高度
func (cw *ConfigWindow) centerWindow(win fyne.Window, width, height float32) {
	win.CenterOnScreen()
}

// configureOpenSSH configures OpenSSH on Windows server
// Flow (based on OpenSSH.md):
//   1. Check if OpenSSH is installed and running
//   2. If running → get listening port
//   3. If installed but not running → start service, set auto-start, get port
//   4. If not installed → download, extract, install, set port 38022, start service
// All progress is output to the output panel
// Parameters:
//   outputText - output panel for displaying progress
//   opensshBtn - OpenSSH button (disabled during operation)
//   startBtn - start config button (disabled during operation)
//   cancelBtn - cancel config button (disabled during operation)
//   refreshBtn - refresh port button (disabled during operation)
func (cw *ConfigWindow) configureOpenSSH(outputText *widget.Entry, opensshBtn, startBtn, cancelBtn, refreshBtn *widget.Button) {
	log.Printf("[ConfigWindow] 开始配置OpenSSH，服务器: %s", cw.server.Host)

	// Prevent duplicate clicks
	if cw.configuring {
		log.Printf("[ConfigWindow] 配置正在进行中，拒绝重复点击")
		cw.showMessageWindow("提示", "配置正在进行中，请等待完成")
		return
	}

	cw.configuring = true
	fyne.Do(func() {
		opensshBtn.Disable()
		startBtn.Disable()
		cancelBtn.Disable()
		refreshBtn.Disable()
		outputText.SetText("")
	})

	debouncer := newScrollDebouncer(outputText)

	// Progress callback function for outputting messages to the panel
	progressCallback := func(message string) {
		fyne.Do(func() {
			outputText.SetText(outputText.Text + message)
			log.Printf("[ConfigWindow] OpenSSH配置输出: %s", message)
			debouncer.scrollToBottom()
		})
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ConfigWindow] OpenSSH配置发生panic: %v", r)
				fyne.Do(func() {
					cw.configuring = false
					opensshBtn.Enable()
					startBtn.Enable()
					cancelBtn.Enable()
					refreshBtn.Enable()
					outputText.SetText(outputText.Text + fmt.Sprintf("\n❌ OpenSSH配置发生异常: %v\n", r))
					debouncer.scrollToBottom()
					cw.showMessageWindow("错误", "OpenSSH配置发生异常")
				})
			}
		}()

		// ==================== OpenSSH配置流程开始 ====================
		// 整体流程：连接服务器 → 检查sshd服务状态 → 根据状态执行不同操作
		// 三种状态：INSTALLED_RUNNING(已安装已运行) / INSTALLED_NOT_RUNNING(已安装未运行) / NOT_INSTALLED(未安装)

		// 第一步：创建SMB客户端，连接目标Windows服务器
		// 通过WMI协议在远程服务器上执行PowerShell命令
		progressCallback("🔗 正在连接服务器...\n")
		client, err := smb.NewClient(cw.server)
		if err != nil {
			// SMB连接失败，可能是网络不通、凭据错误、防火墙拦截
			log.Printf("[ConfigWindow] 创建SMB客户端失败: %v", err)
			progressCallback(fmt.Sprintf("❌ 连接服务器失败: %v\n", err))
			fyne.Do(func() {
				cw.configuring = false
				opensshBtn.Enable()
				startBtn.Enable()
				cancelBtn.Enable()
				refreshBtn.Enable()
				cw.showMessageWindow("错误", "连接服务器失败")
			})
			return
		}
		defer client.Close()
		progressCallback("✅ 服务器连接成功\n")

		// 第二步：检查sshd服务是否已安装及其运行状态
		// 使用Get-Service检测sshd服务（比Get-WindowsCapability更可靠，能检测手动安装的OpenSSH）
		// 返回值：INSTALLED_RUNNING / INSTALLED_NOT_RUNNING / NOT_INSTALLED
		progressCallback("📋 检查OpenSSH安装状态...\n")
		checkCmd := `powershell -Command "$svc = Get-Service -Name sshd -ErrorAction SilentlyContinue; if ($svc) { if ($svc.Status -eq 'Running') { 'INSTALLED_RUNNING' } else { 'INSTALLED_NOT_RUNNING' } } else { 'NOT_INSTALLED' }"`
		checkOutput, err := client.Execute(checkCmd)
		if err != nil {
			// 命令执行失败，可能是杀毒软件拦截WMI调用、权限不足、RPC服务异常
			progressCallback("❌ 检查OpenSSH状态失败\n")
			progressCallback("⚠️ 请检查杀毒软件是否拦截配置\n")
			fyne.Do(func() {
				cw.configuring = false
				opensshBtn.Enable()
				startBtn.Enable()
				cancelBtn.Enable()
				refreshBtn.Enable()
				cw.showMessageWindow("错误", "检查OpenSSH状态失败")
			})
			return
		}

		checkResult := strings.TrimSpace(checkOutput)
		log.Printf("[ConfigWindow] OpenSSH检查结果: %s", checkResult)

		// 根据sshd服务状态执行不同操作
		switch checkResult {

		// ==================== 情况1：sshd已安装且正在运行 ====================
		case "INSTALLED_RUNNING":
			// 服务已运行，只需获取监听端口信息，无需任何安装操作
			progressCallback("✅ OpenSSH已安装且正在运行\n")
			progressCallback("📋 获取监听端口...\n")
			// 查询sshd进程监听的TCP端口
			// 流程：Get-Process获取sshd进程ID → Get-NetTCPConnection查询该进程的监听端口
			portCmd := `powershell -Command "$port = (Get-NetTCPConnection -OwningProcess (Get-Process -Name sshd).Id -ErrorAction SilentlyContinue | Where-Object { $_.State -eq 'Listen' } | Select-Object -First 1 -ExpandProperty LocalPort); if ($port) { 'PORT:' + $port } else { 'PORT_EMPTY' }"`
			portOutput, _ := client.Execute(portCmd)
			portResult := strings.TrimSpace(portOutput)
			if strings.HasPrefix(portResult, "PORT:") {
				progressCallback(fmt.Sprintf("📍 监听端口: %s\n", strings.TrimPrefix(portResult, "PORT:")))
			} else {
				progressCallback("⚠️ 未获取到监听端口\n")
			}
			progressCallback("✅ OpenSSH配置完成\n")
			fyne.Do(func() {
				cw.configuring = false
				opensshBtn.Enable()
				startBtn.Enable()
				cancelBtn.Enable()
				refreshBtn.Enable()
				cw.showMessageWindow("成功", "OpenSSH已安装且正在运行")
			})

		// ==================== 情况2：sshd已安装但未运行 ====================
		case "INSTALLED_NOT_RUNNING":
			// 服务已安装但未启动，需要启动服务并设为自动启动
			progressCallback("⚠️ OpenSSH已安装但未运行\n")
			progressCallback("🚀 正在启动OpenSSH服务...\n")
			// 启动sshd服务并设置为开机自动启动
			// Start-Service启动服务 → Set-Service设置自动启动 → 验证服务状态
			startCmd := `powershell -Command "Start-Service sshd; Set-Service -Name sshd -StartupType 'Automatic'; if ((Get-Service -Name sshd).Status -eq 'Running') { 'STARTED' } else { 'FAILED' }"`
			startOutput, err := client.ExecuteWithTimeout(startCmd, 30*time.Second)
			if err != nil || !strings.Contains(strings.TrimSpace(startOutput), "STARTED") {
				// 启动失败，可能是端口被占用、配置文件错误、权限不足
				progressCallback("❌ 启动服务失败\n")
				fyne.Do(func() {
					cw.configuring = false
					opensshBtn.Enable()
					startBtn.Enable()
					cancelBtn.Enable()
					refreshBtn.Enable()
					cw.showMessageWindow("错误", "启动OpenSSH服务失败")
				})
				return
			}
			progressCallback("✅ 服务已启动\n")
			// 启动成功后获取监听端口
			progressCallback("📋 获取监听端口...\n")
			portCmd := `powershell -Command "$port = (Get-NetTCPConnection -OwningProcess (Get-Process -Name sshd).Id -ErrorAction SilentlyContinue | Where-Object { $_.State -eq 'Listen' } | Select-Object -First 1 -ExpandProperty LocalPort); if ($port) { 'PORT:' + $port } else { 'PORT_EMPTY' }"`
			portOutput, _ := client.Execute(portCmd)
			portResult := strings.TrimSpace(portOutput)
			if strings.HasPrefix(portResult, "PORT:") {
				progressCallback(fmt.Sprintf("📍 监听端口: %s\n", strings.TrimPrefix(portResult, "PORT:")))
			} else {
				progressCallback("⚠️ 未获取到监听端口\n")
			}
			progressCallback("✅ OpenSSH配置完成\n")
			fyne.Do(func() {
				cw.configuring = false
				opensshBtn.Enable()
				startBtn.Enable()
				cancelBtn.Enable()
				refreshBtn.Enable()
				cw.showMessageWindow("成功", "OpenSSH服务已启动")
			})

		// ==================== 情况3：sshd未安装，需要完整安装流程 ====================
		case "NOT_INSTALLED":
			// 完整安装流程：下载 → 解压 → 安装服务 → 修改端口 → 启动服务 → 验证
			progressCallback("⬇️ 开始安装OpenSSH...\n")

			// 步骤3.1：下载OpenSSH.zip安装包
			// 下载源优先级：先app.zhuyan.cloud（国内快），失败后切换GitHub
			// 下载到 C:\Program Files\OpenSSH.zip，超时300秒
			progressCallback("⬇️ 正在下载...\n")
			downloadCmd := `powershell -Command "$urls = @('https://app.zhuyan.cloud/v5/dl/cmd/OpenSSH.zip', 'https://github.com/PowerShell/Win32-OpenSSH/releases/latest/download/OpenSSH-Win64.zip'); foreach ($url in $urls) { try { Invoke-WebRequest -Uri $url -OutFile 'C:\Program Files\OpenSSH.zip' -TimeoutSec 300 -MaximumRedirection 10 -UseBasicParsing -ErrorAction Stop; 'DOWNLOAD_OK'; exit 0 } catch {} }; 'DOWNLOAD_FAILED'"`
			downloadOutput, err := client.ExecuteWithTimeout(downloadCmd, 5*time.Minute)
			if err != nil || !strings.Contains(strings.TrimSpace(downloadOutput), "DOWNLOAD_OK") {
				// 下载失败，可能是服务器无上网权限、防火墙拦截、DNS解析失败
				progressCallback("❌ 下载失败\n")
				progressCallback("⚠️ 请检查服务器上网权限\n")
				fyne.Do(func() {
					cw.configuring = false
					opensshBtn.Enable()
					startBtn.Enable()
					cancelBtn.Enable()
					refreshBtn.Enable()
					cw.showMessageWindow("错误", "下载OpenSSH失败")
				})
				return
			}
			progressCallback("✅ 下载成功\n")

			// 步骤3.2：解压OpenSSH.zip到C:\Program Files
			// zip包内部包含OpenSSH/文件夹，解压后自动创建C:\Program Files\OpenSSH\
			// 解压完成后删除zip包释放空间
			progressCallback("📦 正在解压...\n")
			extractCmd := `powershell -Command "Expand-Archive -Path 'C:\Program Files\OpenSSH.zip' -DestinationPath 'C:\Program Files' -Force; Remove-Item -Path 'C:\Program Files\OpenSSH.zip' -Force; 'EXTRACT_OK'"`
			extractOutput, err := client.Execute(extractCmd)
			if err != nil || !strings.Contains(strings.TrimSpace(extractOutput), "EXTRACT_OK") {
				// 解压失败，可能是磁盘空间不足、文件损坏、权限不足
				progressCallback("❌ 解压失败\n")
				fyne.Do(func() {
					cw.configuring = false
					opensshBtn.Enable()
					startBtn.Enable()
					cancelBtn.Enable()
					refreshBtn.Enable()
					cw.showMessageWindow("错误", "解压OpenSSH失败")
				})
				return
			}
			progressCallback("✅ 解压完成\n")

			// 步骤3.3：执行install-sshd.ps1安装sshd服务
			// 先设置ExecutionPolicy为Bypass允许执行脚本
			// 执行安装脚本后通过Get-Service验证sshd服务是否创建成功
			progressCallback("🔧 正在安装sshd服务...\n")
			installCmd := `powershell -Command "Set-ExecutionPolicy -ExecutionPolicy Bypass -Scope Process -Force; cd 'C:\Program Files\OpenSSH'; .\install-sshd.ps1; if (Get-Service -Name sshd -ErrorAction SilentlyContinue) { 'INSTALL_OK' } else { 'INSTALL_FAILED' }"`
			installOutput, err := client.ExecuteWithTimeout(installCmd, 3*time.Minute)
			if err != nil || !strings.Contains(strings.TrimSpace(installOutput), "INSTALL_OK") {
				// 安装失败，可能是杀毒软件拦截、权限不足、脚本执行错误
				progressCallback("❌ 安装sshd服务失败\n")
				progressCallback("⚠️ 请检查杀毒软件是否拦截配置\n")
				fyne.Do(func() {
					cw.configuring = false
					opensshBtn.Enable()
					startBtn.Enable()
					cancelBtn.Enable()
					refreshBtn.Enable()
					cw.showMessageWindow("错误", "安装sshd服务失败")
				})
				return
			}
			progressCallback("✅ sshd服务安装完成\n")

			// 步骤3.4：检查端口可用性并修改sshd监听端口
			// 先检查38022端口是否被占用，如果被占用则尝试使用默认22端口
			// 如果22端口也被占用，则提示用户检查端口可用性
			progressCallback("🔍 检查端口可用性...\n")
			// 检查38022和22端口的监听状态
			// 返回值：USE_38022(38022可用) / USE_22(38022被占用,22可用) / BOTH_OCCUPIED(都被占用)
			portCheckCmd := `powershell -Command "$p38022 = Get-NetTCPConnection -LocalPort 38022 -State Listen -ErrorAction SilentlyContinue; $p22 = Get-NetTCPConnection -LocalPort 22 -State Listen -ErrorAction SilentlyContinue; if ($p38022) { if ($p22) { 'BOTH_OCCUPIED' } else { 'USE_22' } } else { 'USE_38022' }"`
			portCheckOutput, err := client.Execute(portCheckCmd)
			if err != nil {
				progressCallback("❌ 检查端口可用性失败\n")
				fyne.Do(func() {
					cw.configuring = false
					opensshBtn.Enable()
					startBtn.Enable()
					cancelBtn.Enable()
					refreshBtn.Enable()
					cw.showMessageWindow("错误", "检查端口可用性失败")
				})
				return
			}
			portCheckResult := strings.TrimSpace(portCheckOutput)

			if portCheckResult == "BOTH_OCCUPIED" {
				// 38022和22端口都被占用，无法配置OpenSSH
				progressCallback("❌ 38022和22端口均被占用\n")
				progressCallback("⚠️ 请检查OpenSSH端口可用性\n")
				fyne.Do(func() {
					cw.configuring = false
					opensshBtn.Enable()
					startBtn.Enable()
					cancelBtn.Enable()
					refreshBtn.Enable()
					cw.showMessageWindow("错误", "38022和22端口均被占用")
				})
				return
			}

			// 确定最终使用的SSH监听端口
			// USE_38022：使用38022端口；USE_22：38022被占用则使用默认22端口
			chosenPort := "38022"
			if portCheckResult == "USE_22" {
				chosenPort = "22"
				progressCallback("⚠️ 38022端口被占用，使用默认22端口\n")
			} else {
				progressCallback("🔧 正在修改端口为38022...\n")
			}

			// 修改sshd配置文件（C:\ProgramData\ssh\sshd_config，由install-sshd.ps1创建）
			// 先停止sshd服务（避免文件占用），再清理并重写配置项：
			//   1. 过滤掉所有已有（含被注释）的 'Port' 行：正则 '^\s*#?\s*Port\s+' 匹配 '#Port 22' 或 'Port 22'
			//   2. 过滤掉所有已有（含被注释）的 'Subsystem sftp' 行，避免重复
			//   3. 在文件末尾显式追加 'Port <chosenPort>' 和 'Subsystem sftp sftp-server.exe'
			// 注意：为何不能使用 -replace「仅替换已有行」的方式设置端口？
			//       若配置文件原本不存在 Port 行（或 Port 行被注释且正则不匹配），-replace 不会新增任何内容，
			//       sshd 将回退到默认监听 22 端口。因此必须采用「过滤旧行 + 显式追加新行」的写法，
			//       确保无论配置文件原本内容如何，Port 和 Subsystem sftp 都一定会写入并生效。
			// 注意：Windows OpenSSH 安装后默认将 Subsystem sftp 注释掉，若未启用则无法使用 SFTP 登录，
			//       因此这里必须显式添加并取消注释，保证远程SFTP登录可用
			portConfigCmd := fmt.Sprintf(`powershell -Command "Stop-Service sshd -ErrorAction SilentlyContinue; $configPath = 'C:\ProgramData\ssh\sshd_config'; if (-not (Test-Path $configPath)) { New-Item -ItemType File -Path $configPath -Force }; $lines = @(Get-Content $configPath | Where-Object { $_ -notmatch '^\s*#?\s*Port\s' -and $_ -notmatch '^\s*#?\s*Subsystem\s+sftp\s' }); Set-Content $configPath (@($lines) + 'Port %s' + 'Subsystem sftp sftp-server.exe'); 'PORT_OK'"`, chosenPort)
			portConfigOutput, err := client.ExecuteWithTimeout(portConfigCmd, 30*time.Second)
			if err != nil || !strings.Contains(strings.TrimSpace(portConfigOutput), "PORT_OK") {
				// 修改配置文件失败，可能是配置文件被占用、权限不足
				progressCallback("❌ 修改端口/Subsystem配置失败\n")
				fyne.Do(func() {
					cw.configuring = false
					opensshBtn.Enable()
					startBtn.Enable()
					cancelBtn.Enable()
					refreshBtn.Enable()
					cw.showMessageWindow("错误", "修改OpenSSH配置失败")
				})
				return
			}
			progressCallback("✅ 端口和SFTP子系统配置完成\n")

			// 步骤3.5：启动sshd服务并设置为开机自动启动
			// 端口修改完成后启动服务，验证服务状态是否为Running
			progressCallback("🚀 正在启动OpenSSH服务...\n")
			startSvcCmd := `powershell -Command "Start-Service sshd; Set-Service -Name sshd -StartupType 'Automatic'; if ((Get-Service -Name sshd).Status -eq 'Running') { 'STARTED' } else { 'FAILED' }"`
			startSvcOutput, err := client.ExecuteWithTimeout(startSvcCmd, 30*time.Second)
			if err != nil || !strings.Contains(strings.TrimSpace(startSvcOutput), "STARTED") {
				// 启动失败，可能是端口被占用、配置文件语法错误、依赖服务未启动
				progressCallback("❌ 启动服务失败\n")
				fyne.Do(func() {
					cw.configuring = false
					opensshBtn.Enable()
					startBtn.Enable()
					cancelBtn.Enable()
					refreshBtn.Enable()
					cw.showMessageWindow("错误", "启动OpenSSH服务失败")
				})
				return
			}
			progressCallback("✅ 服务已启动\n")

			// 步骤3.6：验证安装结果，获取实际监听端口
			// 查询sshd进程的TCP监听端口，确认服务正常运行在38022端口
			progressCallback("📋 验证安装状态...\n")
			verifyCmd := `powershell -Command "$port = (Get-NetTCPConnection -OwningProcess (Get-Process -Name sshd).Id -ErrorAction SilentlyContinue | Where-Object { $_.State -eq 'Listen' } | Select-Object -First 1 -ExpandProperty LocalPort); if ($port) { 'PORT:' + $port } else { 'PORT_EMPTY' }"`
			verifyOutput, _ := client.Execute(verifyCmd)
			verifyResult := strings.TrimSpace(verifyOutput)
			if strings.HasPrefix(verifyResult, "PORT:") {
				progressCallback(fmt.Sprintf("📍 监听端口: %s\n", strings.TrimPrefix(verifyResult, "PORT:")))
			} else {
				progressCallback("⚠️ 未获取到监听端口\n")
			}

			progressCallback("✅ OpenSSH配置完成\n")
			fyne.Do(func() {
				cw.configuring = false
				opensshBtn.Enable()
				startBtn.Enable()
				cancelBtn.Enable()
				refreshBtn.Enable()
				cw.showMessageWindow("成功", "OpenSSH安装配置完成")
			})

		// ==================== 默认情况：无法确定sshd状态 ====================
		default:
			// 命令返回了未知结果，可能是PowerShell执行异常、输出被截断
			progressCallback("❌ 无法确定OpenSSH状态\n")
			fyne.Do(func() {
				cw.configuring = false
				opensshBtn.Enable()
				startBtn.Enable()
				cancelBtn.Enable()
				refreshBtn.Enable()
				cw.showMessageWindow("错误", "无法确定OpenSSH状态")
			})
		}
		// ==================== OpenSSH配置流程结束 ====================
	}()
}

// scrollDebouncer 输出面板滚动防抖器
// 功能:
//   - 合并短时间内的多次滚动请求，避免频繁滚动导致界面闪烁
//   - 使用定时器延迟执行滚动操作
type scrollDebouncer struct {
	timer      *time.Timer    // 防抖定时器
	outputText *widget.Entry  // 输出面板文本框
}

// newScrollDebouncer 创建滚动防抖器实例
// 参数:
//   outputText - 输出面板文本框
// 返回:
//   scrollDebouncer指针
func newScrollDebouncer(outputText *widget.Entry) *scrollDebouncer {
	return &scrollDebouncer{outputText: outputText}
}

// scrollToBottom 将输出面板滚动到底部
// 防抖机制:
//   - 取消之前的定时器（如果存在）
//   - 重新设置50ms延迟定时器
//   - 延迟后通过设置CursorRow实现滚动到底部
// 注意:
//   - 必须在fyne.Do()中执行UI操作
func (d *scrollDebouncer) scrollToBottom() {
	// 取消之前的定时器，实现防抖
	if d.timer != nil {
		d.timer.Stop()
	}
	// 设置50ms延迟，合并短时间内的多次滚动请求
	d.timer = time.AfterFunc(50*time.Millisecond, func() {
		fyne.Do(func() {
			// 通过设置光标行到最后一行实现滚动到底部
			d.outputText.CursorRow = len(strings.Split(d.outputText.Text, "\n")) - 1
			d.outputText.Refresh()
		})
	})
}
