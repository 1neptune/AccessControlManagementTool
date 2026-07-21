package ui

import (
	"access-control-tool/internal/db"
	"access-control-tool/internal/models"
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

//go:embed logo.png
var logoFS embed.FS

// MainWindow 主窗口结构体
// 包含主窗口的所有UI组件和业务逻辑
// 字段说明:
//   window - Fyne主窗口对象
//   app - Fyne应用程序实例
//   theme - 自定义线性主题
//   serverTable - 服务器列表表格组件
//   serverDialog - 服务器添加/编辑对话框
//   configWindow - 访问控制配置窗口
//   pagination - 分页组件
//   searchPanel - 搜索面板组件
//   addBtn - 添加服务器按钮
//   editBtn - 编辑服务器按钮
//   deleteBtn - 删除服务器按钮
//   refreshBtn - 刷新列表按钮
//   tableScroll - 表格滚动容器
//   openConfigs - 记录正在打开配置窗口的服务器IP（防止重复打开和编辑删除）
//   searchName - 当前搜索条件-名称
//   searchIP - 当前搜索条件-IP
//   searchStatus - 当前搜索条件-状态
//   searchOSType - 当前搜索条件-系统类型
type MainWindow struct {
	window       fyne.Window
	app          fyne.App
	theme        *LinearTheme
	serverTable  *ServerTable
	serverDialog *ServerDialog
	configWindow *ConfigWindow
	pagination   *Pagination
	searchPanel  *SearchPanel
	addBtn       *widget.Button
	editBtn      *widget.Button
	deleteBtn    *widget.Button
	refreshBtn   *widget.Button
	tableScroll  *container.Scroll
	openConfigs   map[string]bool
	searchName    string
	searchIP      string
	searchStatus  string
	searchOSType  string
}

// NewMainWindow 创建主窗口实例
// 参数:
//   app - Fyne应用程序实例
//   theme - 自定义线性主题
// 返回值:
//   *MainWindow - 主窗口实例指针
// 流程:
//   1. 初始化MainWindow结构体，创建窗口对象
//   2. 设置窗口尺寸(1800x900)、非固定尺寸模式、主窗口标识
//   3. 窗口居中显示
//   4. 创建功能组件(createComponents)
//   5. 创建UI布局(createUI)
//   6. 加载服务器列表(loadServers)
//   7. 返回主窗口实例
func NewMainWindow(app fyne.App, theme *LinearTheme) *MainWindow {
	log.Printf("[MainWindow] 创建主窗口")

	mw := &MainWindow{
		window:      app.NewWindow("访问控制配置工具"),
		theme:       theme,
		app:         app,
		openConfigs: make(map[string]bool),
	}

	mw.window.Resize(fyne.NewSize(1800, 900))
	mw.window.SetFixedSize(false)
	mw.window.SetMaster()
	mw.window.CenterOnScreen()

	mw.createComponents()
	mw.createUI()
	mw.loadServers()

	log.Printf("[MainWindow] 主窗口创建完成")
	return mw
}

// ShowAndRun 显示主窗口并启动应用主循环
// 功能:
//   - 调用Fyne的ShowAndRun()方法，显示主窗口并进入事件循环
//   - 这是应用程序的主入口点，会阻塞直到主窗口关闭
func (mw *MainWindow) ShowAndRun() {
	log.Printf("[MainWindow] 显示主窗口并运行")
	mw.window.ShowAndRun()
}

// createComponents 创建所有功能组件
// 流程:
//   1. 创建服务器对话框(serverDialog) - 用于添加/编辑服务器
//   2. 创建配置窗口(configWindow) - 用于访问控制配置
//   3. 创建服务器表格(serverTable) - 包含配置回调:
//      - 配置回调: 点击配置按钮时触发，检查服务器是否已打开配置窗口，
//                  未打开则记录状态并打开配置窗口，窗口关闭时释放状态
//   4. 创建搜索面板(searchPanel) - 搜索条件变化时过滤表格并更新分页
//   5. 创建分页组件(pagination) - 页码变化时切换页码并更新分页显示
//   6. 初始化分页默认每页大小
// 注意:
//   - openConfigs map 用于防止同一服务器重复打开配置窗口
//   - 配置窗口关闭时通过回调自动从openConfigs中删除服务器IP
func (mw *MainWindow) createComponents() {
	log.Printf("[MainWindow] 创建功能组件")

	mw.serverDialog = NewServerDialog(mw.window, mw.app, "访问控制配置工具")
	mw.configWindow = NewConfigWindow(mw.app, mw.window)

	mw.serverTable = NewServerTable(
		func(server models.Server) {
			log.Printf("[MainWindow] 表格操作触发配置，服务器: %s", server.Name)
			if mw.openConfigs[server.Host] {
				log.Printf("[MainWindow] 服务器 %s 的配置窗口已打开，拒绝重复打开", server.Host)
				mw.serverDialog.showMessageWindow("提示", fmt.Sprintf("服务器 %s 的配置窗口已打开", server.Host))
				return
			}
			mw.openConfigs[server.Host] = true
			mw.configWindow.Show(server, func() {
				delete(mw.openConfigs, server.Host)
				log.Printf("[MainWindow] 配置窗口关闭，服务器: %s", server.Host)
			})
		},
	)

	mw.searchPanel = NewSearchPanel(func(name, ip, status, osType string) {
		log.Printf("[MainWindow] 搜索面板触发搜索")
		mw.searchName = name
		mw.searchIP = ip
		mw.searchStatus = status
		mw.searchOSType = osType
		mw.serverTable.FilterWithConditions(name, ip, status, osType)
		mw.updatePagination()
	})

	mw.pagination = NewPagination(
		func(page int) {
			log.Printf("[MainWindow] 分页变更: %d", page)
			mw.serverTable.SetCurrentPage(page)
			mw.updatePagination()
		},
	)
	mw.pagination.InitDefaultPageSize()
}

// createUI 创建主窗口的UI布局结构
// 布局层次:
//   ┌─────────────────────────────────────────────────────┐
//   │                    header (顶部工具栏)               │
//   │  [添加] [编辑] [删除] [刷新]                          │
//   ├─────────────────────────────────────────────────────┤
//   │              searchPanel (搜索面板)                  │
//   ├─────────────────────────────────────────────────────┤
//   │           serverListCard (服务器列表卡片)             │
//   │  ┌─────────────────────────────────────────────┐   │
//   │  │    headerContainer (表格表头)                │   │
//   │  ├─────────────────────────────────────────────┤   │
//   │  │    tableScroll (表格滚动区域)                │   │
//   │  └─────────────────────────────────────────────┘   │
//   ├─────────────────────────────────────────────────────┤
//   │                  pagination (分页组件)               │
//   └─────────────────────────────────────────────────────┘
// 按钮功能:
//   - 添加按钮: 打开添加服务器对话框，成功后刷新列表并保持搜索条件
//   - 编辑按钮: 打开编辑服务器对话框(openEditServer)
//   - 删除按钮: 删除选中服务器(deleteSelectedServers)
//   - 刷新按钮: 重新加载服务器列表并保持搜索条件
// 附加功能:
//   - 设置主窗口关闭回调
//   - 启动窗口尺寸监控协程(monitorWindowSize)
func (mw *MainWindow) createUI() {
	log.Printf("[MainWindow] 创建UI布局")

	mw.addBtn = NewButtonWithIcon("添加", theme.ContentAddIcon(), func() {
		log.Printf("[MainWindow] 用户点击添加按钮")
		mw.serverDialog.ShowAddDialog(func() {
			log.Printf("[MainWindow] 添加服务器成功，刷新列表并保持搜索条件")
			mw.loadServers()
			mw.applyCurrentSearchConditions()
		})
	})

	mw.editBtn = NewButtonWithIcon("编辑", theme.ContentCopyIcon(), func() {
		log.Printf("[MainWindow] 用户点击编辑按钮")
		mw.openEditServer()
	})

	mw.deleteBtn = NewButtonWithIcon("删除", theme.DeleteIcon(), func() {
		log.Printf("[MainWindow] 用户点击删除按钮")
		mw.deleteSelectedServers()
	})

	mw.refreshBtn = NewButtonWithIcon("刷新", theme.ViewRefreshIcon(), func() {
		log.Printf("[MainWindow] 用户点击刷新按钮")
		mw.loadServers()
		mw.applyCurrentSearchConditions()
	})

	logo := mw.loadLogo()

	header := container.NewBorder(
		nil,
		nil,
		logo,
		container.NewHBox(
			NewHSpace(SpacingS),
			mw.addBtn,
			NewHSpace(SpacingS),
			mw.editBtn,
			NewHSpace(SpacingS),
			mw.deleteBtn,
			NewHSpace(SpacingS),
			mw.refreshBtn,
			NewHSpace(SpacingS),
		),
	)

	headerContainer := mw.serverTable.Header()
	
	mw.tableScroll = container.NewScroll(mw.serverTable.Table())
	mw.tableScroll.SetMinSize(fyne.NewSize(TotalTableWidth, 520))
	
	tableContainer := container.NewBorder(headerContainer, nil, nil, nil, mw.tableScroll)
	
	serverListCard := NewCard(tableContainer)

	poweredByLabel := canvas.NewText("Powered by yaoyw", theme.ForegroundColor())
	poweredByLabel.TextStyle = fyne.TextStyle{Bold: true}
	poweredByLabel.Alignment = fyne.TextAlignCenter

	paginationBox := container.NewHBox(
		mw.pagination.Container(),
		NewHSpace(16),
	)

	bottomBar := container.NewHBox(
		layout.NewSpacer(),
		poweredByLabel,
		NewHSpace(40),
		paginationBox,
	)

	content := container.NewBorder(
		header,
		bottomBar,
		nil,
		nil,
		container.NewVBox(
			NewVSpace(8),
			mw.searchPanel.Container(),
			serverListCard,
		),
	)

	mw.window.SetContent(content)

	mw.window.SetOnClosed(func() {
		log.Printf("[MainWindow] 主窗口关闭")
	})

	go mw.monitorWindowSize()

	log.Printf("[MainWindow] UI布局创建完成")
}

// monitorWindowSize 监控窗口尺寸变化并重置分页
// 功能:
//   - 使用定时器每5秒检查一次表格每页大小
//   - 如果每页大小不是8条，则重置为默认值8
//   - 同时重置表格滚动区域最小尺寸和当前页码为1
//   - 更新分页显示
// 注意:
//   - 使用fyne.Do()确保UI操作在主线程执行，避免并发问题
//   - 定时器在函数退出时自动停止(defer ticker.Stop())
func (mw *MainWindow) monitorWindowSize() {
	ticker := time.NewTicker(5000 * time.Millisecond)
	defer ticker.Stop()
	
	for range ticker.C {
		fyne.Do(func() {
			if mw.serverTable.GetPageSize() != 8 {
				log.Printf("[MainWindow] 重置分页为8条")
				mw.serverTable.SetPageSize(8)
				mw.tableScroll.SetMinSize(fyne.NewSize(TotalTableWidth, 520))
				mw.serverTable.SetCurrentPage(1)
				mw.updatePagination()
			}
		})
	}
}

// loadServers 从数据库加载服务器列表
// 流程:
//   1. 调用db.ListServers()从数据库获取所有服务器记录
//   2. 如果加载失败，显示错误提示并返回
//   3. 如果加载成功，将服务器数据设置到表格组件
//   4. 更新分页信息
// 错误处理:
//   - 数据库查询失败时弹出错误提示窗口
func (mw *MainWindow) loadServers() {
	log.Printf("[MainWindow] 开始加载服务器列表")

	servers, err := db.ListServers()
	if err != nil {
		log.Printf("[MainWindow] 加载服务器列表失败: %v", err)
		mw.serverDialog.showMessageWindow("错误", "加载服务器列表失败")
		return
	}

	log.Printf("[MainWindow] 加载服务器列表成功，共 %d 台", len(servers))

	mw.serverTable.SetServers(servers)
	mw.updatePagination()
}

// updatePagination 更新分页组件显示信息
// 调用:
//   pagination.Update(currentPage, totalPages, pageSize, totalCount)
// 参数来源:
//   - 当前页码: serverTable.GetCurrentPage()
//   - 总页数: serverTable.GetTotalPages()
//   - 每页大小: serverTable.GetPageSize()
//   - 总记录数: 过滤后的服务器数量
// 使用场景:
//   - 加载服务器列表后
//   - 搜索条件变化后
//   - 页码切换后
func (mw *MainWindow) updatePagination() {
	log.Printf("[MainWindow] 更新分页信息")
	mw.pagination.Update(
		mw.serverTable.GetCurrentPage(),
		mw.serverTable.GetTotalPages(),
		mw.serverTable.GetPageSize(),
		len(mw.serverTable.GetFilteredServers()),
	)
}

// openEditServer 打开编辑服务器对话框
// 流程:
//   1. 获取表格中选中的单个服务器(GetSingleSelectedServer)
//   2. 如果未选择或选择多个，弹出警告提示并返回
//   3. 检查服务器是否正在打开配置窗口(openConfigs[server.Host])
//   4. 如果正在配置中，弹出警告提示并返回
//   5. 打开编辑对话框，编辑成功后刷新服务器列表并保持搜索条件
// 安全机制:
//   - 只能编辑单个服务器
//   - 正在配置中的服务器禁止编辑，防止数据不一致
func (mw *MainWindow) openEditServer() {
	log.Printf("[MainWindow] 打开编辑服务器对话框")

	server := mw.serverTable.GetSingleSelectedServer()
	if server == nil {
		log.Printf("[MainWindow] 未选择服务器或选择了多个服务器")
		mw.serverDialog.showMessageWindow("警告", "请选择一台服务器进行编辑")
		return
	}

	if mw.openConfigs[server.Host] {
		log.Printf("[MainWindow] 服务器 %s 正在配置中，禁止编辑", server.Host)
		mw.serverDialog.showMessageWindow("警告", fmt.Sprintf("服务器 %s 正在配置中，请先关闭配置窗口", server.Host))
		return
	}

	mw.serverDialog.ShowEditDialog(*server, func() {
		log.Printf("[MainWindow] 编辑服务器成功，刷新列表并保持搜索条件")
		mw.loadServers()
		mw.applyCurrentSearchConditions()
	})
}

// deleteSelectedServers 删除选中的服务器
// 流程:
//   1. 获取表格中所有选中的服务器(GetSelectedServers)
//   2. 如果未选择任何服务器，弹出警告提示并返回
//   3. 遍历选中的服务器，检查是否有正在配置中的服务器
//   4. 如果有正在配置中的服务器，弹出警告提示并返回
//   5. 显示确认对话框，用户确认后执行删除操作
//   6. 遍历删除每个服务器，记录删除结果日志
//   7. 删除完成后刷新服务器列表并保持搜索条件，显示成功提示
// 安全机制:
//   - 批量删除时，只要有一个服务器正在配置中就禁止删除
//   - 删除前需要用户确认，防止误操作
//   - 单个服务器删除失败不影响其他服务器删除
func (mw *MainWindow) deleteSelectedServers() {
	log.Printf("[MainWindow] 删除选中的服务器")

	selectedServers := mw.serverTable.GetSelectedServers()
	if len(selectedServers) == 0 {
		log.Printf("[MainWindow] 未选择要删除的服务器")
		mw.serverDialog.showMessageWindow("警告", "请先选择要删除的服务器")
		return
	}

	for _, server := range selectedServers {
		if mw.openConfigs[server.Host] {
			log.Printf("[MainWindow] 服务器 %s 正在配置中，禁止删除", server.Host)
			mw.serverDialog.showMessageWindow("警告", fmt.Sprintf("服务器 %s 正在配置中，请先关闭配置窗口", server.Host))
			return
		}
	}

	msg := fmt.Sprintf("确定要删除选中的 %d 台服务器吗？", len(selectedServers))
	mw.serverDialog.showConfirmWindow("确认删除", msg, func() {
		log.Printf("[MainWindow] 用户确认删除 %d 台服务器", len(selectedServers))

		for _, server := range selectedServers {
			if err := db.DeleteServer(server.ID); err != nil {
				log.Printf("[MainWindow] 删除服务器[%d] %s 失败: %v", server.ID, server.Name, err)
			} else {
				log.Printf("[MainWindow] 删除服务器[%d] %s 成功", server.ID, server.Name)
			}
		}

		mw.loadServers()
		mw.applyCurrentSearchConditions()
		mw.serverDialog.showMessageWindow("成功", "服务器删除成功！")
	})
}

// applyCurrentSearchConditions 应用当前搜索条件到服务器列表
// 调用时机:
//   - 添加服务器成功后
//   - 编辑服务器成功后
//   - 删除服务器成功后
//   - 刷新列表后需要保持搜索条件时
// 流程:
//   1. 检查是否有搜索条件（名称、IP、状态、系统类型）
//   2. 如果有条件，调用FilterWithConditions过滤列表
//   3. 更新分页显示
func (mw *MainWindow) applyCurrentSearchConditions() {
	log.Printf("[MainWindow] 应用当前搜索条件 - 名称: '%s', IP: '%s', 状态: '%s', 类型: '%s'",
		mw.searchName, mw.searchIP, mw.searchStatus, mw.searchOSType)

	if mw.searchName != "" || mw.searchIP != "" || mw.searchStatus != "全部" || mw.searchOSType != "全部" {
		mw.serverTable.FilterWithConditions(mw.searchName, mw.searchIP, mw.searchStatus, mw.searchOSType)
		mw.updatePagination()
	}
}

// loadLogo 加载logo图片并返回canvas对象
// 流程:
//   1. 从embed资源中读取logo.png文件
//   2. 解码图片数据
//   3. 创建canvas.Image对象，设置填充模式为保持宽高比
//   4. 设置图片最小尺寸为64x64，增大logo显示尺寸
//   5. 如果加载失败，返回一个空的透明占位符
// 返回值:
//   fyne.CanvasObject - logo图片对象或透明占位符
func (mw *MainWindow) loadLogo() fyne.CanvasObject {
	logoData, err := logoFS.ReadFile("logo.png")
	if err != nil {
		log.Printf("[MainWindow] 加载logo失败: %v", err)
		return canvas.NewRectangle(color.Transparent)
	}

	img, _, err := image.Decode(bytes.NewReader(logoData))
	if err != nil {
		log.Printf("[MainWindow] 解码logo图片失败: %v", err)
		return canvas.NewRectangle(color.Transparent)
	}

	logo := canvas.NewImageFromImage(img)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(64, 64))

	return logo
}