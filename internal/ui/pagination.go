package ui

import (
	"fmt"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Pagination 分页组件结构体
// 用于管理服务器列表的分页显示
type Pagination struct {
	prevBtn      *widget.Button       // 上一页按钮
	nextBtn      *widget.Button       // 下一页按钮
	pageInfo     *canvas.Text         // 页码信息显示
	currentPage  int                  // 当前页码
	totalPages   int                  // 总页数
	pageSize     int                  // 每页显示条数
	totalCount   int                  // 总记录数
	onPageChange func(page int)       // 页码变更回调函数
}

// NewPagination 创建分页组件
// 参数:
//   onPageChange - 页码变更时的回调函数
// 返回值:
//   *Pagination - 分页组件实例指针
func NewPagination(onPageChange func(page int)) *Pagination {
	log.Printf("[Pagination] 创建分页组件")

	p := &Pagination{
		currentPage:  1,
		totalPages:   1,
		pageSize:     8,
		onPageChange: onPageChange,
	}

	p.createUI()
	return p
}

// createUI 创建分页控件界面
// 初始化上一页、下一页按钮和页码信息显示
func (p *Pagination) createUI() {
	p.pageInfo = NewLabel("第 1 / 1 页")

	p.prevBtn = NewButton("上一页", func() {
		log.Printf("[Pagination] 用户点击上一页，当前页: %d", p.currentPage)
		if p.currentPage > 1 {
			p.currentPage--
			p.updateUI()
			if p.onPageChange != nil {
				p.onPageChange(p.currentPage)
			}
		}
	})

	p.nextBtn = NewButton("下一页", func() {
		log.Printf("[Pagination] 用户点击下一页，当前页: %d, 总页数: %d", p.currentPage, p.totalPages)
		if p.currentPage < p.totalPages {
			p.currentPage++
			p.updateUI()
			if p.onPageChange != nil {
				p.onPageChange(p.currentPage)
			}
		}
	})
}

// InitDefaultPageSize 初始化默认每页大小（不触发回调）
// 当前为空实现，预留用于后续扩展
func (p *Pagination) InitDefaultPageSize() {
}

// Update 更新分页信息
// 参数:
//   currentPage - 当前页码
//   totalPages - 总页数
//   pageSize - 每页显示条数
//   totalCount - 总记录数
func (p *Pagination) Update(currentPage, totalPages, pageSize, totalCount int) {
	log.Printf("[Pagination] 更新分页信息 - 当前页: %d, 总页数: %d, 每页大小: %d, 总条数: %d", currentPage, totalPages, pageSize, totalCount)
	p.currentPage = currentPage
	p.totalPages = totalPages
	p.pageSize = pageSize
	p.totalCount = totalCount
	p.updateUI()
}

// updateUI 更新UI显示（不触发回调）
// 更新页码信息文本和按钮禁用状态
func (p *Pagination) updateUI() {
	p.pageInfo.Text = fmt.Sprintf("第 %d / %d 页，共 %d 条", p.currentPage, p.totalPages, p.totalCount)
	p.pageInfo.Refresh()
	
	if p.currentPage > 1 {
		p.prevBtn.Enable()
	} else {
		p.prevBtn.Disable()
	}
	
	if p.currentPage < p.totalPages {
		p.nextBtn.Enable()
	} else {
		p.nextBtn.Disable()
	}

	log.Printf("[Pagination] UI更新完成")
}

// Container 返回分页容器
// 返回值:
//   fyne.CanvasObject - 包含上一页按钮、页码信息、下一页按钮的水平容器
func (p *Pagination) Container() fyne.CanvasObject {
	return container.NewHBox(
		p.prevBtn,
		NewHSpace(8),
		p.pageInfo,
		NewHSpace(8),
		p.nextBtn,
	)
}