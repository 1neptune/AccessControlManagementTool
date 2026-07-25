package db

import (
	"access-control-tool/internal/models"
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

// 全局数据库实例，用于整个应用的数据库操作
var (
	dbInstance *sql.DB  // SQLite数据库连接实例，全局共享
	dbPath     string   // 数据库文件路径，默认为"data.db"
)

// InitDB 初始化SQLite数据库连接并创建必要的表
// 该函数是数据库操作的入口，必须在应用启动时调用
// 返回值:
//   error - 初始化过程中的错误信息，成功返回nil
// 初始化流程:
//   1. 设置数据库文件路径为"data.db"（当前工作目录下）
//   2. 使用modernc.org/sqlite驱动打开数据库连接
//   3. 执行Ping验证连接可用性
//   4. 创建servers和config_history表（如果不存在）
//   5. 执行数据库迁移（添加缺失的列）
// 注意点:
//   - 使用modernc.org/sqlite纯Go实现，无需CGO依赖，可跨平台编译
//   - 数据库文件存储在程序运行的当前目录
//   - 如果数据库文件不存在，会自动创建
//   - 必须在所有数据库操作前调用此函数
func InitDB() error {
	dbPath = "data.db"

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %v", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("数据库连接失败: %v", err)
	}

	dbInstance = db

	if err := createTables(); err != nil {
		return fmt.Errorf("创建表失败: %v", err)
	}

	if err := migrateTables(); err != nil {
		return fmt.Errorf("数据库迁移失败: %v", err)
	}

	log.Printf("数据库初始化成功: %s", dbPath)
	return nil
}

// createTables 创建服务器表和配置历史表
// 该函数在InitDB中调用，确保必要的表结构存在
// 返回值:
//   error - 创建过程中的错误信息，成功返回nil
// 创建的表:
//   1. servers表 - 存储服务器基本信息
//      - id: 主键，自增，唯一标识
//      - name: 服务器名称，必填
//      - host: 主机地址（IP或域名），必填，唯一约束（UNIQUE）
//      - ssh_port: SSH端口，默认22
//      - username: 登录用户名，必填
//      - password: 加密后的密码，必填（AES-GCM加密）
//      - key_path: SSH私钥路径，可选，默认空字符串
//      - os_type: 操作系统类型，取值"linux"或"windows"
//      - os_version: 操作系统版本，如"CentOS Linux release 7.9.2009"
//      - kernel_version: 内核版本，如"3.10.0-1160.el7.x86_64"
//      - kernel_arch: 内核架构，如"x86_64"、"arm64"
//      - hostname: 服务器主机名
//      - is_docker: 是否为Docker容器，0表示否，1表示是
//      - created_at: 创建时间，自动生成
//      - updated_at: 更新时间，自动生成
//   2. config_history表 - 存储访问控制配置历史
//      - id: 主键，自增
//      - server_id: 关联服务器ID（外键，级联删除）
//      - port: 配置的端口号
//      - ip_whitelist: IP白名单，逗号分隔
//      - chain: 防火墙链名称（INPUT或DOCKER-USER）
//      - command: 执行的完整命令
//      - output: 命令执行输出
//      - success: 是否成功，0表示失败，1表示成功
//      - created_at: 创建时间，自动生成
// 注意点:
//   - 使用CREATE TABLE IF NOT EXISTS，不会覆盖已有表
//   - config_history表的server_id字段设置了ON DELETE CASCADE，删除服务器时自动删除关联记录
func createTables() error {
	serversTable := `
		CREATE TABLE IF NOT EXISTS servers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			host TEXT NOT NULL UNIQUE,
			ssh_port INTEGER DEFAULT 22,
			username TEXT NOT NULL,
			password TEXT NOT NULL,
			key_path TEXT DEFAULT '',
			os_type TEXT DEFAULT '',
			os_version TEXT DEFAULT '',
			kernel_version TEXT DEFAULT '',
			kernel_arch TEXT DEFAULT '',
			hostname TEXT DEFAULT '',
			is_docker INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := dbInstance.Exec(serversTable); err != nil {
		return err
	}

	historyTable := `
		CREATE TABLE IF NOT EXISTS config_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			server_id INTEGER NOT NULL,
			port INTEGER NOT NULL,
			ip_whitelist TEXT NOT NULL,
			chain TEXT NOT NULL,
			command TEXT NOT NULL,
			output TEXT DEFAULT '',
			success INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
		);
	`
	if _, err := dbInstance.Exec(historyTable); err != nil {
		return err
	}

	return nil
}

// migrateTables 数据库迁移，确保表结构为最新版本
// 该函数在InitDB中调用，用于处理应用升级时的数据库结构变更
// 返回值:
//   error - 迁移过程中的错误信息，成功返回nil
// 迁移逻辑:
//   1. 使用PRAGMA table_info(servers)查询表的列信息
//   2. 遍历需要添加的列名列表
//   3. 检查每个列是否已存在于表中
//   4. 如果列不存在，使用ALTER TABLE ADD COLUMN添加
//   5. 根据列名确定列类型：is_docker为INTEGER类型，其余为TEXT类型
// 支持的迁移列:
//   - kernel_version - 内核版本（TEXT类型，默认空字符串）
//   - kernel_arch - 内核架构（TEXT类型，默认空字符串）
//   - hostname - 主机名（TEXT类型，默认空字符串）
//   - is_docker - 是否为Docker容器（INTEGER类型，默认0）
// 设计目的:
//   - 支持应用升级时的数据库结构兼容
//   - 不删除任何现有列，保证向后兼容
//   - 迁移失败仅记录日志，不中断应用启动
// 注意点:
//   - SQLite的ALTER TABLE只能添加列，不能删除或修改列
//   - 迁移操作是幂等的，多次执行不会有副作用
func migrateTables() error {
	columns := []string{
		"kernel_version",
		"kernel_arch",
		"hostname",
		"is_docker",
	}

	for _, col := range columns {
		rows, err := dbInstance.Query("PRAGMA table_info(servers)")
		if err != nil {
			continue
		}
		exists := false
		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dfltValue sql.NullString
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
				continue
			}
			if name == col {
				exists = true
				break
			}
		}
		rows.Close()

		if !exists {
			var colType string
			switch col {
			case "is_docker":
				colType = "INTEGER DEFAULT 0"
			default:
				colType = "TEXT DEFAULT ''"
			}
			_, err := dbInstance.Exec(fmt.Sprintf("ALTER TABLE servers ADD COLUMN %s %s", col, colType))
			if err != nil {
				log.Printf("数据库迁移警告: 添加列%s失败: %v", col, err)
			} else {
				log.Printf("数据库迁移: 添加列%s成功", col)
			}
		}
	}

	return nil
}

// GetDB 获取数据库实例
// 返回全局的SQLite数据库连接实例
// 返回值:
//   *sql.DB - SQLite数据库连接实例
// 注意点:
//   - 调用前必须先调用InitDB()初始化数据库
//   - 如果在InitDB之前调用，返回nil
//   - 返回的是共享实例，不需要关闭
func GetDB() *sql.DB {
	return dbInstance
}

// SaveServer 保存服务器信息到数据库
// 根据server.ID判断是新增还是更新操作
// 参数:
//   server - 服务器信息指针，传入时ID为0表示新增，ID不为0表示更新
// 返回值:
//   error - 保存过程中的错误信息，成功返回nil
// 功能:
//   - ID为0时执行INSERT操作，自动生成ID并赋值给server.ID
//   - ID不为0时执行UPDATE操作，更新所有字段并设置updated_at为当前时间
// 注意点:
//   - host字段有UNIQUE约束，重复的host会导致INSERT失败
//   - 密码字段应该是已经加密的（使用AES-GCM加密）
//   - 新增操作后，server.ID会被自动填充为数据库生成的主键值
func SaveServer(server *models.Server) error {
	if server.ID == 0 {
		result, err := dbInstance.Exec(`
			INSERT INTO servers (name, host, ssh_port, username, password, key_path, os_type, os_version, kernel_version, kernel_arch, hostname, is_docker, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`, server.Name, server.Host, server.SSHPort, server.Username, server.Password, server.KeyPath, server.OSType, server.OSVersion, server.KernelVersion, server.KernelArch, server.Hostname, server.IsDocker)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		server.ID = uint(id)
	} else {
		_, err := dbInstance.Exec(`
			UPDATE servers SET name=?, host=?, ssh_port=?, username=?, password=?, key_path=?, os_type=?, os_version=?, kernel_version=?, kernel_arch=?, hostname=?, is_docker=?, updated_at=CURRENT_TIMESTAMP
			WHERE id=?
		`, server.Name, server.Host, server.SSHPort, server.Username, server.Password, server.KeyPath, server.OSType, server.OSVersion, server.KernelVersion, server.KernelArch, server.Hostname, server.IsDocker, server.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetServerByID 根据ID获取服务器信息
// 通过主键ID查询服务器记录
// 参数:
//   id - 服务器ID，正整数
// 返回值:
//   *models.Server - 服务器信息指针，未找到返回nil
//   error - 查询过程中的错误信息，成功返回nil
// 注意点:
//   - 未找到记录时返回nil, nil（不是sql.ErrNoRows错误）
//   - 返回的服务器信息包含加密后的密码，使用前需解密
func GetServerByID(id uint) (*models.Server, error) {
	server := &models.Server{}
	err := dbInstance.QueryRow(`
		SELECT id, name, host, ssh_port, username, password, key_path, os_type, os_version, kernel_version, kernel_arch, hostname, is_docker, created_at, updated_at
		FROM servers WHERE id=?
	`, id).Scan(
		&server.ID, &server.Name, &server.Host, &server.SSHPort, &server.Username, &server.Password,
		&server.KeyPath, &server.OSType, &server.OSVersion, &server.KernelVersion, &server.KernelArch, &server.Hostname,
		&server.IsDocker, &server.CreatedAt, &server.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return server, err
}

// GetServerByHost 根据主机地址获取服务器信息
// 通过主机地址查询服务器记录，用于检测重复添加
// 参数:
//   host - 主机地址（IP或域名）
// 返回值:
//   *models.Server - 服务器信息指针，未找到返回nil
//   error - 查询过程中的错误信息，成功返回nil
// 注意点:
//   - host字段有唯一约束，最多返回一条记录
//   - 未找到记录时返回nil, nil
//   - 常用于添加服务器前检查是否已存在相同主机
func GetServerByHost(host string) (*models.Server, error) {
	server := &models.Server{}
	err := dbInstance.QueryRow(`
		SELECT id, name, host, ssh_port, username, password, key_path, os_type, os_version, kernel_version, kernel_arch, hostname, is_docker, created_at, updated_at
		FROM servers WHERE host=?
	`, host).Scan(
		&server.ID, &server.Name, &server.Host, &server.SSHPort, &server.Username, &server.Password,
		&server.KeyPath, &server.OSType, &server.OSVersion, &server.KernelVersion, &server.KernelArch, &server.Hostname,
		&server.IsDocker, &server.CreatedAt, &server.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return server, err
}

// ListServers 获取所有服务器列表
// 查询所有服务器记录并按创建时间降序排列
// 返回值:
//   []models.Server - 服务器列表，为空时返回空切片
//   error - 查询过程中的错误信息，成功返回nil
// 排序规则:
//   - 按created_at降序排列（最新添加的在前）
// 注意点:
//   - 使用defer rows.Close()确保结果集正确关闭
//   - 返回的列表包含所有服务器，包括已删除的（软删除未实现）
//   - 返回的密码字段是加密的
func ListServers() ([]models.Server, error) {
	rows, err := dbInstance.Query(`
		SELECT id, name, host, ssh_port, username, password, key_path, os_type, os_version, kernel_version, kernel_arch, hostname, is_docker, created_at, updated_at
		FROM servers ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []models.Server
	for rows.Next() {
		server := models.Server{}
		if err := rows.Scan(
			&server.ID, &server.Name, &server.Host, &server.SSHPort, &server.Username, &server.Password,
			&server.KeyPath, &server.OSType, &server.OSVersion, &server.KernelVersion, &server.KernelArch, &server.Hostname,
			&server.IsDocker, &server.CreatedAt, &server.UpdatedAt,
		); err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}

	return servers, nil
}

// DeleteServer 根据ID删除服务器
// 通过主键ID删除服务器记录及其关联的配置历史
// 参数:
//   id - 服务器ID，正整数
// 返回值:
//   error - 删除过程中的错误信息，成功返回nil
// 级联删除:
//   - 删除服务器时，config_history表中关联的记录会自动删除（外键ON DELETE CASCADE）
// 注意点:
//   - 这是物理删除，数据不可恢复
//   - 删除前应确保没有其他操作依赖该服务器
//   - 删除成功后返回nil，即使没有匹配的记录
func DeleteServer(id uint) error {
	_, err := dbInstance.Exec("DELETE FROM servers WHERE id=?", id)
	return err
}

// SaveConfigHistory 保存配置历史记录
// 将访问控制配置操作记录到config_history表，用于审计和追溯
// 参数:
//   history - 配置历史记录指针，传入时ID应为0
// 返回值:
//   error - 保存过程中的错误信息，成功返回nil
// 功能:
//   - 将配置操作记录到数据库
//   - 自动生成ID并赋值给history.ID
//   - 记录内容包括端口、IP白名单、防火墙链、执行命令、输出和执行结果
// 注意点:
//   - 调用前应确保ServerID已正确设置
//   - Command字段应存储完整的执行命令
//   - Output字段可能包含敏感信息，需注意权限控制
func SaveConfigHistory(history *models.ConfigHistory) error {
	result, err := dbInstance.Exec(`
		INSERT INTO config_history (server_id, port, ip_whitelist, chain, command, output, success, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, history.ServerID, history.Port, history.IPWhitelist, history.Chain, history.Command, history.Output, history.Success)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	history.ID = uint(id)
	return nil
}

// GetConfigHistoryByServerID 获取指定服务器的配置历史
// 查询指定服务器的所有配置历史记录
// 参数:
//   serverID - 服务器ID，正整数
// 返回值:
//   []models.ConfigHistory - 配置历史记录列表，为空时返回空切片
//   error - 查询过程中的错误信息，成功返回nil
// 排序规则:
//   - 按created_at降序排列（最新配置在前）
// 注意点:
//   - 使用defer rows.Close()确保结果集正确关闭
//   - 返回的记录包含完整的命令和输出信息
func GetConfigHistoryByServerID(serverID uint) ([]models.ConfigHistory, error) {
	rows, err := dbInstance.Query(`
		SELECT id, server_id, port, ip_whitelist, chain, command, output, success, created_at
		FROM config_history WHERE server_id=? ORDER BY created_at DESC
	`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []models.ConfigHistory
	for rows.Next() {
		history := models.ConfigHistory{}
		if err := rows.Scan(
			&history.ID, &history.ServerID, &history.Port, &history.IPWhitelist,
			&history.Chain, &history.Command, &history.Output, &history.Success, &history.CreatedAt,
		); err != nil {
			return nil, err
		}
		histories = append(histories, history)
	}

	return histories, nil
}
