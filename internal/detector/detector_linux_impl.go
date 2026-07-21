package detector

import (
	"access-control-tool/internal/models"
	"access-control-tool/internal/ssh"
	"fmt"
	"log"
	"strings"
)

// DetectOS 检测Linux服务器的操作系统类型和版本
// 参数:
//   server - 服务器信息
// 返回值:
//   string - 操作系统类型（linux）
//   string - 操作系统版本
//   error - 检测过程中的错误
// 注意:
//   此函数仅用于Linux服务器，不检测Windows系统
func DetectOS(server *models.Server) (string, string, error) {
	log.Printf("[Detector] 开始检测Linux操作系统类型，服务器: %s:%d", server.Host, server.SSHPort)

	client, err := ssh.NewClient(server)
	if err != nil {
		log.Printf("[Detector] SSH客户端创建失败: %v", err)
		return "", "", err
	}
	defer client.Close()

	log.Printf("[Detector] SSH客户端创建成功，读取发行版信息")
	output, err := client.Execute("cat /etc/os-release 2>/dev/null || cat /etc/redhat-release 2>/dev/null || uname -a")
	if err != nil {
		log.Printf("[Detector] 读取发行版信息失败: %v", err)
		return "", "", err
	}

	if len(output) == 0 {
		log.Printf("[Detector] 无法获取发行版信息")
		return "", "", nil
	}

	osType := "linux"
	osVersion := parseLinuxVersion(output)

	log.Printf("[Detector] 操作系统检测成功: %s %s", osType, osVersion)
	return osType, osVersion, nil
}

// parseLinuxVersion 解析Linux版本信息
// 参数:
//   output - os-release文件或uname命令输出
// 返回值:
//   string - 格式化的版本字符串
func parseLinuxVersion(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "NAME=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.Trim(parts[1], "\"'")
				for _, vLine := range lines {
					if strings.HasPrefix(vLine, "VERSION_ID=") {
						vParts := strings.SplitN(vLine, "=", 2)
						if len(vParts) == 2 {
							version := strings.Trim(vParts[1], "\"'")
							return fmt.Sprintf("%s %s", name, version)
						}
					}
				}
				return name
			}
		}
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.Trim(parts[1], "\"'")
				if idx := strings.Index(name, "("); idx > 0 {
					name = strings.TrimSpace(name[:idx])
				}
				return name
			}
		}
		if strings.HasPrefix(line, "VERSION=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.Trim(parts[1], "\"'")
			}
		}
	}
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return "Unknown Linux"
}

// DetectSystemInfoLinux 检测Linux系统详细信息
// 参数:
//   server - 服务器信息
// 返回值:
//   string - 内核版本
//   string - 内核架构
//   string - 主机名
//   string - 发行版名称（空，保留兼容）
//   error - 检测过程中的错误
func DetectSystemInfoLinux(server *models.Server) (string, string, string, string, error) {
	log.Printf("[Detector] 检测Linux系统信息，服务器: %s:%d", server.Host, server.SSHPort)

	client, err := ssh.NewClient(server)
	if err != nil {
		log.Printf("[Detector] SSH客户端创建失败: %v", err)
		return "", "", "", "", err
	}
	defer client.Close()

	kernelVersion, _ := client.Execute("uname -r")
	kernelArch, _ := client.Execute("uname -m")
	hostname, _ := client.Execute("hostname")

	kernelVersion = strings.TrimSpace(kernelVersion)
	kernelArch = strings.TrimSpace(kernelArch)
	hostname = strings.TrimSpace(hostname)

	if idx := strings.LastIndex(kernelVersion, "."); idx > 0 {
		archSuffix := kernelVersion[idx+1:]
		if archSuffix == "x86_64" || archSuffix == "i686" || archSuffix == "aarch64" || archSuffix == "armv7l" {
			kernelVersion = kernelVersion[:idx]
		}
	}

	log.Printf("[Detector] Linux系统信息检测完成 - 内核版本: %s, 架构: %s, 主机名: %s", kernelVersion, kernelArch, hostname)
	return kernelVersion, kernelArch, hostname, "", nil
}