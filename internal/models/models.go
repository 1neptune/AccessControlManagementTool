package models

import "time"

// Server 表示服务器模型，包含SSH连接凭证和操作系统信息
type Server struct {
	ID            uint      `json:"id"`             // 服务器唯一标识
	Name          string    `json:"name"`           // 服务器名称（用于显示）
	Host          string    `json:"host"`           // 服务器主机地址（IP或域名）
	SSHPort       int       `json:"ssh_port"`       // SSH端口，默认22
	Username      string    `json:"username"`       // SSH用户名
	Password      string    `json:"password"`       // SSH密码（加密存储）
	KeyPath       string    `json:"key_path"`       // SSH私钥路径（可选）
	OSType        string    `json:"os_type"`        // 操作系统类型：windows/linux
	OSVersion     string    `json:"os_version"`     // 操作系统版本
	KernelVersion string    `json:"kernel_version"` // 内核版本
	KernelArch    string    `json:"kernel_arch"`    // 内核架构
	Hostname      string    `json:"hostname"`       // 主机名
	IsDocker      bool      `json:"is_docker"`      // 是否为Docker容器环境
	CreatedAt     time.Time `json:"created_at"`     // 创建时间
	UpdatedAt     time.Time `json:"updated_at"`     // 更新时间
}

// ConfigHistory 表示配置历史记录模型
type ConfigHistory struct {
	ID          uint      `json:"id"`           // 记录唯一标识
	ServerID    uint      `json:"server_id"`    // 关联服务器ID
	Port        int       `json:"port"`         // 配置的端口
	IPWhitelist string    `json:"ip_whitelist"` // IP白名单（逗号分隔）
	Chain       string    `json:"chain"`        // 使用的防火墙链（INPUT/DOCKER-USER）
	Command     string    `json:"command"`      // 执行的命令
	Output      string    `json:"output"`       // 命令输出
	Success     bool      `json:"success"`      // 配置是否成功
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
}
