package ui

import (
	"access-control-tool/internal/db"
	"access-control-tool/internal/detector"
	"access-control-tool/internal/models"
	"access-control-tool/internal/utils"
	"log"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ServerDialog 服务器对话框管理器结构体
// 负责管理服务器的添加、编辑对话框以及消息弹窗
// 字段说明:
//   parent - 父窗口引用
//   app - Fyne应用程序实例
//   addWin - 添加服务器对话框窗口
//   editWin - 编辑服务器对话框窗口
//   messageWins - 消息弹窗列表（用于管理多个弹窗）
//   parentTitle - 父窗口标题（用于窗口定位）
type ServerDialog struct {
	parent       fyne.Window
	app          fyne.App
	addWin       fyne.Window
	editWin      fyne.Window
	messageWins  []fyne.Window
	parentTitle  string
}

// NewServerDialog 创建服务器对话框管理器实例
// 参数:
//   parent - 父窗口引用
//   app - Fyne应用程序实例
//   parentTitle - 父窗口标题（用于子窗口定位）
// 返回值:
//   *ServerDialog - 对话框管理器实例指针
func NewServerDialog(parent fyne.Window, app fyne.App, parentTitle string) *ServerDialog {
	log.Printf("[ServerDialog] 创建服务器对话框管理器")
	return &ServerDialog{
		parent:      parent,
		app:         app,
		parentTitle: parentTitle,
		messageWins: make([]fyne.Window, 0),
	}
}

// showMessageWindow 显示消息弹窗
// 参数:
//   title - 弹窗标题
//   message - 弹窗内容
// 功能:
//   - 如果已存在相同标题的弹窗，先关闭旧弹窗
//   - 创建固定尺寸(480x160)的弹窗，白色背景，居中显示
//   - 包含消息标签和确定按钮，点击确定关闭弹窗
//   - 弹窗关闭时自动从messageWins列表中移除
func (sd *ServerDialog) showMessageWindow(title, message string) {
	for _, win := range sd.messageWins {
		if win.Title() == title {
			win.Close()
			break
		}
	}

	msgWin := sd.app.NewWindow(title)
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

	msgWin.SetOnClosed(func() {
		for i, win := range sd.messageWins {
			if win == msgWin {
				sd.messageWins = append(sd.messageWins[:i], sd.messageWins[i+1:]...)
				break
			}
		}
	})

	msgWin.Resize(fyne.NewSize(480, 160))
	msgWin.CenterOnScreen()
	msgWin.Show()

	sd.messageWins = append(sd.messageWins, msgWin)
}

// showConfirmWindow 显示确认对话框
// 参数:
//   title - 对话框标题
//   message - 对话框内容
//   onConfirm - 用户点击确定后的回调函数
// 功能:
//   - 如果已存在相同标题的对话框，先关闭旧对话框
//   - 创建固定尺寸(480x160)的对话框，白色背景，居中显示
//   - 包含消息标签、确定按钮和取消按钮
//   - 点击确定关闭对话框并调用onConfirm回调
//   - 点击取消仅关闭对话框
//   - 对话框关闭时自动从messageWins列表中移除
func (sd *ServerDialog) showConfirmWindow(title, message string, onConfirm func()) {
	for _, win := range sd.messageWins {
		if win.Title() == title {
			win.Close()
			break
		}
	}

	confirmWin := sd.app.NewWindow(title)
	confirmWin.SetFixedSize(true)

	msgLabel := widget.NewLabel(message)
	msgLabel.Wrapping = fyne.TextWrapOff
	msgLabel.Alignment = fyne.TextAlignCenter

	okBtn := widget.NewButton("确定", func() {
		confirmWin.Close()
		if onConfirm != nil {
			onConfirm()
		}
	})
	okBtn.Resize(fyne.NewSize(120, InputHeight))

	cancelBtn := widget.NewButton("取消", func() {
		confirmWin.Close()
	})
	cancelBtn.Resize(fyne.NewSize(120, InputHeight))

	okBorder := canvas.NewRectangle(ColorBorder)
	okBorder.CornerRadius = CornerRadius
	okBorder.SetMinSize(fyne.NewSize(120, InputHeight))
	okField := container.NewStack(okBorder, container.NewPadded(okBtn))

	cancelBorder := canvas.NewRectangle(ColorBorder)
	cancelBorder.CornerRadius = CornerRadius
	cancelBorder.SetMinSize(fyne.NewSize(120, InputHeight))
	cancelField := container.NewStack(cancelBorder, container.NewPadded(cancelBtn))

	content := container.NewVBox(
		NewVSpace(SpacingL),
		container.NewCenter(msgLabel),
		NewVSpace(SpacingL),
		container.NewCenter(container.NewHBox(
			okField,
			NewHSpace(SpacingM),
			cancelField,
		)),
		NewVSpace(SpacingL),
	)

	bg := canvas.NewRectangle(ColorBackground)
	confirmWin.SetContent(container.NewStack(bg, content))

	confirmWin.SetOnClosed(func() {
		for i, win := range sd.messageWins {
			if win == confirmWin {
				sd.messageWins = append(sd.messageWins[:i], sd.messageWins[i+1:]...)
				break
			}
		}
	})

	confirmWin.Resize(fyne.NewSize(480, 160))
	confirmWin.CenterOnScreen()
	confirmWin.Show()

	sd.messageWins = append(sd.messageWins, confirmWin)
}

func (sd *ServerDialog) ShowAddDialog(onSuccess func()) {
	log.Printf("[ServerDialog] 显示添加服务器窗口")

	if sd.addWin != nil {
		log.Printf("[ServerDialog] 添加窗口已存在，显示并聚焦")
		sd.addWin.Show()
		sd.addWin.Resize(fyne.NewSize(420, 400))
		sd.addWin.SetFixedSize(true)
		sd.addWin.RequestFocus()
		return
	}

	addWin := sd.app.NewWindow("添加服务器")
	sd.addWin = addWin

	log.Printf("[ServerDialog] 创建添加窗口，尺寸: 420x400")
	addWin.Resize(fyne.NewSize(420, 400))

	addWin.SetOnClosed(func() {
		log.Printf("[ServerDialog] 添加窗口关闭")
		sd.addWin = nil
	})

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("服务器名称")

	hostEntry := widget.NewEntry()
	hostEntry.SetPlaceHolder("主机地址")

	portEntry := widget.NewEntry()
	portEntry.SetPlaceHolder("SSH端口")
	portEntry.SetText("22")

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("用户名")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("密码")

	inputWidth := float32(280)

	nameBorder := canvas.NewRectangle(ColorBorder)
	nameBorder.CornerRadius = CornerRadius
	nameBorder.SetMinSize(fyne.NewSize(inputWidth, InputHeight))
	nameField := container.NewStack(nameBorder, container.NewPadded(nameEntry))

	hostBorder := canvas.NewRectangle(ColorBorder)
	hostBorder.CornerRadius = CornerRadius
	hostBorder.SetMinSize(fyne.NewSize(inputWidth, InputHeight))
	hostField := container.NewStack(hostBorder, container.NewPadded(hostEntry))

	portBorder := canvas.NewRectangle(ColorBorder)
	portBorder.CornerRadius = CornerRadius
	portBorder.SetMinSize(fyne.NewSize(inputWidth, InputHeight))
	portField := container.NewStack(portBorder, container.NewPadded(portEntry))

	usernameBorder := canvas.NewRectangle(ColorBorder)
	usernameBorder.CornerRadius = CornerRadius
	usernameBorder.SetMinSize(fyne.NewSize(inputWidth, InputHeight))
	usernameField := container.NewStack(usernameBorder, container.NewPadded(usernameEntry))

	passwordBorder := canvas.NewRectangle(ColorBorder)
	passwordBorder.CornerRadius = CornerRadius
	passwordBorder.SetMinSize(fyne.NewSize(inputWidth, InputHeight))
	passwordField := container.NewStack(passwordBorder, container.NewPadded(passwordEntry))

	labelWidth := float32(70)

	nameLabel := canvas.NewText("名称 *", ColorText)
	nameLabel.TextSize = 14
	nameLabelBg := canvas.NewRectangle(ColorBackground)
	nameLabelBg.SetMinSize(fyne.NewSize(labelWidth, InputHeight))
	nameLabelField := container.NewStack(nameLabelBg, container.NewCenter(nameLabel))

	hostLabel := canvas.NewText("主机 *", ColorText)
	hostLabel.TextSize = 14
	hostLabelBg := canvas.NewRectangle(ColorBackground)
	hostLabelBg.SetMinSize(fyne.NewSize(labelWidth, InputHeight))
	hostLabelField := container.NewStack(hostLabelBg, container.NewCenter(hostLabel))

	portLabel := canvas.NewText("远程端口 *", ColorText)
	portLabel.TextSize = 14
	portLabelBg := canvas.NewRectangle(ColorBackground)
	portLabelBg.SetMinSize(fyne.NewSize(labelWidth, InputHeight))
	portLabelField := container.NewStack(portLabelBg, container.NewCenter(portLabel))

	usernameLabel := canvas.NewText("用户名 *", ColorText)
	usernameLabel.TextSize = 14
	usernameLabelBg := canvas.NewRectangle(ColorBackground)
	usernameLabelBg.SetMinSize(fyne.NewSize(labelWidth, InputHeight))
	usernameLabelField := container.NewStack(usernameLabelBg, container.NewCenter(usernameLabel))

	passwordLabel := canvas.NewText("密码 *", ColorText)
	passwordLabel.TextSize = 14
	passwordLabelBg := canvas.NewRectangle(ColorBackground)
	passwordLabelBg.SetMinSize(fyne.NewSize(labelWidth, InputHeight))
	passwordLabelField := container.NewStack(passwordLabelBg, container.NewCenter(passwordLabel))

	var submitBtn *widget.Button
	submitBtn = widget.NewButton("测试连接", func() {
		log.Printf("[ServerDialog] 用户点击测试连接按钮")

		if nameEntry.Text == "" {
			log.Printf("[ServerDialog] 服务器名称为空")
			sd.showMessageWindow("错误", "请填写服务器名称")
			return
		}

		if hostEntry.Text == "" {
			log.Printf("[ServerDialog] 主机地址为空")
			sd.showMessageWindow("错误", "请填写主机地址")
			return
		}

		if existingServer, _ := db.GetServerByHost(hostEntry.Text); existingServer != nil {
			log.Printf("[ServerDialog] 主机地址已存在: %s", hostEntry.Text)
			sd.showMessageWindow("错误", "主机地址已存在")
			return
		}

		if usernameEntry.Text == "" {
			log.Printf("[ServerDialog] 用户名为空")
			sd.showMessageWindow("错误", "请填写用户名")
			return
		}

		if passwordEntry.Text == "" {
			log.Printf("[ServerDialog] 密码为空")
			sd.showMessageWindow("错误", "请填写密码")
			return
		}

		port, err := strconv.Atoi(portEntry.Text)
		if err != nil || port <= 0 || port > 65535 {
			log.Printf("[ServerDialog] 无效的端口号: %s", portEntry.Text)
			sd.showMessageWindow("错误", "无效的端口号")
			return
		}

		submitBtn.SetText("测试中...")
		submitBtn.Disable()

		go func() {
			log.Printf("[ServerDialog] 开始测试服务器连通性: %s:%d", hostEntry.Text, port)

			encryptedPassword, err := utils.Encrypt(passwordEntry.Text)
			if err != nil {
				log.Printf("[ServerDialog] 密码加密失败: %v", err)
				fyne.Do(func() {
					sd.showMessageWindow("错误", "密码加密失败")
					submitBtn.SetText("测试连接")
					submitBtn.Enable()
				})
				return
			}

			server := &models.Server{
				Name:     nameEntry.Text,
				Host:     hostEntry.Text,
				SSHPort:  port,
				Username: usernameEntry.Text,
				Password: encryptedPassword,
			}

			connected := utils.TestConnectivity(hostEntry.Text, port)
			if !connected {
				log.Printf("[ServerDialog] 端口连通性测试失败: %s:%d", hostEntry.Text, port)
				fyne.Do(func() {
					sd.showMessageWindow("错误", "无法连接到服务器，请检查端口")
					submitBtn.SetText("测试连接")
					submitBtn.Enable()
				})
				return
			}

			log.Printf("[ServerDialog] 端口连通性测试成功: %s:%d", hostEntry.Text, port)

			serviceType := utils.DetectServiceType(hostEntry.Text, port)
			log.Printf("[ServerDialog] 检测到服务类型: %s", serviceType)

			if serviceType == utils.ServiceSSH {
				log.Printf("[ServerDialog] 开始检测服务器系统类型(SSH)")
				osType, osVersion, err := detector.DetectOS(server)
				if err != nil {
					log.Printf("[ServerDialog] SSH连接失败: %v", err)
					fyne.Do(func() {
						sd.showMessageWindow("错误", "无法连接到服务器，请检查用户名和密码")
						submitBtn.SetText("测试连接")
						submitBtn.Enable()
					})
					return
				}

				log.Printf("[ServerDialog] 系统检测成功: %s %s", osType, osVersion)
				server.OSType = osType
				server.OSVersion = osVersion

				if osType == "linux" {
					log.Printf("[ServerDialog] Linux系统，Docker检测将在防火墙配置时进行")
				}

				kernelVersion, kernelArch, hostname, _, err := detector.DetectSystemInfo(server)
				if err == nil {
					server.KernelVersion = kernelVersion
					server.KernelArch = kernelArch
					server.Hostname = hostname
					log.Printf("[ServerDialog] 系统信息检测成功: 内核版本=%s, 架构=%s, 主机名=%s", kernelVersion, kernelArch, hostname)
				}
			} else if serviceType == utils.ServiceSMB || serviceType == utils.ServiceDCOM {
				log.Printf("[ServerDialog] 开始检测Windows系统类型(%s)", serviceType)
				osType, osVersion, kernelVersion, kernelArch, hostname, err := detector.DetectWindowsOSAndInfo(server)
				if err != nil {
					log.Printf("[ServerDialog] %s连接失败: %v", serviceType, err)
					fyne.Do(func() {
						sd.showMessageWindow("错误", "无法连接到服务器，请检查用户名和密码")
						submitBtn.SetText("测试连接")
						submitBtn.Enable()
					})
					return
				}

				log.Printf("[ServerDialog] 系统检测成功: %s", osType)
				server.OSType = osType
				server.OSVersion = osVersion
				server.KernelVersion = kernelVersion
				server.KernelArch = kernelArch
				server.Hostname = hostname
				log.Printf("[ServerDialog] 系统信息检测成功: 系统版本=%s, 内核版本=%s, 架构=%s, 主机名=%s", osVersion, kernelVersion, kernelArch, hostname)
			} else {
				log.Printf("[ServerDialog] 检测到未知服务类型: %s:%d", hostEntry.Text, port)
				fyne.Do(func() {
					sd.showMessageWindow("错误", "无法识别服务类型")
					submitBtn.SetText("测试连接")
					submitBtn.Enable()
				})
				return
			}

			fyne.Do(func() {
				if err := db.SaveServer(server); err != nil {
					log.Printf("[ServerDialog] 保存服务器失败: %v", err)
					sd.showMessageWindow("错误", "保存服务器失败")
					submitBtn.SetText("测试连接")
					submitBtn.Enable()
					return
				}

				log.Printf("[ServerDialog] 服务器保存成功: %s", server.Name)
				if onSuccess != nil {
					onSuccess()
				}
				sd.showMessageWindow("成功", "服务器添加成功！")
				addWin.Close()
			})
		}()
	})
	submitBtn.Resize(fyne.NewSize(180, InputHeight))

	cancelBtn := widget.NewButton("取消", func() {
		log.Printf("[ServerDialog] 用户点击取消按钮")
		addWin.Close()
	})
	cancelBtn.Resize(fyne.NewSize(180, InputHeight))

	submitBorder := canvas.NewRectangle(ColorBorder)
	submitBorder.CornerRadius = CornerRadius
	submitBorder.SetMinSize(fyne.NewSize(180, InputHeight))
	submitField := container.NewStack(submitBorder, container.NewPadded(submitBtn))

	cancelBorder := canvas.NewRectangle(ColorBorder)
	cancelBorder.CornerRadius = CornerRadius
	cancelBorder.SetMinSize(fyne.NewSize(180, InputHeight))
	cancelField := container.NewStack(cancelBorder, container.NewPadded(cancelBtn))

	formContent := container.NewVBox(
		NewVSpace(SpacingM),
		container.NewHBox(
			NewHSpace(SpacingM),
			nameLabelField,
			NewHSpace(SpacingS),
			nameField,
			NewHSpace(SpacingM),
		),
		NewVSpace(SpacingS),
		container.NewHBox(
			NewHSpace(SpacingM),
			hostLabelField,
			NewHSpace(SpacingS),
			hostField,
			NewHSpace(SpacingM),
		),
		NewVSpace(SpacingS),
		container.NewHBox(
			NewHSpace(SpacingM),
			portLabelField,
			NewHSpace(SpacingS),
			portField,
			NewHSpace(SpacingM),
		),
		NewVSpace(SpacingS),
		container.NewHBox(
			NewHSpace(SpacingM),
			usernameLabelField,
			NewHSpace(SpacingS),
			usernameField,
			NewHSpace(SpacingM),
		),
		NewVSpace(SpacingS),
		container.NewHBox(
			NewHSpace(SpacingM),
			passwordLabelField,
			NewHSpace(SpacingS),
			passwordField,
			NewHSpace(SpacingM),
		),
		NewVSpace(SpacingL),
		container.NewHBox(
			NewHSpace(SpacingM),
			submitField,
			NewHSpace(SpacingM),
			cancelField,
			NewHSpace(SpacingM),
		),
		NewVSpace(SpacingM),
	)

	bg := canvas.NewRectangle(ColorBackground)

	content := container.NewStack(bg, container.NewCenter(formContent))

	addWin.SetContent(content)
	log.Printf("[ServerDialog] 添加窗口设置内容完成，强制固定尺寸")
	addWin.SetFixedSize(true)
	addWin.Resize(fyne.NewSize(420, 400))
	log.Printf("[ServerDialog] 窗口内容尺寸: %v", addWin.Content().Size())
	addWin.CenterOnScreen()
	addWin.Show()
	log.Printf("[ServerDialog] 添加窗口居中并显示完成")
}

func (sd *ServerDialog) ShowEditDialog(server models.Server, onSuccess func()) {
	log.Printf("[ServerDialog] 显示编辑服务器窗口，服务器ID: %d, 名称: %s", server.ID, server.Name)

	if sd.editWin != nil {
		log.Printf("[ServerDialog] 编辑窗口已存在，关闭并重建")
		sd.editWin.Close()
		sd.editWin = nil
	}

	editWin := sd.app.NewWindow("编辑服务器")
	sd.editWin = editWin

	log.Printf("[ServerDialog] 创建编辑窗口，尺寸: 420x400")
	editWin.Resize(fyne.NewSize(420, 400))

	editWin.SetOnClosed(func() {
		log.Printf("[ServerDialog] 编辑窗口关闭")
		sd.editWin = nil
	})

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("服务器名称")
	nameEntry.SetText(server.Name)

	hostEntry := widget.NewEntry()
	hostEntry.SetPlaceHolder("主机地址")
	hostEntry.SetText(server.Host)

	portEntry := widget.NewEntry()
	portEntry.SetPlaceHolder("SSH端口")
	portEntry.SetText(strconv.Itoa(server.SSHPort))

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("用户名")
	usernameEntry.SetText(server.Username)

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("密码（不修改则留空）")

	inputWidth := float32(280)

	nameBorder := canvas.NewRectangle(ColorBorder)
	nameBorder.CornerRadius = CornerRadius
	nameBorder.SetMinSize(fyne.NewSize(inputWidth, InputHeight))
	nameField := container.NewStack(nameBorder, container.NewPadded(nameEntry))

	hostBorder := canvas.NewRectangle(ColorBorder)
	hostBorder.CornerRadius = CornerRadius
	hostBorder.SetMinSize(fyne.NewSize(inputWidth, InputHeight))
	hostField := container.NewStack(hostBorder, container.NewPadded(hostEntry))

	portBorder := canvas.NewRectangle(ColorBorder)
	portBorder.CornerRadius = CornerRadius
	portBorder.SetMinSize(fyne.NewSize(inputWidth, InputHeight))
	portField := container.NewStack(portBorder, container.NewPadded(portEntry))

	usernameBorder := canvas.NewRectangle(ColorBorder)
	usernameBorder.CornerRadius = CornerRadius
	usernameBorder.SetMinSize(fyne.NewSize(inputWidth, InputHeight))
	usernameField := container.NewStack(usernameBorder, container.NewPadded(usernameEntry))

	passwordBorder := canvas.NewRectangle(ColorBorder)
	passwordBorder.CornerRadius = CornerRadius
	passwordBorder.SetMinSize(fyne.NewSize(inputWidth, InputHeight))
	passwordField := container.NewStack(passwordBorder, container.NewPadded(passwordEntry))

	labelWidth := float32(70)

	nameLabel := canvas.NewText("名称 *", ColorText)
	nameLabel.TextSize = 14
	nameLabelBg := canvas.NewRectangle(ColorBackground)
	nameLabelBg.SetMinSize(fyne.NewSize(labelWidth, InputHeight))
	nameLabelField := container.NewStack(nameLabelBg, container.NewCenter(nameLabel))

	hostLabel := canvas.NewText("主机 *", ColorText)
	hostLabel.TextSize = 14
	hostLabelBg := canvas.NewRectangle(ColorBackground)
	hostLabelBg.SetMinSize(fyne.NewSize(labelWidth, InputHeight))
	hostLabelField := container.NewStack(hostLabelBg, container.NewCenter(hostLabel))

	portLabel := canvas.NewText("远程端口 *", ColorText)
	portLabel.TextSize = 14
	portLabelBg := canvas.NewRectangle(ColorBackground)
	portLabelBg.SetMinSize(fyne.NewSize(labelWidth, InputHeight))
	portLabelField := container.NewStack(portLabelBg, container.NewCenter(portLabel))

	usernameLabel := canvas.NewText("用户名 *", ColorText)
	usernameLabel.TextSize = 14
	usernameLabelBg := canvas.NewRectangle(ColorBackground)
	usernameLabelBg.SetMinSize(fyne.NewSize(labelWidth, InputHeight))
	usernameLabelField := container.NewStack(usernameLabelBg, container.NewCenter(usernameLabel))

	passwordLabel := canvas.NewText("密码", ColorText)
	passwordLabel.TextSize = 14
	passwordLabelBg := canvas.NewRectangle(ColorBackground)
	passwordLabelBg.SetMinSize(fyne.NewSize(labelWidth, InputHeight))
	passwordLabelField := container.NewStack(passwordLabelBg, container.NewCenter(passwordLabel))

	var submitBtn *widget.Button
	submitBtn = widget.NewButton("测试连接", func() {
		log.Printf("[ServerDialog] 用户点击测试连接按钮")

		if nameEntry.Text == "" {
			log.Printf("[ServerDialog] 服务器名称为空")
			sd.showMessageWindow("错误", "请填写服务器名称")
			return
		}

		if hostEntry.Text == "" {
			log.Printf("[ServerDialog] 主机地址为空")
			sd.showMessageWindow("错误", "请填写主机地址")
			return
		}

		if usernameEntry.Text == "" {
			log.Printf("[ServerDialog] 用户名为空")
			sd.showMessageWindow("错误", "请填写用户名")
			return
		}

		port, err := strconv.Atoi(portEntry.Text)
		if err != nil || port <= 0 || port > 65535 {
			log.Printf("[ServerDialog] 无效的端口号: %s", portEntry.Text)
			sd.showMessageWindow("错误", "无效的端口号")
			return
		}

		submitBtn.SetText("测试中...")
		submitBtn.Disable()

		go func() {
			log.Printf("[ServerDialog] 开始测试服务器连通性: %s:%d", hostEntry.Text, port)

			server.Name = nameEntry.Text
			server.Host = hostEntry.Text
			server.SSHPort = port
			server.Username = usernameEntry.Text

			if passwordEntry.Text != "" {
				log.Printf("[ServerDialog] 用户修改了密码，开始加密")
				encryptedPassword, err := utils.Encrypt(passwordEntry.Text)
				if err != nil {
					log.Printf("[ServerDialog] 密码加密失败: %v", err)
					fyne.Do(func() {
						sd.showMessageWindow("错误", "密码加密失败")
						submitBtn.SetText("测试连接")
						submitBtn.Enable()
					})
					return
				}
				server.Password = encryptedPassword
				log.Printf("[ServerDialog] 密码加密成功")
			}

			connected := utils.TestConnectivity(hostEntry.Text, port)
			if !connected {
				log.Printf("[ServerDialog] 端口连通性测试失败: %s:%d", hostEntry.Text, port)
				fyne.Do(func() {
					sd.showMessageWindow("错误", "无法连接到服务器，请检查端口")
					submitBtn.SetText("测试连接")
					submitBtn.Enable()
				})
				return
			}

			log.Printf("[ServerDialog] 端口连通性测试成功: %s:%d", hostEntry.Text, port)

			serviceType := utils.DetectServiceType(hostEntry.Text, port)
			log.Printf("[ServerDialog] 检测到服务类型: %s", serviceType)

			if serviceType == utils.ServiceSSH {
				log.Printf("[ServerDialog] 开始检测服务器系统类型(SSH)")
				osType, osVersion, err := detector.DetectOS(&server)
				if err != nil {
					log.Printf("[ServerDialog] SSH连接失败: %v", err)
					fyne.Do(func() {
						sd.showMessageWindow("错误", "无法连接到服务器，请检查用户名和密码")
						submitBtn.SetText("测试连接")
						submitBtn.Enable()
					})
					return
				}

				log.Printf("[ServerDialog] 系统检测成功: %s %s", osType, osVersion)
				server.OSType = osType
				server.OSVersion = osVersion

				if osType == "linux" {
					log.Printf("[ServerDialog] Linux系统，Docker检测将在防火墙配置时进行")
				}

				kernelVersion, kernelArch, hostname, _, _ := detector.DetectSystemInfo(&server)
				server.KernelVersion = kernelVersion
				server.KernelArch = kernelArch
				server.Hostname = hostname
				log.Printf("[ServerDialog] 系统信息检测成功: 内核版本=%s, 架构=%s, 主机名=%s", kernelVersion, kernelArch, hostname)
			} else if serviceType == utils.ServiceSMB {
				log.Printf("[ServerDialog] 开始检测Windows系统类型(SMB)")
				osType, osVersion, kernelVersion, kernelArch, hostname, err := detector.DetectWindowsOSAndInfo(&server)
				if err != nil {
					log.Printf("[ServerDialog] SMB连接失败: %v", err)
					fyne.Do(func() {
						sd.showMessageWindow("错误", "无法连接到服务器，请检查用户名和密码")
						submitBtn.SetText("测试连接")
						submitBtn.Enable()
					})
					return
				}

				log.Printf("[ServerDialog] 系统检测成功: %s %s", osType, osVersion)
				server.OSType = osType
				server.OSVersion = osVersion
				server.KernelVersion = kernelVersion
				server.KernelArch = kernelArch
				server.Hostname = hostname
				log.Printf("[ServerDialog] 系统信息检测成功: 内核版本=%s, 架构=%s, 主机名=%s", kernelVersion, kernelArch, hostname)
			} else {
				log.Printf("[ServerDialog] 检测到未知服务类型: %s:%d", hostEntry.Text, port)
				fyne.Do(func() {
					sd.showMessageWindow("错误", "无法识别服务类型")
					submitBtn.SetText("测试连接")
					submitBtn.Enable()
				})
				return
			}

			fyne.Do(func() {
				if err := db.SaveServer(&server); err != nil {
					log.Printf("[ServerDialog] 保存服务器失败: %v", err)
					sd.showMessageWindow("错误", "保存服务器失败")
					submitBtn.SetText("测试连接")
					submitBtn.Enable()
					return
				}

				log.Printf("[ServerDialog] 服务器更新成功: %s", server.Name)
				if onSuccess != nil {
					onSuccess()
				}
				sd.showMessageWindow("成功", "服务器更新成功！")
				editWin.Close()
			})
		}()
	})
	submitBtn.Resize(fyne.NewSize(180, InputHeight))

	cancelBtn := widget.NewButton("取消", func() {
		log.Printf("[ServerDialog] 用户点击取消按钮")
		editWin.Close()
	})
	cancelBtn.Resize(fyne.NewSize(180, InputHeight))

	submitBorder := canvas.NewRectangle(ColorBorder)
	submitBorder.CornerRadius = CornerRadius
	submitBorder.SetMinSize(fyne.NewSize(180, InputHeight))
	submitField := container.NewStack(submitBorder, container.NewPadded(submitBtn))

	cancelBorder := canvas.NewRectangle(ColorBorder)
	cancelBorder.CornerRadius = CornerRadius
	cancelBorder.SetMinSize(fyne.NewSize(180, InputHeight))
	cancelField := container.NewStack(cancelBorder, container.NewPadded(cancelBtn))

	formContent := container.NewVBox(
		NewVSpace(SpacingM),
		container.NewHBox(
			NewHSpace(SpacingM),
			nameLabelField,
			NewHSpace(SpacingS),
			nameField,
			NewHSpace(SpacingM),
		),
		NewVSpace(SpacingS),
		container.NewHBox(
			NewHSpace(SpacingM),
			hostLabelField,
			NewHSpace(SpacingS),
			hostField,
			NewHSpace(SpacingM),
		),
		NewVSpace(SpacingS),
		container.NewHBox(
			NewHSpace(SpacingM),
			portLabelField,
			NewHSpace(SpacingS),
			portField,
			NewHSpace(SpacingM),
		),
		NewVSpace(SpacingS),
		container.NewHBox(
			NewHSpace(SpacingM),
			usernameLabelField,
			NewHSpace(SpacingS),
			usernameField,
			NewHSpace(SpacingM),
		),
		NewVSpace(SpacingS),
		container.NewHBox(
			NewHSpace(SpacingM),
			passwordLabelField,
			NewHSpace(SpacingS),
			passwordField,
			NewHSpace(SpacingM),
		),
		NewVSpace(SpacingL),
		container.NewHBox(
			NewHSpace(SpacingM),
			submitField,
			NewHSpace(SpacingM),
			cancelField,
			NewHSpace(SpacingM),
		),
		NewVSpace(SpacingM),
	)

	bg := canvas.NewRectangle(ColorBackground)

	content := container.NewStack(bg, container.NewCenter(formContent))

	editWin.SetContent(content)
	log.Printf("[ServerDialog] 编辑窗口设置内容完成，强制固定尺寸")
	editWin.SetFixedSize(true)
	editWin.Resize(fyne.NewSize(420, 400))
	log.Printf("[ServerDialog] 窗口内容尺寸: %v", editWin.Content().Size())
	editWin.CenterOnScreen()
	editWin.Show()
	log.Printf("[ServerDialog] 编辑窗口居中并显示完成")
}

func (sd *ServerDialog) centerWindow(win fyne.Window, width, height float32) {
	win.CenterOnScreen()
	win.Show()
	centerWindowRelativeTo(sd.parentTitle, win.Title(), int32(width), int32(height))
}
