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
	msgWin := cw.app.NewWindow(title)
	msgWin.SetFixedSize(true)

	// 创建消息标签，设置为不换行、居中对齐
	msgLabel := widget.NewLabel(message)
	msgLabel.Wrapping = fyne.TextWrapOff
	msgLabel.Alignment = fyne.TextAlignCenter

	// 创建确定按钮
	okBtn := widget.NewButton("确定", func() {
		msgWin.Close()
	})
	okBtn.Resize(fyne.NewSize(120, InputHeight))

	// 为按钮添加边框
	okBorder := canvas.NewRectangle(ColorBorder)
	okBorder.CornerRadius = CornerRadius
	okBorder.SetMinSize(fyne.NewSize(120, InputHeight))
	okField := container.NewStack(okBorder, container.NewPadded(okBtn))

	// 组装弹窗内容布局
	content := container.NewVBox(
		NewVSpace(SpacingL),
		container.NewCenter(msgLabel),
		NewVSpace(SpacingL),
		container.NewCenter(okField),
		NewVSpace(SpacingL),
	)

	// 设置弹窗背景和内容
	bg := canvas.NewRectangle(ColorBackground)
	msgWin.SetContent(container.NewStack(bg, content))

	// 设置弹窗尺寸并居中显示
	msgWin.Resize(fyne.NewSize(480, 160))
	msgWin.CenterOnScreen()
	msgWin.Show()
}

// newLabelWithAlign 创建带对齐的标签容器
// 参数:
//   text - 标签文本内容
//   width - 标签宽度
//   height - 标签高度
// 返回:
//   包含文本的容器，文本垂直居中
// 说明:
//   使用GridWrap固定尺寸，通过计算paddingTop实现文本垂直居中
func newLabelWithAlign(text string, width, height float32) *fyne.Container {
	textCanvas := canvas.NewText(text, ColorText)
	textCanvas.TextSize = 14
	// 计算顶部padding使文本垂直居中（TextSize=14，行高约16）
	paddingTop := (height - 16) / 2
	return container.NewGridWrap(fyne.NewSize(width, height), container.NewVBox(
		NewVSpace(paddingTop),
		container.NewHBox(textCanvas),
	))
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

	// 服务名称输入框（只读，由端口选择自动填充）
	serviceNameEntry := widget.NewEntry()
	serviceNameEntry.Disable()
	serviceNameBorder := canvas.NewRectangle(ColorBorder)
	serviceNameBorder.CornerRadius = CornerRadius
	serviceNameBorder.SetMinSize(fyne.NewSize(340, InputHeight))
	serviceNameField := container.NewStack(serviceNameBorder, container.NewPadded(serviceNameEntry))

	// 关联业务IP输入框（只读，多行，带滚动）
	businessIPEntry := widget.NewMultiLineEntry()
	businessIPEntry.Disable()
	businessIPEntry.Wrapping = fyne.TextWrapWord
	businessIPScroll := container.NewScroll(businessIPEntry)
	businessIPScroll.SetMinSize(fyne.NewSize(700, 80))
	businessIPBorder := canvas.NewRectangle(ColorBorder)
	businessIPBorder.CornerRadius = CornerRadius
	businessIPBorder.SetMinSize(fyne.NewSize(700, 80))
	businessIPField := container.NewStack(businessIPBorder, container.NewPadded(businessIPScroll))

	// 自定义白名单输入框（必填，支持逗号分隔多个IP）
	ipEntry := widget.NewEntry()
	ipEntry.SetPlaceHolder("多个IP用逗号分隔，支持IP段如 192.168.1.0/24")
	ipBorder := canvas.NewRectangle(ColorBorder)
	ipBorder.CornerRadius = CornerRadius
	ipBorder.SetMinSize(fyne.NewSize(700, InputHeight))
	ipField := container.NewStack(ipBorder, container.NewPadded(ipEntry))

	// 端口选择下拉框（选择后自动填充服务名称和业务IP）
	portSelect := widget.NewSelect([]string{}, func(s string) {
		cw.handlePortSelect(s, serviceNameEntry, businessIPEntry, ipEntry)
	})
	portSelectBorder := canvas.NewRectangle(ColorBorder)
	portSelectBorder.CornerRadius = CornerRadius
	portSelectBorder.SetMinSize(fyne.NewSize(150, InputHeight))
	portSelectField := container.NewStack(portSelectBorder, container.NewPadded(portSelect))

	// 输出面板（多行文本框，只读，固定尺寸780x200）
	outputText := widget.NewMultiLineEntry()
	outputText.Disable()
	outputBorder := canvas.NewRectangle(ColorBorder)
	outputBorder.CornerRadius = CornerRadius
	outputField := container.NewStack(outputBorder, container.NewPadded(container.NewGridWrap(fyne.NewSize(780, 200), outputText)))

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
	labelWidth := float32(80)
	labelHeight := float32(InputHeight)

	// 端口行：端口选择 + 服务名称
	portRow := container.NewHBox(
		newLabelWithAlign("端口", labelWidth, labelHeight),
		NewHSpace(8),
		portSelectField,
		NewHSpace(16),
		newLabelWithAlign("服务名称", labelWidth, labelHeight),
		NewHSpace(8),
		serviceNameField,
	)

	// 关联业务行
	businessRow := container.NewHBox(
		newLabelWithAlign("关联业务", labelWidth, 80),
		NewHSpace(8),
		businessIPField,
	)

	// 白名单行（带*表示必填）
	whitelistRow := container.NewHBox(
		newLabelWithAlign("自定义白名单*", labelWidth, labelHeight),
		NewHSpace(8),
		ipField,
	)

	// 按钮行（居中对齐）
	configBtnBox := container.NewGridWrap(fyne.NewSize(840, 48), container.NewHBox(
		NewHSpace(280),
		startConfigBtn,
		NewHSpace(20),
		cancelConfigBtn,
	))

	// ===== 组装卡片 =====

	// 配置卡片
	configCard := NewCard(
		container.NewBorder(
			nil, nil, nil, nil,
			container.NewVBox(
				NewVSpace(12),
				portRow,
				NewVSpace(12),
				businessRow,
				NewVSpace(12),
				whitelistRow,
				NewVSpace(16),
				configBtnBox,
				NewVSpace(12),
			),
		),
	)

	// 输出卡片
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
	configWin.SetFixedSize(true)
	configWin.Resize(fyne.NewSize(880, 680))
	log.Printf("[ConfigWindow] 内容尺寸: %v", configWin.Content().Size())
	configWin.CenterOnScreen()
	configWin.Show()
	log.Printf("[ConfigWindow] 配置窗口居中并显示完成")

	// 异步加载端口信息
	go cw.loadPorts(portSelect, serviceNameEntry, businessIPEntry, ipEntry)
}

// handlePortSelect 处理端口选择事件
// 参数:
//   portStr - 选中的端口字符串
//   serviceNameEntry - 服务名称输入框
//   businessIPEntry - 关联业务IP输入框
//   ipEntry - 自定义白名单输入框
// 功能:
//   - 当用户选择端口时，自动填充服务名称和关联业务IP
//   - 将关联业务IP同时填充到白名单输入框（作为默认值）
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

	// 从端口信息映射表中获取端口详情
	if info, ok := cw.portInfoMap[port]; ok {
		// 填充服务名称
		serviceNameEntry.SetText(info.ServiceName)
		// 填充关联业务IP
		if len(info.ConnectedIPs) > 0 {
			businessIPEntry.SetText(strings.Join(info.ConnectedIPs, "\n"))
			// 将关联IP作为白名单默认值
			ipEntry.SetText(strings.Join(info.ConnectedIPs, ", "))
		} else {
			businessIPEntry.SetText("无")
			ipEntry.SetText("")
		}
		log.Printf("[ConfigWindow] 端口 %d 服务名称: %s, 业务IP: %v", port, info.ServiceName, info.ConnectedIPs)
	}
}

// loadPorts 异步加载服务器端口信息
// 参数:
//   portSelect - 端口选择下拉框
//   serviceNameEntry - 服务名称输入框
//   businessIPEntry - 关联业务IP输入框
//   ipEntry - 自定义白名单输入框
// 功能:
//   - 调用端口扫描器获取服务器开放端口列表
//   - 将端口信息存入portInfoMap供后续使用
//   - 更新端口选择下拉框选项
//   - 默认选中第一个端口并自动填充相关信息
// 注意:
//   - 在goroutine中执行，UI更新需通过fyne.Do()
func (cw *ConfigWindow) loadPorts(portSelect *widget.Select, serviceNameEntry, businessIPEntry, ipEntry *widget.Entry) {
	log.Printf("[ConfigWindow] 开始扫描服务器端口: %s:%d", cw.server.Host, cw.server.SSHPort)

	// 调用端口扫描器获取端口列表
	portInfoList, err := portscanner.ScanPorts(cw.server)
	if err != nil {
		log.Printf("[ConfigWindow] 端口扫描失败: %v", err)
		fyne.Do(func() {
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

	// 更新UI：设置端口选项并默认选中第一个
	fyne.Do(func() {
		portSelect.SetOptions(portStrings)
		if len(portStrings) > 0 {
			portSelect.SetSelected(portStrings[0])
			log.Printf("[ConfigWindow] 默认选中端口: %s", portStrings[0])
			cw.handlePortSelect(portStrings[0], serviceNameEntry, businessIPEntry, ipEntry)
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

	// 3. 校验白名单输入（必填）
	ipWhitelist := strings.TrimSpace(ipEntry.Text)
	if ipWhitelist == "" {
		log.Printf("[ConfigWindow] 自定义白名单为空")
		cw.showMessageWindow("错误", "请输入自定义白名单IP")
		return
	}

	// 4. 设置配置状态，禁用按钮，清空输出面板
	cw.configuring = true
	fyne.Do(func() {
		startBtn.Disable()
		cancelBtn.Disable()
		outputText.SetText("")
	})
	log.Printf("[ConfigWindow] 配置参数 - 端口: %d, IP白名单: '%s'", port, ipWhitelist)

	// 5. 创建滚动防抖器（避免频繁滚动导致闪烁）
	debouncer := newScrollDebouncer(outputText)

	// 6. 创建进度回调函数（输出配置日志到面板）
	progressCallback := func(message string) {
		fyne.Do(func() {
			outputText.SetText(outputText.Text + message)
			log.Printf("[ConfigWindow] 配置输出: %s", message)
			debouncer.scrollToBottom()
		})
	}

	// 7. 在goroutine中执行配置（避免阻塞UI）
	go func() {
		log.Printf("[ConfigWindow] 开始调用防火墙配置")

		// defer + recover：确保panic时也能恢复按钮状态
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

		// 调用防火墙配置函数
		result, err := firewall.Configure(cw.server, port, ipWhitelist, progressCallback)

		// 8. 配置完成，恢复UI状态
		fyne.Do(func() {
			cw.configuring = false
			startBtn.Enable()
			cancelBtn.Enable()

			// 处理配置错误
			if err != nil {
				log.Printf("[ConfigWindow] 配置失败: %v", err)
				outputText.SetText(outputText.Text + fmt.Sprintf("\n❌ 配置失败: %v\n", err))
				debouncer.scrollToBottom()
				cw.showMessageWindow("错误", "配置失败")
				return
			}

			log.Printf("[ConfigWindow] 配置完成，结果: %v", result.Success)

			// 显示配置结果
			if result.Success {
				cw.showMessageWindow("成功", "配置成功")
			} else {
				cw.showMessageWindow("错误", "配置失败")
			}
		})
	}()
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

	// 创建确认对话框窗口
	confirmWin := cw.app.NewWindow("确认取消配置")

	// 创建提示消息（说明完整取消的影响）
	msgLabel := widget.NewLabel(fmt.Sprintf("确定要取消端口 %d 的访问控制策略吗？\n\n这将删除该端口的所有规则配置。", port))
	msgLabel.Wrapping = fyne.TextWrapWord
	msgLabel.Alignment = fyne.TextAlignCenter

	// 确定按钮：执行完整取消配置
	confirmBtn := widget.NewButton("确定", func() {
		log.Printf("[ConfigWindow] 用户确认取消配置")
		confirmWin.Close()
		cw.doDeconfigure(port, "", outputText, startBtn, cancelBtn)
	})
	confirmBtn.Resize(fyne.NewSize(100, InputHeight))

	// 取消按钮：恢复按钮状态，不执行取消配置
	cancelBtnInternal := widget.NewButton("取消", func() {
		log.Printf("[ConfigWindow] 用户取消取消配置")
		fyne.Do(func() {
			cw.configuring = false
			startBtn.Enable()
			cancelBtn.Enable()
		})
		confirmWin.Close()
	})
	cancelBtnInternal.Resize(fyne.NewSize(100, InputHeight))

	// 组装按钮布局
	btnBox := container.NewHBox(
		confirmBtn,
		NewHSpace(20),
		cancelBtnInternal,
	)

	// 组装对话框内容
	content := container.NewVBox(
		NewVSpace(24),
		container.NewCenter(msgLabel),
		NewVSpace(24),
		container.NewCenter(btnBox),
		NewVSpace(24),
	)

	// 设置对话框背景和内容
	bg := canvas.NewRectangle(ColorBackground)
	confirmWin.SetContent(container.NewStack(bg, content))

	// 设置窗口关闭回调（用户点击右上角关闭按钮时恢复状态）
	confirmWin.SetOnClosed(func() {
		fyne.Do(func() {
			cw.configuring = false
			startBtn.Enable()
			cancelBtn.Enable()
		})
	})

	// 设置对话框尺寸并居中显示
	confirmWin.SetFixedSize(true)
	confirmWin.Resize(fyne.NewSize(480, 200))
	confirmWin.CenterOnScreen()
	confirmWin.Show()
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
