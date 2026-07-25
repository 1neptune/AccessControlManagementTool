package detector

import (
	"access-control-tool/internal/models"
	"access-control-tool/internal/ssh"
	"fmt"
	"log"
	"strings"
)

// DetectOS 检测Linux服务器的操作系统类型和版本
// 通过SSH连接执行命令读取发行版信息文件，识别Linux发行版
// 参数:
//   server - 服务器信息，包含主机地址、端口、用户名、密码等
// 返回值:
//   string - 操作系统类型（固定返回"linux"）
//   string - 操作系统版本（如 "CentOS Linux 7"、"Ubuntu 22.04"）
//   error - 检测过程中的错误，连接失败或命令执行失败时返回
// 执行流程:
//   1. 创建SSH客户端并建立连接
//   2. 执行命令读取发行版信息，按优先级尝试:
//      - cat /etc/os-release（标准Linux发行版）
//      - cat /etc/redhat-release（CentOS/RHEL旧版本）
//      - uname -a（通用回退）
//   3. 解析命令输出，提取发行版名称和版本
//   4. 返回检测结果
// 注意点:
//   - 此函数仅用于Linux服务器，不检测Windows系统
//   - 使用defer client.Close()确保SSH连接在函数结束时关闭
//   - 命令执行失败时返回空版本和错误
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
// 从命令输出中提取发行版名称和版本号，支持多种格式
// 参数:
//   output - /etc/os-release文件、/etc/redhat-release文件或uname命令的输出
// 返回值:
//   string - 格式化的版本字符串（如 "CentOS Linux 7"、"Ubuntu 22.04"）
// 解析优先级:
//   1. NAME= + VERSION_ID= 组合（标准格式）
//   2. PRETTY_NAME=（完整名称）
//   3. VERSION=（版本字段）
//   4. 首行文本（回退）
//   5. "Unknown Linux"（默认）
// 注意点:
//   - 自动处理引号（单引号和双引号）
//   - 去除PRETTY_NAME中括号内的内容
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
// 通过SSH连接执行uname命令获取内核版本、架构和主机名
// 参数:
//   server - 服务器信息，包含主机地址、端口、用户名、密码等
// 返回值:
//   string - 内核版本（如 "3.10.0-1160.el7"）
//   string - 内核架构（如 "x86_64"、"aarch64"）
//   string - 主机名（服务器的主机名称）
//   string - 发行版名称（当前版本返回空字符串，保留兼容）
//   error - 检测过程中的错误，连接失败时返回
// 执行流程:
//   1. 创建SSH客户端并建立连接
//   2. 执行 uname -r 获取内核版本
//   3. 执行 uname -m 获取内核架构
//   4. 执行 hostname 获取主机名
//   5. 清理输出（去除首尾空白）
//   6. 特殊处理：如果内核版本末尾包含架构后缀，去除后缀
//   7. 返回检测结果
// 注意点:
//   - 使用defer client.Close()确保SSH连接在函数结束时关闭
//   - 命令执行失败时不返回错误，而是使用空字符串（容错设计）
//   - 内核版本清理：去除末尾的架构后缀（如 .x86_64）
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