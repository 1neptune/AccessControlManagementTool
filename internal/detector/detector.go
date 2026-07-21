package detector

import (
	"access-control-tool/internal/models"
	"log"
)

// DetectSystemInfo 检测服务器系统详细信息
// 参数:
//   server - 服务器信息，包含主机地址、端口、操作系统类型等
// 返回值:
//   string - 内核版本（kernel version）
//   string - 系统架构（architecture）
//   string - 主机名（hostname）
//   string - 操作系统名称（OS name）
//   error - 检测过程中的错误
// 分发逻辑:
//   - Linux系统: 调用DetectSystemInfoLinux
//   - Windows系统: 调用DetectSystemInfoWindows
//   - 其他系统: 返回空值，不报错
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
