package service

import (
	"access-control-tool/internal/db"
	"access-control-tool/internal/detector"
	"access-control-tool/internal/firewall"
	"access-control-tool/internal/models"
	"access-control-tool/internal/portscanner"
	"log"
)

// ServerService 服务器服务接口
// 定义服务器管理的核心业务操作
type ServerService interface {
	ListServers() ([]models.Server, error)
	GetServerByID(id uint) (*models.Server, error)
	SaveServer(server *models.Server) error
	DeleteServer(id uint) error
	DetectServerOS(server *models.Server) error
	ScanServerPorts(server *models.Server) ([]portscanner.PortInfo, error)
	ConfigureFirewall(server *models.Server, port int, ipWhitelist string) (*firewall.ConfigResult, error)
}

// serverServiceImpl 服务器服务实现
// 作为业务逻辑层，协调各底层模块完成服务器管理操作
type serverServiceImpl struct {
}

// NewServerService 创建服务器服务实例
func NewServerService() ServerService {
	return &serverServiceImpl{}
}

// ListServers 获取所有服务器列表
// 调用数据库层获取服务器记录
func (s *serverServiceImpl) ListServers() ([]models.Server, error) {
	return db.ListServers()
}

// GetServerByID 根据ID获取服务器
// 参数: id - 服务器ID
// 返回: 服务器信息和错误
func (s *serverServiceImpl) GetServerByID(id uint) (*models.Server, error) {
	return db.GetServerByID(id)
}

// SaveServer 保存服务器信息
// 调用数据库层保存或更新服务器记录
// 参数: server - 服务器信息
func (s *serverServiceImpl) SaveServer(server *models.Server) error {
	return db.SaveServer(server)
}

// DeleteServer 删除服务器
// 调用数据库层删除指定ID的服务器记录
// 参数: id - 服务器ID
func (s *serverServiceImpl) DeleteServer(id uint) error {
	return db.DeleteServer(id)
}

// DetectServerOS 检测服务器操作系统
// 流程:
// 1. 调用detector.DetectOS获取操作系统类型和版本
// 2. 如果是Linux系统，额外检测是否为Docker容器
// 参数: server - 服务器信息（检测后会更新OSType、OSVersion、IsDocker字段）
func (s *serverServiceImpl) DetectServerOS(server *models.Server) error {
	osType, osVersion, err := detector.DetectOS(server)
	if err != nil {
		return err
	}
	server.OSType = osType
	server.OSVersion = osVersion

	if osType == "linux" {
		log.Printf("[Service] Linux系统，Docker检测将在防火墙配置时进行")
	}

	return nil
}

// ScanServerPorts 扫描服务器监听端口
// 调用portscanner模块进行端口扫描
// 参数: server - 服务器信息
// 返回: 端口信息列表和错误
func (s *serverServiceImpl) ScanServerPorts(server *models.Server) ([]portscanner.PortInfo, error) {
	return portscanner.ScanPorts(server)
}

// ConfigureFirewall 配置防火墙规则
// 调用firewall模块配置IP安全策略
// 参数: server - 服务器信息
//       port - 目标端口
//       ipWhitelist - 白名单IP列表（逗号分隔）
// 返回: 配置结果和错误
func (s *serverServiceImpl) ConfigureFirewall(server *models.Server, port int, ipWhitelist string) (*firewall.ConfigResult, error) {
	return firewall.Configure(server, port, ipWhitelist)
}