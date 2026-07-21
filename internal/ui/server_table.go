package ui

import (
	"access-control-tool/internal/models"
	"access-control-tool/internal/utils"
	"log"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const (
	ColWidthSelect      = 60
	ColWidthName        = 150
	ColWidthIP          = 130
	ColWidthPort        = 70
	ColWidthOSType      = 90
	ColWidthOSVersion   = 340
	ColWidthKernelVer   = 240
	ColWidthKernelArch  = 90
	ColWidthHostname    = 180
	ColWidthStatus      = 70
	ColWidthConfig      = 80

	TotalTableWidth = ColWidthSelect + ColWidthName + ColWidthIP + ColWidthPort + ColWidthOSType +
		ColWidthOSVersion + ColWidthKernelVer + ColWidthKernelArch + ColWidthHostname + ColWidthStatus +
		ColWidthConfig
)

// ServerTable 服务器表格组件结构体
// 用于显示服务器列表，支持分页、搜索过滤、在线状态检测和行操作
type ServerTable struct {
	table           *widget.Table           // Fyne表格组件
	servers         []models.Server         // 原始服务器列表
	filteredServers []models.Server         // 过滤后的服务器列表
	selectedID      uint                    // 当前选中的单个服务器ID
	selectedIDs     map[uint]bool           // 选中的服务器ID集合（支持多选）
	onlineStatus    map[uint]bool           // 服务器在线状态映射（key为服务器ID）
	currentPage     int                     // 当前页码
	pageSize        int                     // 每页显示条数
	onConfig        func(models.Server)     // 配置按钮点击回调
	headerSelectBtn *widget.Button          // 表头全选按钮
}

// NewServerTable 创建服务器表格组件
// 参数:
//   onConfig - 配置按钮点击回调函数
// 返回值:
//   *ServerTable - 服务器表格实例指针
func NewServerTable(onConfig func(models.Server)) *ServerTable {
	log.Printf("[ServerTable] 开始创建服务器表格组件")

	st := &ServerTable{
		selectedIDs:  make(map[uint]bool),
		onlineStatus: make(map[uint]bool),
		currentPage:  1,
		pageSize:     8,
		onConfig:     onConfig,
	}

	st.createTable()

	log.Printf("[ServerTable] 服务器表格组件创建完成")
	return st
}

// createTable 创建表格组件
// 初始化表格的行列数量、单元格模板、单元格更新函数
// 设置行高、列宽和选中事件处理
func (st *ServerTable) createTable() {
	st.table = widget.NewTable(
		func() (int, int) {
			start := (st.currentPage - 1) * st.pageSize
			end := start + st.pageSize
			if end > len(st.filteredServers) {
				end = len(st.filteredServers)
			}
			return end - start, 11
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Alignment = fyne.TextAlignCenter
			return label
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			st.updateCell(id, obj)
		},
	)

	rowHeight := float32(50)
	for i := 0; i < 500; i++ {
		st.table.SetRowHeight(i, rowHeight)
	}
	log.Printf("[ServerTable] 表格行高设置为: %.2fpx", rowHeight)

	st.table.SetColumnWidth(0, ColWidthSelect)
	st.table.SetColumnWidth(1, ColWidthName)
	st.table.SetColumnWidth(2, ColWidthIP)
	st.table.SetColumnWidth(3, ColWidthPort)
	st.table.SetColumnWidth(4, ColWidthOSType)
	st.table.SetColumnWidth(5, ColWidthOSVersion)
	st.table.SetColumnWidth(6, ColWidthKernelVer)
	st.table.SetColumnWidth(7, ColWidthKernelArch)
	st.table.SetColumnWidth(8, ColWidthHostname)
	st.table.SetColumnWidth(9, ColWidthStatus)
	st.table.SetColumnWidth(10, ColWidthConfig)

	st.table.OnSelected = func(id widget.TableCellID) {
		st.handleCellClick(id)
		st.table.Unselect(id)
	}
}

// Header 返回表格表头组件
// 返回值:
//   fyne.CanvasObject - 包含全选按钮和各列标题的水平容器
func (st *ServerTable) Header() fyne.CanvasObject {
	st.headerSelectBtn = widget.NewButton("☐", func() {
		st.handleSelectAll()
	})
	st.headerSelectBtn.Resize(fyne.NewSize(ColWidthSelect, 50))
	st.headerSelectBtn.Importance = widget.LowImportance

	headerCells := []fyne.CanvasObject{
		container.NewGridWrap(fyne.NewSize(ColWidthSelect, 50), container.NewCenter(st.headerSelectBtn)),
		container.NewGridWrap(fyne.NewSize(ColWidthName, 50), NewLabelBold("服务器名称")),
		container.NewGridWrap(fyne.NewSize(ColWidthIP, 50), NewLabelBold("IP地址")),
		container.NewGridWrap(fyne.NewSize(ColWidthPort, 50), NewLabelBold("端口")),
		container.NewGridWrap(fyne.NewSize(ColWidthOSType, 50), NewLabelBold("系统类型")),
		container.NewGridWrap(fyne.NewSize(ColWidthOSVersion, 50), NewLabelBold("系统版本")),
		container.NewGridWrap(fyne.NewSize(ColWidthKernelVer, 50), NewLabelBold("内核版本")),
		container.NewGridWrap(fyne.NewSize(ColWidthKernelArch, 50), NewLabelBold("内核架构")),
		container.NewGridWrap(fyne.NewSize(ColWidthHostname, 50), NewLabelBold("主机名")),
		container.NewGridWrap(fyne.NewSize(ColWidthStatus, 50), NewLabelBold("状态")),
		container.NewGridWrap(fyne.NewSize(ColWidthConfig, 50), NewLabelBold("访问控制")),
	}

	headerBox := container.NewHBox(headerCells...)
	headerBox.Resize(fyne.NewSize(TotalTableWidth, 50))
	
	return headerBox
}

// Table 返回表格组件
// 返回值:
//   *widget.Table - Fyne表格组件
func (st *ServerTable) Table() *widget.Table {
	return st.table
}

// updateCell 更新表格单元格内容
// 参数:
//   id - 单元格ID（行号和列号）
//   obj - 单元格对象（Label类型）
// 功能:
//   根据列号填充不同的内容（复选框、名称、IP、端口、系统类型等）
func (st *ServerTable) updateCell(id widget.TableCellID, obj fyne.CanvasObject) {
	serverIndex := (st.currentPage-1)*st.pageSize + id.Row
	if serverIndex >= len(st.filteredServers) {
		return
	}
	server := st.filteredServers[serverIndex]

	label, ok := obj.(*widget.Label)
	if !ok {
		return
	}

	var text string
	switch id.Col {
	case 0:
		if st.selectedIDs[server.ID] {
			text = "☑"
		} else {
			text = "☐"
		}
		label.TextStyle = fyne.TextStyle{Bold: true}
		label.Alignment = fyne.TextAlignCenter
	case 1:
		text = server.Name
	case 2:
		text = server.Host
	case 3:
		text = strconv.Itoa(server.SSHPort)
	case 4:
		text = server.OSType
	case 5:
		text = server.OSVersion
	case 6:
		text = server.KernelVersion
	case 7:
		text = server.KernelArch
	case 8:
		text = server.Hostname
	case 9:
		if st.onlineStatus[server.ID] {
			text = "在线"
		} else {
			text = "离线"
		}
	case 10:
		text = "配置"
	}

	label.SetText(text)
}

// handleCellClick 处理单元格点击事件
// 参数:
//   id - 单元格ID（行号和列号）
// 功能:
//   - 第0列（复选框）: 切换选中状态
//   - 第10列（访问控制）: 调用配置回调
//   - 第11列（AI运维）: 调用运维回调
func (st *ServerTable) handleCellClick(id widget.TableCellID) {
	serverIndex := (st.currentPage-1)*st.pageSize + id.Row
	if serverIndex >= len(st.filteredServers) {
		return
	}
	server := st.filteredServers[serverIndex]

	switch id.Col {
	case 0:
		if st.selectedIDs[server.ID] {
			delete(st.selectedIDs, server.ID)
			log.Printf("[ServerTable] 取消选中服务器[%d] %s", server.ID, server.Name)
		} else {
			st.selectedIDs[server.ID] = true
			st.selectedID = server.ID
			log.Printf("[ServerTable] 选中服务器[%d] %s", server.ID, server.Name)
		}
		st.table.Refresh()
		st.updateHeaderSelectBtn()
	case 10:
		log.Printf("[ServerTable] 点击访问控制列，服务器: %s", server.Name)
		if st.onConfig != nil {
			st.onConfig(server)
		}
	}
}

// updateHeaderSelectBtn 更新表头全选按钮状态
// 根据当前过滤列表中所有服务器是否都被选中来更新按钮文本（☑/☐）
func (st *ServerTable) updateHeaderSelectBtn() {
	if st.headerSelectBtn == nil {
		return
	}

	allSelected := true
	for _, server := range st.filteredServers {
		if !st.selectedIDs[server.ID] {
			allSelected = false
			break
		}
	}

	if allSelected {
		st.headerSelectBtn.SetText("☑")
	} else {
		st.headerSelectBtn.SetText("☐")
	}
}

// handleSelectAll 处理全选/取消全选操作
// 如果当前已全选则取消全选，否则选中所有服务器
func (st *ServerTable) handleSelectAll() {
	allSelected := true
	for _, server := range st.filteredServers {
		if !st.selectedIDs[server.ID] {
			allSelected = false
			break
		}
	}

	if allSelected {
		for id := range st.selectedIDs {
			delete(st.selectedIDs, id)
		}
		st.selectedID = 0
		log.Printf("[ServerTable] 取消全选")
	} else {
		for id := range st.selectedIDs {
			delete(st.selectedIDs, id)
		}
		for _, server := range st.filteredServers {
			st.selectedIDs[server.ID] = true
		}
		if len(st.filteredServers) > 0 {
			st.selectedID = st.filteredServers[0].ID
		}
		log.Printf("[ServerTable] 全选 %d 台服务器", len(st.filteredServers))
	}
	st.table.Refresh()
	st.updateHeaderSelectBtn()
}

// SetServers 设置服务器数据
// 参数:
//   servers - 服务器列表
// 功能:
//   更新原始数据和过滤数据，重置页码为1，触发在线状态检测
func (st *ServerTable) SetServers(servers []models.Server) {
	log.Printf("[ServerTable] 设置服务器数据，共 %d 台", len(servers))
	st.servers = servers
	st.filteredServers = servers
	st.currentPage = 1
	st.checkOnlineStatus()
	st.table.Refresh()
	st.updateHeaderSelectBtn()
}

// Filter 按关键词搜索过滤
// 参数:
//   keyword - 搜索关键词
// 功能:
//   根据关键词模糊匹配服务器名称和IP地址
func (st *ServerTable) Filter(keyword string) {
	log.Printf("[ServerTable] 搜索过滤: '%s'", keyword)
	if keyword == "" {
		st.filteredServers = st.servers
	} else {
		st.filteredServers = []models.Server{}
		for _, server := range st.servers {
			if containsIgnoreCase(server.Name, keyword) || containsIgnoreCase(server.Host, keyword) {
				st.filteredServers = append(st.filteredServers, server)
			}
		}
	}
	st.currentPage = 1
	st.table.Refresh()
	st.updateHeaderSelectBtn()
}

// FilterWithConditions 多条件过滤
// 参数:
//   name - 名称关键词（为空不过滤）
//   ip - IP关键词（为空不过滤）
//   status - 状态筛选（"全部"/"在线"/"离线"）
//   osType - 系统类型筛选（"全部"/"Linux"/"Windows"）
// 功能:
//   根据多个条件组合过滤服务器列表
func (st *ServerTable) FilterWithConditions(name, ip, status, osType string) {
	log.Printf("[ServerTable] 多条件过滤 - 名称: '%s', IP: '%s', 状态: '%s', 系统类型: '%s'", name, ip, status, osType)

	st.filteredServers = []models.Server{}
	for _, server := range st.servers {
		if name != "" && !containsIgnoreCase(server.Name, name) {
			continue
		}
		if ip != "" && !containsIgnoreCase(server.Host, ip) {
			continue
		}
		if status != "全部" {
			isOnline := st.onlineStatus[server.ID]
			if (status == "在线" && !isOnline) || (status == "离线" && isOnline) {
				continue
			}
		}
		if osType != "全部" {
			if (osType == "Linux" && server.OSType != "linux") ||
				(osType == "Windows" && server.OSType != "windows") {
				continue
			}
		}
		st.filteredServers = append(st.filteredServers, server)
	}
	st.currentPage = 1
	st.table.Refresh()
	st.updateHeaderSelectBtn()
}

// SetPageSize 设置每页显示数量
// 参数:
//   size - 每页条数
func (st *ServerTable) SetPageSize(size int) {
	log.Printf("[ServerTable] 设置每页显示数量: %d", size)
	st.pageSize = size
	st.currentPage = 1
	st.table.Refresh()
}

// SetCurrentPage 设置当前页码
// 参数:
//   page - 页码
func (st *ServerTable) SetCurrentPage(page int) {
	log.Printf("[ServerTable] 设置当前页码: %d", page)
	st.currentPage = page
	st.table.Refresh()
}

// GetCurrentPage 获取当前页码
// 返回值:
//   int - 当前页码
func (st *ServerTable) GetCurrentPage() int {
	return st.currentPage
}

// GetTotalPages 获取总页数
// 返回值:
//   int - 总页数
func (st *ServerTable) GetTotalPages() int {
	return (len(st.filteredServers) + st.pageSize - 1) / st.pageSize
}

// GetPageSize 获取每页显示数量
// 返回值:
//   int - 每页条数
func (st *ServerTable) GetPageSize() int {
	return st.pageSize
}

// GetFilteredServers 获取过滤后的服务器列表
// 返回值:
//   []models.Server - 过滤后的服务器列表
func (st *ServerTable) GetFilteredServers() []models.Server {
	return st.filteredServers
}

// GetSelectedServers 获取选中的服务器列表
// 返回值:
//   []models.Server - 选中的服务器列表
func (st *ServerTable) GetSelectedServers() []models.Server {
	var selected []models.Server
	for _, server := range st.filteredServers {
		if st.selectedIDs[server.ID] {
			selected = append(selected, server)
		}
	}
	log.Printf("[ServerTable] 获取选中服务器，共 %d 台", len(selected))
	return selected
}

// GetSingleSelectedServer 获取单个选中的服务器
// 返回值:
//   *models.Server - 单个选中的服务器指针，选中多个或未选中时返回nil
func (st *ServerTable) GetSingleSelectedServer() *models.Server {
	var selected *models.Server
	for _, server := range st.filteredServers {
		if st.selectedIDs[server.ID] {
			if selected != nil {
				log.Printf("[ServerTable] 发现多个选中服务器，返回nil")
				return nil
			}
			s := server
			selected = &s
		}
	}
	return selected
}

// checkOnlineStatus 异步检测服务器在线状态
// 为每个服务器启动独立goroutine，通过TCP连接测试检测连通性
func (st *ServerTable) checkOnlineStatus() {
	log.Printf("[ServerTable] 开始异步检测服务器在线状态")
	for _, server := range st.servers {
		go func(s models.Server) {
			log.Printf("[ServerTable] 检测服务器[%d] %s:%d 连通性", s.ID, s.Host, s.SSHPort)
			connected := utils.TestConnectivity(s.Host, s.SSHPort)
			log.Printf("[ServerTable] 服务器[%d] %s:%d 连通性检测结果: %v", s.ID, s.Host, s.SSHPort, connected)
			fyne.Do(func() {
				st.onlineStatus[s.ID] = connected
				st.table.Refresh()
			})
		}(server)
	}
}

// containsIgnoreCase 忽略大小写检查子字符串是否存在
// 参数:
//   str - 源字符串
//   substr - 子字符串
// 返回值:
//   bool - 是否包含子字符串
func containsIgnoreCase(str, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(str)-len(substr); i++ {
		if toLower(str[i:i+len(substr)]) == toLower(substr) {
			return true
		}
	}
	return false
}

// toLower 将字符串转换为小写
// 参数:
//   str - 源字符串
// 返回值:
//   string - 小写字符串
func toLower(str string) string {
	result := []rune(str)
	for i, r := range result {
		if r >= 'A' && r <= 'Z' {
			result[i] = r + ('a' - 'A')
		}
	}
	return string(result)
}