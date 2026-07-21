package db

import (
	"access-control-tool/internal/models"
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

// 全局数据库实例
var (
	dbInstance *sql.DB  // SQLite数据库连接实例
	dbPath     string   // 数据库文件路径
)

// InitDB 初始化SQLite数据库连接并创建必要的表
// 返回值:
//   error - 初始化过程中的错误信息，成功返回nil
// 初始化流程:
//   1. 设置数据库文件路径为"data.db"
//   2. 使用modernc.org/sqlite驱动打开数据库
//   3. 执行Ping验证连接可用性
//   4. 创建servers和config_history表
//   5. 执行数据库迁移（添加缺失的列）
// 注意:
//   - 使用modernc.org/sqlite纯Go实现，无需CGO依赖
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
// 返回值:
//   error - 创建过程中的错误信息，成功返回nil
// 创建的表:
//   1. servers表 - 存储服务器基本信息
//      - id: 主键，自增
//      - name: 服务器名称
//      - host: 主机地址（唯一约束）
//      - ssh_port: SSH端口（默认22）
//      - username: 登录用户名
//      - password: 加密后的密码
//      - os_type: 操作系统类型（linux/windows）
//      - os_version: 操作系统版本
//      - kernel_version: 内核版本
//      - kernel_arch: 内核架构
//      - hostname: 主机名
//      - is_docker: 是否为Docker容器（0/1）
//      - created_at/updated_at: 创建和更新时间
//   2. config_history表 - 存储访问控制配置历史
//      - server_id: 关联服务器ID（外键级联删除）
//      - port: 配置的端口号
//      - ip_whitelist: IP白名单
//      - chain: 防火墙链名称
//      - command: 执行的命令
//      - output: 命令输出
//      - success: 是否成功（0/1）
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
// 返回值:
//   error - 迁移过程中的错误信息，成功返回nil
// 迁移逻辑:
//   - 使用PRAGMA table_info检查表中是否存在指定列
//   - 如果列不存在，使用ALTER TABLE ADD COLUMN添加
//   - 支持的迁移列:
//     * kernel_version - 内核版本（TEXT类型）
//     * kernel_arch - 内核架构（TEXT类型）
//     * hostname - 主机名（TEXT类型）
//     * is_docker - 是否为Docker容器（INTEGER类型，默认0）
// 设计目的:
//   - 支持应用升级时的数据库结构兼容
//   - 不删除任何现有列，保证向后兼容
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
// 返回值:
//   *sql.DB - SQLite数据库连接实例
// 注意:
//   - 调用前必须先调用InitDB()初始化数据库
func GetDB() *sql.DB {
	return dbInstance
}

// SaveServer 保存服务器信息到数据库
// 参数:
//   server - 服务器信息指针
// 返回值:
//   error - 保存过程中的错误信息，成功返回nil
// 功能:
//   - 根据server.ID判断是新增还是更新操作
//   - ID为0时执行INSERT操作，自动生成ID并赋值给server.ID
//   - ID不为0时执行UPDATE操作，更新所有字段并设置updated_at
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
// 参数:
//   id - 服务器ID
// 返回值:
//   *models.Server - 服务器信息指针，未找到返回nil
//   error - 查询过程中的错误信息
// 注意:
//   - 未找到记录时返回nil, nil（非sql.ErrNoRows错误）
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
// 参数:
//   host - 主机地址（IP或域名）
// 返回值:
//   *models.Server - 服务器信息指针，未找到返回nil
//   error - 查询过程中的错误信息
// 注意:
//   - host字段有唯一约束，最多返回一条记录
//   - 未找到记录时返回nil, nil
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
// 返回值:
//   []models.Server - 服务器列表
//   error - 查询过程中的错误信息
// 排序规则:
//   - 按created_at降序排列（最新添加的在前）
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
// 参数:
//   id - 服务器ID
// 返回值:
//   error - 删除过程中的错误信息，成功返回nil
// 级联删除:
//   - 删除服务器时，config_history表中关联的记录会自动删除（外键ON DELETE CASCADE）
func DeleteServer(id uint) error {
	_, err := dbInstance.Exec("DELETE FROM servers WHERE id=?", id)
	return err
}

// SaveConfigHistory 保存配置历史记录
// 参数:
//   history - 配置历史记录指针
// 返回值:
//   error - 保存过程中的错误信息，成功返回nil
// 功能:
//   - 将访问控制配置操作记录到config_history表
//   - 自动生成ID并赋值给history.ID
//   - 记录内容包括端口、IP白名单、防火墙链、执行命令、输出和执行结果
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
// 参数:
//   serverID - 服务器ID
// 返回值:
//   []models.ConfigHistory - 配置历史记录列表
//   error - 查询过程中的错误信息
// 排序规则:
//   - 按created_at降序排列（最新配置在前）
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
