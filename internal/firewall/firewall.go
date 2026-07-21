package firewall

import (
	"access-control-tool/internal/models"
	"access-control-tool/internal/smb"
	"access-control-tool/internal/ssh"
	"fmt"
	"log"
)

// ConfigResult 防火墙配置结果结构体
// 字段说明:
//   Success - 配置是否成功
//   Command - 执行的完整命令
//   Output - 命令执行输出
type ConfigResult struct {
	Success bool   // 配置是否成功
	Command string // 执行的完整命令
	Output  string // 命令执行输出
}

// WinRMClient Windows远程管理客户端接口
// 用于执行Windows服务器上的PowerShell命令
type WinRMClient interface {
	Execute(command string) (string, error) // 执行命令并返回输出
	Close()                                 // 关闭连接
}

// SSHClient SSH客户端接口
// 用于执行Linux服务器上的命令
type SSHClient interface {
	Execute(command string) (string, error) // 执行命令并返回输出
	Close()                                 // 关闭连接
}

// WriteFunc 进度回调函数类型
// 用于向UI报告配置进度
type WriteFunc func(message string)

// Deconfigure 取消配置防火墙规则
// 参数:
//   server - 服务器信息
//   port - 要取消配置的端口
//   ipWhitelist - IP白名单（空表示完整取消）
//   progressCallback - 可选的进度回调函数
// 返回值:
//   error - 操作过程中的错误
// 功能:
//   - 根据服务器操作系统类型选择不同的取消配置方式
//   - Windows系统使用SMB客户端执行PowerShell命令
//   - Linux系统使用SSH客户端执行iptables命令
func Deconfigure(server *models.Server, port int, ipWhitelist string, progressCallback ...func(message string)) error {
	log.Printf("[Firewall] 开始取消配置防火墙，服务器: %s:%d, 操作系统: %s, 端口: %d, IP白名单: '%s'",
		server.Host, server.SSHPort, server.OSType, port, ipWhitelist)

	var callback func(message string)
	if len(progressCallback) > 0 {
		callback = progressCallback[0]
	}

	if server.OSType == "windows" {
		log.Printf("[Firewall] 创建SMB客户端")
		client, err := smb.NewClient(server)
		if err != nil {
			log.Printf("[Firewall] 创建SMB客户端失败: %v", err)
			if callback != nil {
				callback(fmt.Sprintf("❌ 创建SMB客户端失败: %v\n", err))
			}
			return fmt.Errorf("创建SMB客户端失败: %v", err)
		}
		defer client.Close()

		log.Printf("[Firewall] SMB客户端创建成功")

		if err := deconfigureWindows(client, port, ipWhitelist, callback); err != nil {
			log.Printf("[Firewall] 取消配置失败: %v", err)
			return err
		}

		log.Printf("[Firewall] 取消配置完成")
		return nil
	} else {
		log.Printf("[Firewall] 创建SSH客户端")
		client, err := ssh.NewClient(server)
		if err != nil {
			log.Printf("[Firewall] 创建SSH客户端失败: %v", err)
			if callback != nil {
				callback(fmt.Sprintf("❌ 创建SSH客户端失败: %v\n", err))
			}
			return fmt.Errorf("创建SSH客户端失败: %v", err)
		}
		defer client.Close()

		log.Printf("[Firewall] SSH客户端创建成功")

		distroInfo, err := detectDistro(client)
		if err != nil {
			log.Printf("[Firewall] 发行版检测失败: %v", err)
			if callback != nil {
				callback(fmt.Sprintf("❌ 发行版检测失败: %v\n", err))
			}
			return fmt.Errorf("发行版检测失败: %v", err)
		}
		if err := deconfigureLinux(client, port, ipWhitelist, distroInfo.IptablesSaveCommand, callback); err != nil {
			log.Printf("[Firewall] 取消配置失败: %v", err)
			return err
		}

		log.Printf("[Firewall] 取消配置完成")
		if callback != nil {
			callback("✅ 取消配置完成\n")
		}
		return nil
	}
}

// Configure 配置防火墙规则
// 参数:
//   server - 服务器信息
//   port - 要配置的端口
//   ipWhitelist - IP白名单（逗号分隔，空或"all"表示允许所有IP）
//   progressCallback - 可选的进度回调函数
// 返回值:
//   *ConfigResult - 配置结果
//   error - 操作过程中的错误
// 功能:
//   - 根据服务器操作系统类型选择不同的配置方式
//   - Windows系统使用SMB客户端执行PowerShell命令配置Windows防火墙
//   - Linux系统使用SSH客户端，自动检测发行版并配置iptables规则
//     * 支持发行版：Ubuntu、Kali、Debian、CentOS、RHEL、Rocky、Fedora、SUSE
//     * 自动检测包管理器：apt/yum/dnf/zypper
//     * 自动停止firewalld/ufw并切换到iptables
//     * Docker环境自动使用DOCKER-USER链
func Configure(server *models.Server, port int, ipWhitelist string, progressCallback ...func(message string)) (*ConfigResult, error) {
	log.Printf("[Firewall] 开始配置防火墙，服务器: %s:%d, 操作系统: %s, 端口: %d, IP白名单: '%s'",
		server.Host, server.SSHPort, server.OSType, port, ipWhitelist)

	var callback func(message string)
	if len(progressCallback) > 0 {
		callback = progressCallback[0]
	}

	if server.OSType == "windows" {
		log.Printf("[Firewall] 创建SMB客户端")
		client, err := smb.NewClient(server)
		if err != nil {
			log.Printf("[Firewall] 创建SMB客户端失败: %v", err)
			return nil, err
		}
		defer client.Close()
		return configureWindows(client, server, port, ipWhitelist, callback)
	}

	log.Printf("[Firewall] 创建SSH客户端")
	client, err := ssh.NewClient(server)
	if err != nil {
		log.Printf("[Firewall] SSH客户端创建失败: %v", err)
		return nil, err
	}
	defer client.Close()
	return configureLinux(client, server, port, ipWhitelist, callback)
}