package ui

import (
	"syscall"
	"unsafe"
)

var (
	user32        = syscall.NewLazyDLL("user32.dll")
	findWindowW   = user32.NewProc("FindWindowW")
	getWindowRect = user32.NewProc("GetWindowRect")
	moveWindow    = user32.NewProc("MoveWindow")
)

// RECT Windows API 矩形结构体
// 用于存储窗口的位置和尺寸信息
type RECT struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

// findWindow 根据窗口标题查找窗口句柄
// 参数:
//   title - 窗口标题
// 返回值:
//   uintptr - 窗口句柄，未找到返回0
func findWindow(title string) uintptr {
	wTitle, _ := syscall.UTF16PtrFromString(title)
	ret, _, _ := findWindowW.Call(0, uintptr(unsafe.Pointer(wTitle)))
	return ret
}

// getWindowPosition 获取窗口位置和尺寸
// 参数:
//   hwnd - 窗口句柄
// 返回值:
//   left - 窗口左上角X坐标
//   top - 窗口左上角Y坐标
//   width - 窗口宽度
//   height - 窗口高度
func getWindowPosition(hwnd uintptr) (left, top, width, height int32) {
	var rect RECT
	getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	return rect.Left, rect.Top, rect.Right - rect.Left, rect.Bottom - rect.Top
}

// setWindowPosition 设置窗口位置和尺寸
// 参数:
//   hwnd - 窗口句柄
//   x - 窗口左上角X坐标
//   y - 窗口左上角Y坐标
//   width - 窗口宽度
//   height - 窗口高度
func setWindowPosition(hwnd uintptr, x, y, width, height int32) {
	moveWindow.Call(hwnd, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 1)
}

// centerWindowRelativeTo 将子窗口相对于父窗口居中显示
// 参数:
//   parentTitle - 父窗口标题
//   childTitle - 子窗口标题
//   childWidth - 子窗口宽度
//   childHeight - 子窗口高度
func centerWindowRelativeTo(parentTitle, childTitle string, childWidth, childHeight int32) {
	parentHWND := findWindow(parentTitle)
	if parentHWND == 0 {
		return
	}

	childHWND := findWindow(childTitle)
	if childHWND == 0 {
		return
	}

	pLeft, pTop, pWidth, pHeight := getWindowPosition(parentHWND)

	x := pLeft + (pWidth-childWidth)/2
	y := pTop + (pHeight-childHeight)/2

	setWindowPosition(childHWND, x, y, childWidth, childHeight)
}

// centerWindowOnScreen 将窗口在屏幕上居中显示
// 参数:
//   childTitle - 窗口标题
//   childWidth - 窗口宽度
//   childHeight - 窗口高度
func centerWindowOnScreen(childTitle string, childWidth, childHeight int32) {
	childHWND := findWindow(childTitle)
	if childHWND == 0 {
		return
	}

	screenWidth := int32(getSystemMetrics(0))
	screenHeight := int32(getSystemMetrics(1))

	x := (screenWidth - childWidth) / 2
	y := (screenHeight - childHeight) / 2

	setWindowPosition(childHWND, x, y, childWidth, childHeight)
}

// getSystemMetrics 获取系统度量值
// 参数:
//   index - 度量值索引，0表示屏幕宽度，1表示屏幕高度
// 返回值:
//   uintptr - 系统度量值
func getSystemMetrics(index int32) uintptr {
	getSystemMetricsProc := user32.NewProc("GetSystemMetrics")
	ret, _, _ := getSystemMetricsProc.Call(uintptr(index))
	return ret
}