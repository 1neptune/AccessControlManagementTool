package portscanner

import (
	"access-control-tool/internal/models"
	"access-control-tool/internal/smb"
	"access-control-tool/internal/ssh"
	"log"
)

// PortInfo 端口信息结构体
// 包含端口的基本信息和连接IP列表
// 字段说明:
//   Port - 端口号
//   ServiceName - 服务名称（如sshd, nginx等）
//   ConnectedIPs - 已连接的客户端IP列表
type PortInfo struct {
	Port         int      `json:"port"`
	ServiceName  string   `json:"service_name"`
	ConnectedIPs []string `json:"connected_ips"`
}

// ScanPorts 扫描服务器监听端口
// 参数:
//   server - 服务器信息，包含主机地址、端口、操作系统类型等
// 返回值:
//   []PortInfo - 端口信息列表
//   error - 扫描过程中的错误
// 分发逻辑:
//   - Windows系统: 使用SMB客户端连接，调用scanWindowsPorts
//   - Linux系统: 使用SSH客户端连接，调用scanLinuxPorts
// 资源管理:
//   - 使用defer确保客户端连接在函数结束时关闭
func ScanPorts(server *models.Server) ([]PortInfo, error) {
	log.Printf("[PortScanner] 开始扫描服务器端口，服务器: %s:%d, 操作系统: %s", server.Host, server.SSHPort, server.OSType)

	if server.OSType == "windows" {
		log.Printf("[PortScanner] 创建SMB客户端")
		client, err := smb.NewClient(server)
		if err != nil {
			log.Printf("[PortScanner] SMB客户端创建失败: %v", err)
			return nil, err
		}
		defer client.Close()
		return scanWindowsPorts(client)
	}

	log.Printf("[PortScanner] 创建SSH客户端")
	client, err := ssh.NewClient(server)
	if err != nil {
		log.Printf("[PortScanner] SSH客户端创建失败: %v", err)
		return nil, err
	}
	defer client.Close()
	return scanLinuxPorts(client)
}

// contains 判断字符串切片中是否包含指定元素
// 参数:
//   slice - 字符串切片
//   item - 要查找的元素
// 返回值:
//   bool - 是否包含
// 使用场景:
//   - 去重已连接IP列表
//   - 检查IP是否在白名单中
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
