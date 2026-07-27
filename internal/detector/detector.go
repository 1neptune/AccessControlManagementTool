package detector

import (
	"access-control-tool/internal/models"
	"log"
)

// DetectSystemInfo 检测服务器系统详细信息的统一入口函数
// 根据服务器的操作系统类型分发到对应的检测实现
// 参数:
//   server - 服务器信息，必须包含OSType字段（"linux"或"windows"）
// 返回值:
//   string - 内核版本（如 "3.10.0-1160.el7"）
//   string - 系统架构（如 "x86_64"、"arm64"）
//   string - 主机名（服务器的主机名称）
//   string - 操作系统名称（当前版本返回空字符串，保留兼容）
//   error - 检测过程中的错误，成功返回nil
// 分发逻辑:
//   - Linux系统: 调用DetectSystemInfoLinux，使用SSH连接执行uname命令
//   - Windows系统: 调用DetectSystemInfoWindows，使用SMB/WMI执行ver、hostname等命令
//   - 其他系统: 返回空值，不报错，记录警告日志
// 注意点:
//   - server.OSType字段必须正确设置，否则无法正确检测
//   - 检测过程会建立远程连接（SSH或SMB），需要网络连通
//   - 调用方负责处理连接失败的情况
func DetectSystemInfo(server *models.Server) (string, string, string, string, error) {
	log.Printf("[Detector] 开始检测系统详细信息，服务器: %s:%d, 操作系统: %s", server.Host, server.SSHPort, server.OSType)

	if server.OSType == "linux" {
		return DetectSystemInfoLinux(server)
	} else if server.OSType == "windows" {
		return DetectSystemInfoWindows(server)
	}

	log.Printf("[Detector] 无法识别的操作系统类型: %s", server.OSType)
	return "", "", "", "", nil
}
