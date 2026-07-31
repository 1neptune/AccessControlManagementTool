package main

import (
	"access-control-tool/internal/db"
	"access-control-tool/internal/ui"
	"embed"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

//go:embed icons/logo_256.png
var logoFS embed.FS

// defaultListenAddr 默认TCP监听地址，用于进程间通信
const defaultListenAddr = "127.0.0.1:9999"

// DebugMode 调试模式开关
// 控制debug.log日志输出:
//   - true  : 输出日志到debug.log文件（追加模式）
//   - false : 不输出任何日志（默认值）
// 修改此值即可控制日志输出，方便调试
var DebugMode = false

// listenAddr 实际使用的TCP监听地址（支持自动端口检测）
var listenAddr string

// filterWriter 自定义日志写入器，用于过滤Fyne线程模型警告
// 
// Fyne线程模型问题说明:
//   从Fyne v2.6.0开始，框架引入了新的线程模型，要求所有UI更新操作必须在主协程(Goroutine)中执行。
//   如果在非主协程(如使用go func()创建的协程)中直接更新UI，Fyne会输出警告日志。
//   在未来的Fyne v2.9.0版本中，这个"安全保护"会被移除，届时在非主协程更新UI将直接导致程序崩溃。
// 
// 本项目解决方案:
//   本项目已通过使用fyne.Do()函数解决了线程模型问题，所有UI更新操作都已包装在fyne.Do()中执行。
//   但由于Fyne框架内部某些操作仍会触发警告，因此使用此过滤器在日志输出层面进行过滤，
//   避免debug.log文件被大量警告信息淹没。
// 
// 注意事项:
//   此过滤器仅用于过滤日志，不能替代正确的线程模型适配。
//   真正的解决方案是在所有非主协程中使用fyne.Do()来更新UI。
type filterWriter struct {
	writer io.Writer // 底层日志写入器，通常为文件句柄
}

// Write 实现io.Writer接口，过滤Fyne线程模型警告日志
// 参数:
//   p - 待写入的日志字节数据
// 返回:
//   n - 写入的字节数（与输入相同，表示"已处理"）
//   err - 错误信息（始终为nil）
// 
// 过滤逻辑:
//   1. 将字节数据转换为字符串
//   2. 检查是否包含Fyne线程模型警告的特征字符串
//   3. 如果包含，返回len(p)但不写入底层writer，实现静默过滤
//   4. 如果不包含，正常写入底层writer
func (fw *filterWriter) Write(p []byte) (n int, err error) {
	// 检查底层writer是否为nil，防止写入nil导致问题
	if fw.writer == nil {
		return len(p), nil
	}
	
	msg := string(p)
	
	// Fyne线程模型警告的特征字符串列表
	// 匹配这些字符串的日志将被过滤掉
	filterPatterns := []string{
		"This application has not been migrated to the fyne.Do threading model", // 线程模型迁移警告
		"The next major Fyne release will remove this safety",                      // 安全机制移除警告
		"docs.fyne.io/started/goroutines",                                         // 官方文档链接
	}
	
	// 检查日志消息是否包含任何过滤模式
	for _, pattern := range filterPatterns {
		if strings.Contains(msg, pattern) {
			// 匹配到警告模式，返回len(p)表示"已处理"，但不写入日志
			// 这样可以避免debug.log被大量重复警告信息污染
			return len(p), nil
		}
	}
	
	// 非警告日志，正常写入底层writer
	return fw.writer.Write(p)
}

// init 初始化函数，在main函数之前执行
// 主要功能:
//   1. 设置Windows兼容模式环境变量，强制使用软件渲染驱动
//   注意: 日志初始化移到main函数中，在单实例检测通过后执行，避免第二次启动时覆盖日志
func init() {
	if runtime.GOOS == "windows" {
		os.Setenv("FYNE_DRIVER", "software")
		os.Setenv("FYNE_ANGLE", "1")
		os.Setenv("FYNE_RENDERER", "software")
		os.Setenv("FYNE_DISABLE_GPU", "1")
		os.Setenv("FYNE_DISABLE_HARDWARE_ACCEL", "1")
		os.Setenv("LIBGL_ALWAYS_SOFTWARE", "1")
	}
}

// initLogger 初始化日志输出
// 根据DebugMode控制日志输出:
//   - DebugMode=true  : 输出到debug.log文件（追加模式，不覆盖旧日志）
//   - DebugMode=false : 日志输出到io.Discard，不产生任何日志文件
// 修改DebugMode变量即可切换日志模式，无需修改其他代码
func initLogger() {
	if !DebugMode {
		log.SetOutput(io.Discard)
		log.SetFlags(0)
		return
	}

	logFile, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("打开日志文件失败: %v", err)
		return
	}
	log.SetOutput(&filterWriter{writer: logFile})
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("日志初始化完成（调试模式已启用）")
}

// main 函数是程序的入口点
// 主要流程:
//   1. 设置程序崩溃恢复机制（defer recover）
//   2. 单实例检测：尝试连接本地TCP端口，如果成功说明程序已在运行
//      - 如果已运行，发送显示窗口命令并退出
//      - 如果未运行，创建TCP监听并继续启动
//   3. 初始化数据库
//   4. 创建Fyne应用实例
//   5. 设置应用图标
//   6. 创建自定义主题
//   7. 创建主窗口
//   8. 启动TCP监听协程
//   9. 显示主窗口并进入事件循环
func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("程序崩溃: %v\n%s", r, debug.Stack())
		}
	}()

	if runtime.GOOS == "windows" {
		// 自动检测可用端口，解决端口被占用的问题
		listenAddr = findAvailablePort()

		if sendShowCommand() {
			return
		}
	}

	initLogger()
	log.Println("程序启动")

	if err := db.InitDB(); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}



	log.Println("开始创建Fyne应用")
	myApp := app.NewWithID("com.example.access-control-tool")

	log.Println("加载应用图标")
	if logoData, err := logoFS.ReadFile("icons/logo_256.png"); err == nil {
		myApp.SetIcon(fyne.NewStaticResource("logo_256.png", logoData))
		log.Println("应用图标设置成功")
	} else {
		log.Printf("加载应用图标失败: %v", err)
	}

	log.Println("Fyne应用创建成功")

	log.Println("开始创建主题")
	linearTheme := ui.NewLinearTheme(false)
	myApp.Settings().SetTheme(linearTheme)
	log.Println("主题创建成功")

	log.Println("开始创建主窗口")
	mainWindow := ui.NewMainWindow(myApp, linearTheme)
	log.Println("主窗口创建完成")

	if runtime.GOOS == "windows" {
		go startTcpListener(mainWindow.Window())
	}

	log.Println("开始显示主窗口")
	mainWindow.ShowAndRun()

	log.Println("程序退出")

	// 调试模式下，程序退出时清空debug.log文件
	if DebugMode {
		logFile, _ := os.OpenFile("debug.log", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
		logFile.Close()
	}
}

// findAvailablePort 自动检测可用端口
// 从默认端口开始，依次尝试绑定，直到找到可用端口
// 返回格式: "127.0.0.1:端口号"
// 注意: 此函数在日志初始化之前调用，不使用log输出
func findAvailablePort() string {
	host, portStr, err := net.SplitHostPort(defaultListenAddr)
	if err != nil {
		return defaultListenAddr
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return defaultListenAddr
	}

	maxAttempts := 100
	for i := 0; i < maxAttempts; i++ {
		currentPort := port + i
		currentAddr := host + ":" + strconv.Itoa(currentPort)
		
		listener, err := net.Listen("tcp", currentAddr)
		if err == nil {
			listener.Close()
			return currentAddr
		}
	}

	return defaultListenAddr
}

// sendShowCommand 尝试连接本地TCP端口
// 如果连接成功且收到正确响应，说明程序已在运行，发送"SHOW"命令后返回true
// 如果连接失败或响应不正确，说明程序未运行，返回false
// 添加握手验证：发送"SHOW"后等待"OK"响应，防止误判其他程序占用端口
// 注意: 此函数在日志初始化之前调用，不使用log输出
func sendShowCommand() bool {
	if tryConnectAndVerify(defaultListenAddr) {
		return true
	}

	if listenAddr != "" && listenAddr != defaultListenAddr {
		if tryConnectAndVerify(listenAddr) {
			return true
		}
	}

	return false
}

// tryConnectAndVerify 尝试连接指定地址并验证响应
// 返回true表示成功连接到自己的程序，返回false表示连接失败或不是自己的程序
// 注意: 此函数在日志初始化之前调用，不使用log输出
func tryConnectAndVerify(addr string) bool {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return false
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	_, err = conn.Write([]byte("SHOW"))
	if err != nil {
		return false
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return false
	}

	response := strings.TrimSpace(string(buf[:n]))
	if response == "OK" {
		return true
	}

	return false
}

// startTcpListener 启动TCP监听协程
// 当收到其他实例的命令时，执行相应操作（如显示窗口）
func startTcpListener(window fyne.Window) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Printf("创建TCP监听失败: %v", err)
		return
	}
	defer listener.Close()

	log.Printf("TCP监听启动: %s", listenAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("接受连接失败: %v", err)
			continue
		}

		go handleConnection(conn, window)
	}
}

// handleConnection 处理TCP连接
// 读取客户端发送的命令并执行相应操作
// 收到"SHOW"命令后返回"OK"响应，用于验证是否是自己的程序
func handleConnection(conn net.Conn, window fyne.Window) {
	defer conn.Close()

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("读取数据失败: %v", err)
		return
	}

	command := string(buf[:n])
	log.Printf("收到命令: %s", command)

	if command == "SHOW" {
		// 返回"OK"响应，验证身份
		_, err := conn.Write([]byte("OK"))
		if err != nil {
			log.Printf("发送OK响应失败: %v", err)
		} else {
			log.Printf("发送OK响应成功")
		}
		
		fyne.Do(func() {
			log.Printf("执行显示窗口命令")
			window.Show()
			window.RequestFocus()
		})
	}
}


