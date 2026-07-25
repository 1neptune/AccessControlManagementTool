package ui

import (
	"access-control-tool/internal/firewall"
	"access-control-tool/internal/models"
	"access-control-tool/internal/portscanner"
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

	// 统一输入框宽度配置（用户指定）
	// labelWidth: 标签固定宽度（80px），所有标签统一此宽度
	// labelHeight: 标签高度，与输入框高度一致
	// portSelectWidth: 端口选择下拉框宽度（120px）
	// serviceInputWidth: 服务名称输入框宽度（300px），用户指定
	// contentInputWidth: 关联业务/白名单输入框宽度（750px），用户指定
	// formTotalWidth: 表单区域总宽度（838px），由关联业务行决定
	//   计算公式: labelWidth(80) + 间距(8) + contentInputWidth(750) = 838px
	labelWidth := float32(80)
	labelHeight := float32(InputHeight)
	portSelectWidth := float32(120)
	serviceInputWidth := float32(300)
	contentInputWidth := float32(750)
	formTotalWidth := labelWidth + 8 + contentInputWidth

	log.Printf("[ConfigWindow] 布局参数 - labelWidth=%.2f, labelHeight=%.2f, portSelectWidth=%.2f, serviceInputWidth=%.2f, contentInputWidth=%.2f, formTotalWidth=%.2f",
		labelWidth, labelHeight, portSelectWidth, serviceInputWidth, contentInputWidth, formTotalWidth)

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

	// 端口选择下拉框（选择后自动填充服务名称和业务IP）
	portSelect := widget.NewSelect([]string{}, func(s string) {
		cw.handlePortSelect(s, serviceNameEntry, businessIPEntry, ipEntry)
	})
	portSelectBorder := canvas.NewRectangle(ColorBorder)
	portSelectBorder.CornerRadius = CornerRadius
	portSelectBorder.SetMinSize(fyne.NewSize(portSelectWidth, InputHeight))
	portSelectField := container.NewStack(portSelectBorder, portSelect)
	log.Printf("[ConfigWindow] 端口选择下拉框创建完成，宽度=%.2f, 高度=%.2f", portSelectWidth, InputHeight)

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
		cw.configure(portSelect, ipEntry, outputText, startConfigBtn, cancelConfigBtn)
	}

	cancelConfigBtn.OnTapped = func() {
		log.Printf("[ConfigWindow] 用户点击取消配置按钮")
		cw.deconfigure(portSelect, ipEntry, outputText, startConfigBtn, cancelConfigBtn)
	}

	// ===== 组装表单行 =====

	// 刷新按钮（点击后重新扫描端口信息，样式与其他按钮一致）
	var refreshBtn *widget.Button
	refreshBtn = widget.NewButton("刷新端口", func() {
		log.Printf("[ConfigWindow] 用户点击刷新按钮")
		fyne.Do(func() {
			portSelect.Disable()
			refreshBtn.Disable()
			portSelect.SetSelected("正在获取...")
			serviceNameEntry.SetText("正在获取...")
			businessIPEntry.SetText("正在获取...")
			ipEntry.SetText("")
		})
		go cw.loadPorts(portSelect, serviceNameEntry, businessIPEntry, ipEntry, refreshBtn)
	})

	// 重新设计布局：使用简单的HBox布局，每行独立
	// 端口行: [端口标签(80)] + [8] + [端口选择(120)] + [16] + [服务名称标签(80)] + [8] + [服务名称输入框(700)]
	// 关联业务行: [关联业务标签(80)] + [8] + [关联业务输入框(924)]
	// 白名单行: [自定义白名单标签(80)] + [8] + [白名单输入框(924)]
	// 
	// contentInputWidth = portSelectWidth(120) + 间距(16) + labelWidth(80) + 间距(8) + serviceInputWidth(700) = 924px
	// formTotalWidth = labelWidth(80) + 间距(8) + contentInputWidth(924) = 1012px
	
	log.Printf("[ConfigWindow] 开始组装布局...")
	
	// 创建端口行
	portRow := container.NewHBox(
		newLabelWithAlign("端口", labelWidth, labelHeight, true),
		NewHSpace(8),
		portSelectField,
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

	// 按钮行（居中对齐，间距均匀）：开始配置 + 取消配置 + 刷新端口
	// 使用formTotalWidth确保按钮区域宽度与表单行一致
	// 三个按钮总宽度: 100(开始配置) + 20(间距) + 100(取消配置) + 20(间距) + 100(刷新端口) = 340px
	// 两侧空白间距: (formTotalWidth - 340) / 2，使按钮居中显示
	configBtnBox := container.NewHBox(
		NewHSpace((formTotalWidth-100-100-100)/2),
		startConfigBtn,
		NewHSpace(20),
		cancelConfigBtn,
		NewHSpace(20),
		refreshBtn,
		NewHSpace((formTotalWidth-100-100-100)/2),
	)

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
		refreshBtn.Disable()
		portSelect.SetSelected("正在获取...")
		serviceNameEntry.SetText("正在获取...")
		businessIPEntry.SetText("正在获取...")
		ipEntry.SetText("")
	})

	// 异步加载端口信息
	go cw.loadPorts(portSelect, serviceNameEntry, businessIPEntry, ipEntry, refreshBtn)
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
//   - 更新端口选择下拉框选项（加载完成后启用下拉框和刷新按钮）
//   - 默认选中第一个端口并自动填充相关信息
//   - 加载失败时显示错误提示并启用刷新按钮
// 注意:
//   - 在goroutine中执行，UI更新需通过fyne.Do()
//   - 调用前应先禁用refreshBtn，加载完成后自动启用
func (cw *ConfigWindow) loadPorts(portSelect *widget.Select, serviceNameEntry, businessIPEntry, ipEntry *widget.Entry, refreshBtn *widget.Button) {
	log.Printf("[ConfigWindow] 开始扫描服务器端口: %s:%d", cw.server.Host, cw.server.SSHPort)

	// 调用端口扫描器获取端口列表
	portInfoList, err := portscanner.ScanPorts(cw.server)
	if err != nil {
		log.Printf("[ConfigWindow] 端口扫描失败: %v", err)
		fyne.Do(func() {
			portSelect.Enable()
			refreshBtn.Enable()
			portSelect.SetOptions([]string{})
			portSelect.SetSelected("获取失败")
			serviceNameEntry.SetText("获取失败")
			businessIPEntry.SetText("获取失败")
			cw.showMessageWindow("错误", "端口扫描失败")
		})
		return
	}

	log.Printf("[ConfigWindow] 端口扫描成功，发现 %d 个端口", len(portInfoList))

	// 将端口信息存入映射表
	cw.portInfoMap = make(map[int]portscanner.PortInfo)
	portStrings := make([]string, 0, len(portInfoList))
	for _, info := range portInfoList {
		portStrings = append(portStrings, strconv.Itoa(info.Port))
		cw.portInfoMap[info.Port] = info
	}

	// 更新UI：启用端口下拉框和刷新按钮，设置端口选项并默认选中第一个
	fyne.Do(func() {
		portSelect.Enable()
		refreshBtn.Enable()
		portSelect.SetOptions(portStrings)
		if len(portStrings) > 0 {
			portSelect.SetSelected(portStrings[0])
			log.Printf("[ConfigWindow] 默认选中端口: %s", portStrings[0])
			cw.handlePortSelect(portStrings[0], serviceNameEntry, businessIPEntry, ipEntry)
		} else {
			portSelect.SetSelected("无端口")
			serviceNameEntry.SetText("无")
			businessIPEntry.SetText("无")
		}
	})
}

// configure 执行访问控制策略配置
// 参数:
//   portSelect - 端口选择下拉框
//   ipEntry - 自定义白名单输入框
//   outputText - 输出面板文本框
//   startBtn - 开始配置按钮
//   cancelBtn - 取消配置按钮
// 流程:
//   1. 检查配置状态，防止重复点击
//   2. 校验端口和白名单输入
//   3. 设置配置状态，禁用按钮，清空输出面板
//   4. 在goroutine中调用firewall.Configure()执行配置
//   5. 配置完成后恢复按钮状态，显示结果弹窗
// 安全:
//   - 使用defer + recover捕获panic，确保按钮状态恢复
//   - UI更新通过fyne.Do()确保在主线程执行
func (cw *ConfigWindow) configure(portSelect *widget.Select, ipEntry *widget.Entry, outputText *widget.Entry, startBtn, cancelBtn *widget.Button) {
	log.Printf("[ConfigWindow] 开始执行配置")

	// 1. 检查配置状态，防止重复点击
	if cw.configuring {
		log.Printf("[ConfigWindow] 配置正在进行中，拒绝重复点击")
		cw.showMessageWindow("提示", "配置正在进行中，请等待完成")
		return
	}

	// 2. 校验端口选择
	if portSelect.Selected == "" {
		log.Printf("[ConfigWindow] 未选择端口")
		cw.showMessageWindow("错误", "请选择端口")
		return
	}

	port, err := strconv.Atoi(portSelect.Selected)
	if err != nil {
		log.Printf("[ConfigWindow] 无效的端口号: %s", portSelect.Selected)
		cw.showMessageWindow("错误", "无效的端口号")
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
//   portSelect - 端口选择下拉框
//   ipEntry - 自定义白名单输入框
//   outputText - 输出面板文本框
//   startBtn - 开始配置按钮
//   cancelBtn - 取消配置按钮
// 流程:
//   1. 检查配置状态，防止重复点击
//   2. 校验端口选择
//   3. 如果白名单为空，弹出确认对话框（完整取消）
//   4. 如果白名单不为空，直接执行取消配置（仅删除白名单IP）
// 注意:
//   - 白名单为空时表示完整取消所有规则
//   - 白名单不为空时仅删除指定IP的访问规则
func (cw *ConfigWindow) deconfigure(portSelect *widget.Select, ipEntry *widget.Entry, outputText *widget.Entry, startBtn, cancelBtn *widget.Button) {
	log.Printf("[ConfigWindow] 开始执行取消配置")

	// 1. 检查配置状态，防止重复点击
	if cw.configuring {
		log.Printf("[ConfigWindow] 配置正在进行中，拒绝重复点击")
		cw.showMessageWindow("提示", "配置正在进行中，请等待完成")
		return
	}

	// 2. 校验端口选择
	if portSelect.Selected == "" {
		log.Printf("[ConfigWindow] 未选择端口")
		cw.showMessageWindow("错误", "请选择端口")
		return
	}

	port, err := strconv.Atoi(portSelect.Selected)
	if err != nil {
		log.Printf("[ConfigWindow] 无效的端口号: %s", portSelect.Selected)
		cw.showMessageWindow("错误", "无效的端口号")
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
