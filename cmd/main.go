package main

import (
	"access-control-tool/internal/db"
	"access-control-tool/internal/ui"
	"embed"
	"log"
	"os"
	"runtime"
	"runtime/debug"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

//go:embed icons/logo_256.png
var logoFS embed.FS

func init() {
	logFile, err := os.OpenFile("debug.log", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if runtime.GOOS == "windows" {
		os.Setenv("FYNE_DRIVER", "software")
		os.Setenv("FYNE_ANGLE", "1")
		os.Setenv("FYNE_RENDERER", "software")
		os.Setenv("FYNE_DISABLE_GPU", "1")
		os.Setenv("FYNE_DISABLE_HARDWARE_ACCEL", "1")
		os.Setenv("LIBGL_ALWAYS_SOFTWARE", "1")
		log.Println("Windows兼容模式已启用，强制使用软件渲染驱动")
	}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("程序崩溃: %v\n%s", r, debug.Stack())
		}
	}()

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

	log.Println("开始显示主窗口")
	mainWindow.ShowAndRun()

	log.Println("程序退出")
}