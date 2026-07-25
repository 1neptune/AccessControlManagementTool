package models

import "time"

// Server 服务器模型，用于存储服务器连接凭证、操作系统信息及检测结果
// 该模型是整个系统的核心数据结构，所有服务器管理、系统检测、端口扫描、
// 防火墙配置操作均围绕此模型展开
// 注意点:
//   - Password字段存储的是加密后的密码（AES-GCM算法），使用前需解密
//   - Host字段在数据库中具有UNIQUE约束，同一主机地址只能添加一次
//   - OSType字段用于区分Linux和Windows系统，决定后续操作的执行方式
//   - SSHPort字段对Linux系统表示SSH端口(默认22)，对Windows系统表示SMB端口(默认445)
type Server struct {
	ID            uint      `json:"id"`             // 服务器唯一标识，自增主键
	Name          string    `json:"name"`           // 服务器名称，用于列表显示，可自定义
	Host          string    `json:"host"`           // 服务器主机地址，支持IP地址或域名
	SSHPort       int       `json:"ssh_port"`       // 远程连接端口，Linux默认22(SSH)，Windows默认445(SMB)
	Username      string    `json:"username"`       // 远程登录用户名
	Password      string    `json:"password"`       // 密码，经过AES-GCM加密存储，避免明文泄露
	KeyPath       string    `json:"key_path"`       // SSH私钥文件路径，可选，优先于密码认证
	OSType        string    `json:"os_type"`        // 操作系统类型，取值为"linux"或"windows"
	OSVersion     string    `json:"os_version"`     // 操作系统版本，如"CentOS Linux release 7.9.2009"
	KernelVersion string    `json:"kernel_version"` // 内核版本，如"3.10.0-1160.el7.x86_64"
	KernelArch    string    `json:"kernel_arch"`    // 内核架构，如"x86_64"、"arm64"
	Hostname      string    `json:"hostname"`       // 服务器主机名
	IsDocker      bool      `json:"is_docker"`      // 是否为Docker容器环境，影响防火墙链选择
	CreatedAt     time.Time `json:"created_at"`     // 创建时间，自动生成
	UpdatedAt     time.Time `json:"updated_at"`     // 更新时间，每次修改自动更新
}

// ConfigHistory 配置历史记录模型，用于记录每次防火墙配置操作的详细信息
// 该模型用于追溯配置历史，支持故障排查和操作审计
// 注意点:
//   - 通过ServerID字段与Server模型建立关联
//   - Command字段存储执行的完整命令，用于问题复现
//   - Output字段存储命令执行输出，可能包含敏感信息
//   - Chain字段用于区分不同场景的防火墙规则（INPUT链用于普通服务器，DOCKER-USER链用于Docker环境）
type ConfigHistory struct {
	ID          uint      `json:"id"`           // 记录唯一标识，自增主键
	ServerID    uint      `json:"server_id"`    // 关联的服务器ID，外键引用Server.ID
	Port        int       `json:"port"`         // 配置的目标端口
	IPWhitelist string    `json:"ip_whitelist"` // IP白名单，逗号分隔的IP地址列表，"all"表示允许所有IP
	Chain       string    `json:"chain"`        // 使用的防火墙链，取值为"INPUT"或"DOCKER-USER"
	Command     string    `json:"command"`      // 执行的完整命令字符串
	Output      string    `json:"output"`       // 命令执行的标准输出和错误输出
	Success     bool      `json:"success"`      // 配置是否成功，true表示成功，false表示失败
	CreatedAt   time.Time `json:"created_at"`   // 记录创建时间，即配置操作执行时间
}
