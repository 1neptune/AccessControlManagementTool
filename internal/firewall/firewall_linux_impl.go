package firewall

import (
	"access-control-tool/internal/models"
	"fmt"
	"log"
	"strings"
)

type DistroInfo struct {
	Name                  string // 发行版名称：ubuntu/kali/centos/redhat/rocky
	PackageManager        string // 包管理器：apt/yum
	FirewallType          string // 防火墙类型：firewalld/ufw
	IptablesPackageName   string // iptables持久化包名：iptables-persistent（apt）/ iptables-services（yum）
	IptablesServiceName   string // iptables服务名：netfilter-persistent（apt）/ iptables（yum）
	IptablesSaveCommand   string // iptables规则保存命令：netfilter-persistent save / service iptables save
}

// detectDistro 检测Linux发行版信息
// 参数:
//   client - SSH客户端
// 返回值:
//   DistroInfo - 发行版信息
//   error - 检测过程中的错误
// 支持的发行版: ubuntu/kali/centos/redhat/rocky
func detectDistro(client SSHClient) (DistroInfo, error) {
	log.Printf("[Firewall] 开始检测Linux发行版")

	output, err := client.Execute(`cat /etc/os-release 2>/dev/null || cat /etc/redhat-release 2>/dev/null || cat /etc/lsb-release 2>/dev/null`)
	if err != nil {
		log.Printf("[Firewall] 读取发行版信息失败: %v", err)
		return DistroInfo{}, fmt.Errorf("读取发行版信息失败: %v", err)
	}

	info := DistroInfo{}

	switch {
	case strings.Contains(strings.ToLower(output), "ubuntu"):
		info.Name = "ubuntu"
		info.PackageManager = "apt"
		info.FirewallType = "ufw"
		info.IptablesPackageName = "iptables-persistent"
		info.IptablesServiceName = "netfilter-persistent"
		info.IptablesSaveCommand = "netfilter-persistent save"
	case strings.Contains(strings.ToLower(output), "kali"):
		info.Name = "kali"
		info.PackageManager = "apt"
		info.FirewallType = "ufw"
		info.IptablesPackageName = "iptables-persistent"
		info.IptablesServiceName = "netfilter-persistent"
		info.IptablesSaveCommand = "netfilter-persistent save"
	case strings.Contains(strings.ToLower(output), "centos"):
		info.Name = "centos"
		info.PackageManager = "yum"
		info.FirewallType = "firewalld"
		info.IptablesPackageName = "iptables-services"
		info.IptablesServiceName = "iptables"
		info.IptablesSaveCommand = "service iptables save"
	case strings.Contains(strings.ToLower(output), "red hat") || strings.Contains(strings.ToLower(output), "redhat"):
		info.Name = "redhat"
		info.PackageManager = "yum"
		info.FirewallType = "firewalld"
		info.IptablesPackageName = "iptables-services"
		info.IptablesServiceName = "iptables"
		info.IptablesSaveCommand = "service iptables save"
	case strings.Contains(strings.ToLower(output), "rocky"):
		info.Name = "rocky"
		info.PackageManager = "yum"
		info.FirewallType = "firewalld"
		info.IptablesPackageName = "iptables-services"
		info.IptablesServiceName = "iptables"
		info.IptablesSaveCommand = "service iptables save"
	default:
		info.Name = "unknown"
		info.PackageManager = "yum"
		info.FirewallType = "firewalld"
		info.IptablesPackageName = "iptables-services"
		info.IptablesServiceName = "iptables"
		info.IptablesSaveCommand = "service iptables save"
	}

	log.Printf("[Firewall] 发行版检测结果: %s, 包管理器: %s, 防火墙类型: %s", info.Name, info.PackageManager, info.FirewallType)
	return info, nil
}

// checkFirewalldStatus 检查firewalld状态
// 参数:
//   client - SSH客户端
// 返回值:
//   bool - 是否运行中
//   error - 检查过程中的错误
func checkFirewalldStatus(client SSHClient) (bool, error) {
	output, err := client.Execute(`systemctl is-active firewalld 2>/dev/null || echo 'inactive'`)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) == "active", nil
}

// checkUfwStatus 检查ufw状态
// 参数:
//   client - SSH客户端
// 返回值:
//   bool - 是否运行中
//   error - 检查过程中的错误
func checkUfwStatus(client SSHClient) (bool, error) {
	output, err := client.Execute(`systemctl is-active ufw 2>/dev/null || echo 'inactive'`)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) == "active", nil
}

// stopAndDisableFirewall 停止并禁用系统防火墙
// 参数:
//   client - SSH客户端
//   firewallType - 防火墙类型（firewalld/ufw）
//   callback - 进度回调函数
// 返回值:
//   error - 操作过程中的错误
// 执行命令:
//   firewalld: systemctl stop firewalld && systemctl disable firewalld
//   ufw: systemctl stop ufw && systemctl disable ufw
func stopAndDisableFirewall(client SSHClient, firewallType string, callback func(message string)) error {
	log.Printf("[Firewall] 停止并禁用防火墙，类型: %s", firewallType)

	if callback != nil {
		callback(fmt.Sprintf("⏹️ 停止并禁用%s...\n", firewallType))
	}

	var commands []string
	switch firewallType {
	case "firewalld":
		commands = []string{
			"systemctl stop firewalld",
			"systemctl disable firewalld",
		}
	case "ufw":
		commands = []string{
			"systemctl stop ufw",
			"systemctl disable ufw",
		}
	default:
		return fmt.Errorf("不支持的防火墙类型: %s", firewallType)
	}

	for _, cmd := range commands {
		output, err := client.Execute(cmd)
		if err != nil {
			log.Printf("[Firewall] 执行命令失败: %s, 错误: %v", cmd, err)
			if callback != nil {
				callback(fmt.Sprintf("❌ 执行命令失败: %s - %v\n", cmd, err))
			}
			return fmt.Errorf("执行命令失败: %s - %v", cmd, err)
		} else {
			log.Printf("[Firewall] 命令执行成功: %s, 输出: %s", cmd, output)
		}
	}

	log.Printf("[Firewall] %s已停止并禁用", firewallType)
	if callback != nil {
		callback(fmt.Sprintf("✅ %s已停止并禁用\n", firewallType))
	}
	return nil
}

// checkIptablesServicesInstalled 检查iptables持久化服务是否安装和启动
// 参数:
//   client - SSH客户端
//   serviceName - 服务名（netfilter-persistent / iptables）
// 返回值:
//   bool - 是否已安装
//   bool - 是否已启动
//   error - 检查过程中的错误
func checkIptablesServicesInstalled(client SSHClient, serviceName string) (bool, bool, error) {
	log.Printf("[Firewall] 检查iptables服务状态，服务名: %s", serviceName)

	output, err := client.Execute(fmt.Sprintf(`systemctl list-unit-files | grep -i %s 2>/dev/null || echo ''`, serviceName))
	if err != nil {
		log.Printf("[Firewall] 检查iptables服务安装状态失败: %v", err)
		return false, false, err
	}

	isInstalled := strings.Contains(strings.ToLower(output), strings.ToLower(serviceName))
	log.Printf("[Firewall] iptables服务安装状态: %v", isInstalled)

	if !isInstalled {
		return false, false, nil
	}

	output, err = client.Execute(fmt.Sprintf(`systemctl is-active %s 2>/dev/null || echo 'inactive'`, serviceName))
	if err != nil {
		log.Printf("[Firewall] 检查iptables服务状态失败: %v", err)
		return true, false, err
	}

	isActive := strings.TrimSpace(output) == "active"
	log.Printf("[Firewall] iptables服务运行状态: %v", isActive)

	return isInstalled, isActive, nil
}



// installIptablesServices 安装iptables持久化包
// 参数:
//   client - SSH客户端
//   packageManager - 包管理器（apt/yum）
//   packageName - 包名（iptables-persistent / iptables-services）
//   callback - 进度回调函数
// 返回值:
//   error - 安装过程中的错误
// 判断逻辑: 使用白名单机制，安装输出中包含以下关键词即为成功:
//   already installed, nothing to do, installed, success, completed, 已安装
//   其他情况均为失败
func installIptablesServices(client SSHClient, packageManager string, packageName string, callback func(message string)) error {
	log.Printf("[Firewall] 安装iptables持久化包，包管理器: %s, 包名: %s", packageManager, packageName)

	if callback != nil {
		callback(fmt.Sprintf("📦 使用%s安装%s...\n", packageManager, packageName))
	}

	var installCmd string
	switch packageManager {
	case "apt":
		installCmd = fmt.Sprintf("apt install -y %s", packageName)
	case "yum":
		installCmd = fmt.Sprintf("yum install -y %s", packageName)
	default:
		installCmd = fmt.Sprintf("yum install -y %s", packageName)
	}

	output, err := client.Execute(installCmd)
	if err != nil {
		log.Printf("[Firewall] 安装命令执行失败: %v", err)
		if callback != nil {
			callback(fmt.Sprintf("❌ 安装命令执行失败: %v\n", err))
		}
		return fmt.Errorf("安装命令执行失败: %v", err)
	}
	log.Printf("[Firewall] 安装命令输出: %s", output)

	successKeywords := []string{
		"already installed",
		"nothing to do",
		"installed",
		"success",
		"completed",
		"已安装",
		"is already the newest",
		"0 upgraded",
		"no packages to install",
	}

	isSuccess := false
	lowerOutput := strings.ToLower(output)
	for _, keyword := range successKeywords {
		if strings.Contains(lowerOutput, keyword) {
			isSuccess = true
			break
		}
	}

	if !isSuccess {
		log.Printf("[Firewall] 安装%s失败: 输出中未包含成功关键词", packageName)
		if callback != nil {
			callback(fmt.Sprintf("❌ 安装%s失败\n", packageName))
		}
		return fmt.Errorf("安装%s失败", packageName)
	}

	log.Printf("[Firewall] %s安装成功", packageName)
	if callback != nil {
		callback(fmt.Sprintf("✅ %s安装成功\n", packageName))
	}
	return nil
}

// installIpsetPackage 安装ipset包
// 参数:
//   client - SSH客户端
//   packageManager - 包管理器（apt/yum）
//   callback - 进度回调函数
// 返回值:
//   error - 安装过程中的错误
func installIpsetPackage(client SSHClient, packageManager string, callback func(message string)) error {
	log.Printf("[Firewall] 安装ipset包，包管理器: %s", packageManager)

	if callback != nil {
		callback("📦 安装ipset包...\n")
	}

	var installCmd string
	switch packageManager {
	case "apt":
		installCmd = "apt install -y ipset"
	case "yum":
		installCmd = "yum install -y ipset"
	default:
		installCmd = "yum install -y ipset"
	}

	output, err := client.Execute(installCmd)
	if err != nil {
		log.Printf("[Firewall] ipset安装命令执行失败: %v", err)
		if callback != nil {
			callback(fmt.Sprintf("❌ ipset安装命令执行失败: %v\n", err))
		}
		return fmt.Errorf("ipset安装命令执行失败: %v", err)
	}
	log.Printf("[Firewall] ipset安装命令输出: %s", output)

	successKeywords := []string{
		"already installed",
		"nothing to do",
		"installed",
		"success",
		"completed",
		"已安装",
		"is already the newest",
		"0 upgraded",
		"no packages to install",
	}

	isSuccess := false
	lowerOutput := strings.ToLower(output)
	for _, keyword := range successKeywords {
		if strings.Contains(lowerOutput, keyword) {
			isSuccess = true
			break
		}
	}

	if !isSuccess {
		log.Printf("[Firewall] 安装ipset失败")
		if callback != nil {
			callback("❌ 安装ipset失败\n")
		}
		return fmt.Errorf("安装ipset失败")
	}

	log.Printf("[Firewall] ipset安装成功")
	if callback != nil {
		callback("✅ ipset安装成功\n")
	}
	return nil
}

// startAndEnableIptablesServices 启动并启用iptables持久化服务
// 参数:
//   client - SSH客户端
//   serviceName - 服务名（netfilter-persistent / iptables）
//   callback - 进度回调函数
// 返回值:
//   error - 操作过程中的错误
func startAndEnableIptablesServices(client SSHClient, serviceName string, callback func(message string)) error {
	log.Printf("[Firewall] 启动并启用iptables服务，服务名: %s", serviceName)

	if callback != nil {
		callback(fmt.Sprintf("🚀 启动并启用%s...\n", serviceName))
	}

	commands := []string{
		fmt.Sprintf("systemctl start %s", serviceName),
		fmt.Sprintf("systemctl enable %s", serviceName),
	}

	for _, cmd := range commands {
		output, err := client.Execute(cmd)
		if err != nil {
			log.Printf("[Firewall] 执行命令失败: %s, 错误: %v", cmd, err)
			if callback != nil {
				callback(fmt.Sprintf("❌ 执行命令失败: %s - %v\n", cmd, err))
			}
			return fmt.Errorf("执行命令失败: %s - %v", cmd, err)
		} else {
			log.Printf("[Firewall] 命令执行成功: %s, 输出: %s", cmd, output)
		}
	}

	if callback != nil {
		callback(fmt.Sprintf("✅ %s已启动并启用\n", serviceName))
	}
	return nil
}

// checkPortServiceIsDocker 检查端口服务是否是Docker服务
// 参数:
//   client - SSH客户端
//   port - 端口号
// 返回值:
//   bool - 是否为Docker服务
//   error - 检测过程中的错误
// 判断逻辑: 通过ss/netstat输出判断端口服务名是否包含docker
func checkPortServiceIsDocker(client SSHClient, port int) (bool, error) {
	log.Printf("[Firewall] 检查端口%d服务是否是Docker服务", port)

	output, err := client.Execute(fmt.Sprintf(`ss -tlnp 2>/dev/null | grep -w ":%d" || netstat -tlnp 2>/dev/null | grep -w ":%d"`, port, port))
	if err != nil {
		log.Printf("[Firewall] 检查端口服务失败: %v", err)
		return false, err
	}

	isDocker := strings.Contains(strings.ToLower(output), "docker")
	log.Printf("[Firewall] 端口%d服务是否为Docker: %v", port, isDocker)

	return isDocker, nil
}

// createIpsetRestoreService 创建ipset自启动服务（先检查是否已存在）
// 参数:
//   client - SSH客户端
//   callback - 进度回调函数
// 返回值:
//   error - 操作过程中的错误
// 执行流程:
//   1. 判断ipset-restore服务是否已启用（systemctl is-enabled ipset-restore）
//   2. 如果已启用，判断是否已启动（systemctl is-active ipset-restore）
//   3. 如果已启用且已启动，跳过创建
//   4. 如果已启用但未启动，只启动服务
//   5. 如果未启用，创建服务文件并启用启动
func createIpsetRestoreService(client SSHClient, callback func(message string)) error {
	log.Printf("[Firewall] 检查并创建ipset自启动服务")

	if callback != nil {
		callback("🔧 检查ipset自启动服务...\n")
	}

	output, _ := client.Execute(`systemctl is-enabled ipset-restore 2>/dev/null || echo 'disabled'`)
	isEnabled := strings.TrimSpace(output) == "enabled"

	if isEnabled {
		log.Printf("[Firewall] ipset-restore服务已启用")

		output, _ := client.Execute(`systemctl is-active ipset-restore 2>/dev/null`)
		isActive := strings.TrimSpace(output)

		if isActive == "active" || isActive == "inactive" {
			log.Printf("[Firewall] ipset-restore服务已存在，状态: %s", isActive)
			if callback != nil {
				callback("✅ ipset-restore服务已存在，跳过创建\n")
			}
			return nil
		}

		if isActive == "failed" {
			log.Printf("[Firewall] ipset-restore服务启动失败，需要修复")
			return fmt.Errorf("ipset-restore服务状态为failed，请检查服务配置")
		}

		log.Printf("[Firewall] ipset-restore服务已启用但状态异常(%s)，启动服务", isActive)
		if callback != nil {
			callback("🔄 ipset-restore服务状态异常，启动服务...")
		}

		_, err := client.Execute("systemctl start ipset-restore")
		if err != nil {
			log.Printf("[Firewall] 启动ipset-restore服务失败: %v", err)
			return fmt.Errorf("启动ipset-restore服务失败: %v", err)
		}

		log.Printf("[Firewall] ipset-restore服务启动成功")
		if callback != nil {
			callback("✅ 启动成功\n")
		}
		return nil
	}

	log.Printf("[Firewall] ipset-restore服务未启用，创建服务")
	if callback != nil {
		callback("📦 创建ipset-restore服务...\n")
	}

	serviceFile := `[Unit]
Description=Restore ipset rules on boot
Before=iptables.service

[Service]
Type=oneshot
ExecStart=/bin/sh -c '/sbin/ipset restore -exist < /etc/ipset.conf'

[Install]
WantedBy=multi-user.target
`

	createCmd := fmt.Sprintf("cat > /etc/systemd/system/ipset-restore.service <<'EOF'\n%sEOF", serviceFile)
	_, err := client.Execute(createCmd)
	if err != nil {
		log.Printf("[Firewall] 创建ipset-restore.service失败: %v", err)
		if callback != nil {
			callback(fmt.Sprintf("❌ 创建ipset-restore.service失败: %v\n", err))
		}
		return err
	}

	commands := []string{
		"systemctl daemon-reload",
		"systemctl enable ipset-restore",
		"systemctl start ipset-restore",
	}

	for _, cmd := range commands {
		_, err := client.Execute(cmd)
		if err != nil {
			log.Printf("[Firewall] 执行命令失败: %s, 错误: %v", cmd, err)
			if callback != nil {
				callback(fmt.Sprintf("❌ 执行命令失败: %s - %v\n", cmd, err))
			}
			return fmt.Errorf("执行命令失败: %s - %v", cmd, err)
		} else {
			log.Printf("[Firewall] 命令执行成功: %s", cmd)
		}
	}

	log.Printf("[Firewall] ipset自启动服务创建完成")
	if callback != nil {
		callback("✅ ipset自启动服务创建完成\n")
	}
	return nil
}

// manageIpset 管理ipset白名单
// 参数:
//   client - SSH客户端
//   port - 端口号
//   ipWhitelist - IP白名单（逗号分隔）
//   callback - 进度回调函数
// 返回值:
//   error - 操作过程中的错误
// 执行流程:
//   1. 判断ipset集合是否存在（{port}_allow_ip）
//   2. 如果不存在，创建集合 ipset create {port}_allow_ip hash:net
//   3. 检查每个IP是否已存在于集合中
//   4. 如果不存在，添加IP ipset add -exist {port}_allow_ip {ip}
//   5. 保存ipset配置 ipset save > /etc/ipset.conf
func manageIpset(client SSHClient, port int, ipWhitelist string, callback func(message string)) error {
	log.Printf("[Firewall] 管理ipset白名单，端口: %d, IP白名单: %s", port, ipWhitelist)

	if callback != nil {
		callback("🔧 管理ipset白名单...\n")
	}

	setName := fmt.Sprintf("%d_allow_ip", port)

	output, err := client.Execute(fmt.Sprintf(`ipset list %s 2>/dev/null || echo 'not_found'`, setName))
	if err != nil {
		log.Printf("[Firewall] 检查ipset集合失败: %v", err)
		return err
	}

	if strings.Contains(strings.ToLower(output), "not_found") || !strings.Contains(output, setName) {
		log.Printf("[Firewall] ipset集合%s不存在，创建集合", setName)
		if callback != nil {
			callback(fmt.Sprintf("📦 创建ipset集合%s...\n", setName))
		}

		createCmd := fmt.Sprintf("ipset create %s hash:net", setName)
		_, err := client.Execute(createCmd)
		if err != nil {
			log.Printf("[Firewall] 创建ipset集合失败: %v", err)
			return err
		}
		log.Printf("[Firewall] ipset集合%s创建成功", setName)
		if callback != nil {
			callback(fmt.Sprintf("✅ ipset集合%s创建成功\n", setName))
		}
	} else {
		log.Printf("[Firewall] ipset集合%s已存在", setName)
		if callback != nil {
			callback(fmt.Sprintf("✅ ipset集合%s已存在\n", setName))
		}
	}

	ips := strings.Split(ipWhitelist, ",")
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}

		output, err := client.Execute(fmt.Sprintf(`ipset test %s %s 2>/dev/null`, setName, ip))
		isInSet := true
		if err != nil || strings.Contains(strings.ToLower(output), "not in set") {
			isInSet = false
		}

		if !isInSet {
			log.Printf("[Firewall] IP%s不存在于集合%s中，添加IP", ip, setName)
			if callback != nil {
				callback(fmt.Sprintf("➕ 添加IP%s到白名单...", ip))
			}

			addCmd := fmt.Sprintf("ipset add -exist %s %s", setName, ip)
			_, err := client.Execute(addCmd)
			if err != nil {
				log.Printf("[Firewall] 添加IP%s失败: %v", ip, err)
				return fmt.Errorf("添加IP%s失败: %v", ip, err)
			}
			log.Printf("[Firewall] IP%s添加成功", ip)
			if callback != nil {
				callback(fmt.Sprintf("✅ IP%s添加成功\n", ip))
			}
		} else {
			log.Printf("[Firewall] IP%s已存在于集合%s中，跳过", ip, setName)
			if callback != nil {
				callback(fmt.Sprintf("⏭️ IP%s已存在，跳过\n", ip))
			}
		}
	}

	saveCmd := "ipset save > /etc/ipset.conf"
	output, err = client.Execute(saveCmd)
	if err != nil {
		log.Printf("[Firewall] 保存ipset配置失败: %v", err)
		if callback != nil {
			callback(fmt.Sprintf("❌ 保存ipset配置失败: %v\n", err))
		}
		return fmt.Errorf("保存ipset配置失败: %v", err)
	}
	log.Printf("[Firewall] ipset配置保存成功")
	if callback != nil {
		callback("✅ ipset配置保存成功\n")
	}

	return nil
}

// checkDockerChains 检查Docker相关链是否可用
// 参数:
//   client - SSH客户端
//   callback - 进度回调函数
// 返回值:
//   bool - 链是否可用
//   error - 检测过程中的错误
// 检查逻辑:
//   只检查 DOCKER 和 DOCKER-USER 两个链
//   如果输出包含 "No chain/target/match by that name" 或输出为空，说明链不可用
//   链不可用时尝试重启Docker服务
func checkDockerChains(client SSHClient, callback func(message string)) (bool, error) {
	log.Printf("[Firewall] 检查Docker相关链是否可用")

	if callback != nil {
		callback("🔧 检查Docker链（DOCKER、DOCKER-USER）...\n")
	}

	checkCommands := []string{
		"iptables -L DOCKER -n",
		"iptables -L DOCKER-USER -n",
	}

	allChainsAvailable := true
	for _, cmd := range checkCommands {
		output, err := client.Execute(cmd)
		if err != nil || output == "" || strings.Contains(output, "No chain/target/match by that name") {
			log.Printf("[Firewall] Docker链检查失败: %s", cmd)
			allChainsAvailable = false
			break
		}
	}

	if allChainsAvailable {
		log.Printf("[Firewall] Docker链检查通过")
		if callback != nil {
			callback("✅ Docker链检查通过\n")
		}
		return true, nil
	}

	log.Printf("[Firewall] Docker链不可用，尝试重启Docker服务")
	if callback != nil {
		callback("🔄 Docker链不可用，重启Docker服务...\n")
	}

	_, err := client.Execute("systemctl restart docker")
	if err != nil {
		log.Printf("[Firewall] 重启Docker服务失败: %v", err)
		if callback != nil {
			callback(fmt.Sprintf("⚠️ 重启Docker服务失败: %v\n", err))
		}
		return false, err
	}

	log.Printf("[Firewall] Docker服务重启成功")
	if callback != nil {
		callback("✅ Docker服务重启成功\n")
	}

	allChainsAvailable = true
	for _, cmd := range checkCommands {
		output, err := client.Execute(cmd)
		if err != nil || output == "" || strings.Contains(output, "No chain/target/match by that name") {
			log.Printf("[Firewall] Docker链检查失败: %s", cmd)
			allChainsAvailable = false
			break
		}
	}

	if allChainsAvailable {
		log.Printf("[Firewall] Docker链检查通过（重启后）")
		if callback != nil {
			callback("✅ Docker链检查通过（重启后）\n")
		}
	} else {
		log.Printf("[Firewall] Docker链检查失败（重启后）")
		if callback != nil {
			callback("⚠️ Docker链检查失败（重启后）\n")
		}
	}

	return allChainsAvailable, nil
}

// ensureDockerChains 统一检查Docker环境及DOCKER链可用性
// 参数:
//   client - SSH客户端
//   port - 端口号
//   callback - 进度回调函数
// 返回值:
//   isDockerEnv - 服务器是否存在Docker运行环境（用于决定是否需要检查链）
//   portIsDockerProxy - 目标端口是否由docker-proxy监听（用于决定规则添加到INPUT还是DOCKER-USER链）
//   error - 操作过程中的错误
// 执行流程:
//   1. 检查服务器上Docker服务是否正在运行（systemctl is-active docker）
//      - 这是判断是否为Docker环境的依据，与目标端口无关
//      - 只要Docker服务在运行，就说明服务器处于Docker环境
//   2. 如果Docker服务未运行，返回(false, false, nil)，跳过所有Docker相关检查
//   3. 如果Docker服务正在运行，检查DOCKER和DOCKER-USER链是否可用
//      - 无论目标端口是普通服务还是Docker容器服务，只要Docker环境存在
//        就必须确保DOCKER和DOCKER-USER链可用
//      - 这是为了防止配置普通端口规则时破坏Docker容器的网络访问
//   4. 如果链可用，检查目标端口是否由docker-proxy监听（用于后续规则选择）
//   5. 如果链不可用，执行systemctl restart docker重启Docker服务
//      - 背景: Docker服务在iptables安装之后启动时，iptables规则冲突
//        会导致Docker的DOCKER和DOCKER-USER链无法正确创建
//      - 重启Docker可以让它重新初始化iptables链，恢复Docker网络功能
//   6. 等待3秒让Docker服务完全启动
//   7. 再次检查DOCKER和DOCKER-USER链是否可用
//   8. 返回Docker环境状态、端口是否为docker-proxy、链可用性结果
// 注意:
//   - 此函数必须在iptables服务检查通过之后、配置任何规则之前调用
//   - 关键设计: 只要服务器上有Docker服务在运行，不管配置的是什么端口，
//     都必须检查DOCKER和DOCKER-USER链，确保Docker网络不被破坏
//   - 避免服务器先安装Docker再安装iptables导致Docker服务无法访问的问题
func ensureDockerChains(client SSHClient, port int, callback func(message string)) (bool, bool, error) {
	log.Printf("[Firewall] 开始统一检查Docker环境，端口: %d", port)

	if callback != nil {
		callback("🐳 检查Docker环境...\n")
	}

	// Step 1: 检查服务器上Docker服务是否正在运行
	// 判断依据: systemctl is-active docker 返回 "active" 说明Docker在运行
	// 重要: 此检查与目标端口无关，只要Docker服务在运行就视为Docker环境
	dockerStatus, err := client.Execute("systemctl is-active docker 2>/dev/null || echo 'inactive'")
	dockerActive := strings.TrimSpace(dockerStatus) == "active"
	if err != nil || !dockerActive {
		log.Printf("[Firewall] Docker服务未运行，跳过Docker环境检查")
		if callback != nil {
			callback("🐳 Docker服务未运行\n")
		}
		return false, false, nil
	}

	log.Printf("[Firewall] Docker服务正在运行，服务器为Docker环境")
	if callback != nil {
		callback("🐳 Docker服务正在运行，检查DOCKER/DOCKER-USER链...\n")
	}

	// Step 2: Docker环境下，必须检查DOCKER和DOCKER-USER链是否可用
	// 无论目标端口是普通服务还是Docker容器服务，都要确保链可用
	// 背景: 配置普通端口的iptables规则也可能影响Docker容器的网络
	chainCheckCommands := []string{
		"iptables -L DOCKER -n",
		"iptables -L DOCKER-USER -n",
	}

	chainsAvailable := true
	for _, cmd := range chainCheckCommands {
		output, err := client.Execute(cmd)
		if err != nil || output == "" || strings.Contains(output, "No chain/target/match by that name") {
			log.Printf("[Firewall] Docker链检查失败: %s, 错误: %v, 输出: %s", cmd, err, output)
			chainsAvailable = false
			break
		}
	}

	// Step 3: 如果链不可用，执行systemctl restart docker重启Docker服务以重建链
	// 背景: 当服务器先安装Docker服务，再安装iptables持久化服务时，
	// iptables服务的启动可能会清除Docker创建的iptables链。
	// 重启Docker服务可以让它重新初始化DOCKER和DOCKER-USER链，
	// 确保Docker容器的网络功能正常。
	if !chainsAvailable {
		log.Printf("[Firewall] Docker链不可用，重启Docker服务以重建链")
		if callback != nil {
			callback("⚠️ Docker链不可用，重启Docker服务...\n")
		}

		_, err = client.Execute("systemctl restart docker")
		if err != nil {
			log.Printf("[Firewall] 重启Docker服务失败: %v", err)
			if callback != nil {
				callback(fmt.Sprintf("❌ 重启Docker服务失败: %v\n", err))
			}
			// 重启失败但仍返回Docker环境状态，不阻塞后续流程
			portIsDockerProxy, _ := checkPortServiceIsDocker(client, port)
			return true, portIsDockerProxy, nil
		}

		log.Printf("[Firewall] Docker服务重启成功，等待链重建...")
		if callback != nil {
			callback("✅ Docker服务重启成功\n")
		}

		// 等待Docker服务完全启动并重建iptables链
		_, _ = client.Execute("sleep 3")

		// 再次检查DOCKER和DOCKER-USER链是否重建成功
		chainsAvailable = true
		for _, cmd := range chainCheckCommands {
			output, err := client.Execute(cmd)
			if err != nil || output == "" || strings.Contains(output, "No chain/target/match by that name") {
				log.Printf("[Firewall] Docker链检查失败（重启后）: %s", cmd)
				chainsAvailable = false
				break
			}
		}

		if !chainsAvailable {
			log.Printf("[Firewall] Docker链重建失败")
			if callback != nil {
				callback("❌ Docker链重建失败\n")
			}
			// 链重建失败但仍继续流程，后续配置可能会受限
		} else {
			log.Printf("[Firewall] Docker链重建成功")
			if callback != nil {
				callback("✅ Docker链重建成功\n")
			}
		}
	} else {
		log.Printf("[Firewall] DOCKER和DOCKER-USER链检查通过")
		if callback != nil {
			callback("✅ DOCKER和DOCKER-USER链检查通过\n")
		}
	}

	// Step 4: 检查目标端口是否由docker-proxy监听
	// 此结果用于决定后续iptables规则添加到INPUT链还是DOCKER-USER链
	portIsDockerProxy, _ := checkPortServiceIsDocker(client, port)

	if portIsDockerProxy {
		log.Printf("[Firewall] 端口%d由docker-proxy监听，规则将添加到DOCKER-USER链", port)
		if callback != nil {
			callback(fmt.Sprintf("📋 端口%d为Docker容器端口，规则将添加到DOCKER-USER链\n", port))
		}
	} else {
		log.Printf("[Firewall] 端口%d为普通服务端口，规则将添加到INPUT链", port)
		if callback != nil {
			callback(fmt.Sprintf("📋 端口%d为普通服务端口，规则将添加到INPUT链\n", port))
		}
	}

	return true, portIsDockerProxy, nil
}

// createIptablesRules 创建iptables规则（先检查规则是否已存在）
// 参数:
//   client - SSH客户端
//   port - 端口号
//   ipWhitelist - IP白名单（逗号分隔）
//   isDocker - 是否为Docker服务
//   callback - 进度回调函数
// 返回值:
//   error - 操作过程中的错误
// 非Docker环境（INPUT链）:
//   iptables -A INPUT -p tcp --dport {port} -m set --match-set {port}_allow_ip src -j ACCEPT
//   iptables -A INPUT -p tcp --dport {port} -j LOG --log-prefix "iptables-drop-{port}: "
//   iptables -A INPUT -p tcp --dport {port} -j DROP
// Docker环境（DOCKER-USER链）:
//   iptables -I DOCKER-USER 1 -p tcp --dport {port} -m set --match-set {port}_allow_ip src -j ACCEPT
//   iptables -I DOCKER-USER 2 -p tcp --dport {port} -j LOG --log-prefix "iptables-drop-{port}: "
//   iptables -I DOCKER-USER 3 -p tcp --dport {port} -j DROP
func createIptablesRules(client SSHClient, port int, ipWhitelist string, isDocker bool, callback func(message string)) error {
	log.Printf("[Firewall] 创建iptables规则，端口: %d, Docker环境: %v", port, isDocker)

	if callback != nil {
		callback("🔧 创建iptables规则...\n")
	}

	setName := fmt.Sprintf("%d_allow_ip", port)
	logPrefix := fmt.Sprintf("iptables-drop-%d: ", port)

	type ruleInfo struct {
		checkCmd  string
		addCmd    string
		ruleType  string
	}

	var rules []ruleInfo

	if !isDocker {
		log.Printf("[Firewall] 创建INPUT链规则")
		if callback != nil {
			callback("📋 创建INPUT链规则...\n")
		}

		rules = []ruleInfo{
			{
				checkCmd: fmt.Sprintf("iptables -C INPUT -p tcp --dport %d -m set --match-set %s src -j ACCEPT 2>/dev/null", port, setName),
				addCmd: fmt.Sprintf("iptables -A INPUT -p tcp --dport %d -m set --match-set %s src -j ACCEPT", port, setName),
				ruleType: "允许",
			},
			{
				checkCmd: fmt.Sprintf("iptables -C INPUT -p tcp --dport %d -j LOG --log-prefix \"%s\" 2>/dev/null", port, logPrefix),
				addCmd: fmt.Sprintf("iptables -A INPUT -p tcp --dport %d -j LOG --log-prefix \"%s\"", port, logPrefix),
				ruleType: "记录",
			},
			{
				checkCmd: fmt.Sprintf("iptables -C INPUT -p tcp --dport %d -j DROP 2>/dev/null", port),
				addCmd: fmt.Sprintf("iptables -A INPUT -p tcp --dport %d -j DROP", port),
				ruleType: "拒绝",
			},
		}
	} else {
		log.Printf("[Firewall] 创建DOCKER-USER链规则")
		if callback != nil {
			callback("📋 创建DOCKER-USER链规则...\n")
		}

		rules = []ruleInfo{
			{
				checkCmd: fmt.Sprintf("iptables -C DOCKER-USER -p tcp --dport %d -m set --match-set %s src -j ACCEPT 2>/dev/null", port, setName),
				addCmd: fmt.Sprintf("iptables -I DOCKER-USER 1 -p tcp --dport %d -m set --match-set %s src -j ACCEPT", port, setName),
				ruleType: "允许",
			},
			{
				checkCmd: fmt.Sprintf("iptables -C DOCKER-USER -p tcp --dport %d -j LOG --log-prefix \"%s\" 2>/dev/null", port, logPrefix),
				addCmd: fmt.Sprintf("iptables -I DOCKER-USER 2 -p tcp --dport %d -j LOG --log-prefix \"%s\"", port, logPrefix),
				ruleType: "记录",
			},
			{
				checkCmd: fmt.Sprintf("iptables -C DOCKER-USER -p tcp --dport %d -j DROP 2>/dev/null", port),
				addCmd: fmt.Sprintf("iptables -I DOCKER-USER 3 -p tcp --dport %d -j DROP", port),
				ruleType: "拒绝",
			},
		}
	}

	for _, rule := range rules {
		_, err := client.Execute(rule.checkCmd)
		if err != nil {
			log.Printf("[Firewall] 规则不存在，添加规则: %s", rule.addCmd)
			if callback != nil {
				callback(fmt.Sprintf("➕ 添加%s规则...\n", rule.ruleType))
			}
			_, err := client.Execute(rule.addCmd)
			if err != nil {
				log.Printf("[Firewall] 添加%s规则失败: %v", rule.ruleType, err)
				return fmt.Errorf("添加%s规则失败: %v", rule.ruleType, err)
			} else {
				log.Printf("[Firewall] %s规则添加成功", rule.ruleType)
				if callback != nil {
					callback(fmt.Sprintf("✅ %s规则添加成功\n", rule.ruleType))
				}
			}
		} else {
			log.Printf("[Firewall] %s规则已存在，跳过", rule.ruleType)
			if callback != nil {
				callback(fmt.Sprintf("⏭️ %s规则已存在，跳过\n", rule.ruleType))
			}
		}
	}

	if !isDocker {
		if callback != nil {
			callback(fmt.Sprintf("✅ INPUT链规则创建完成（端口%d）\n", port))
		}
	} else {
		if callback != nil {
			callback(fmt.Sprintf("✅ DOCKER-USER链规则创建完成（端口%d）\n", port))
		}
	}

	return nil
}

// configureLinux 执行Linux环境验证和防火墙配置前置检查
// 参数:
//   client - SSH客户端
//   server - 服务器信息
//   port - 要配置的端口
//   ipWhitelist - IP白名单
//   callback - 进度回调函数
// 返回值:
//   *ConfigResult - 配置结果
//   error - 配置过程中的错误
// 执行流程:
//   1. 检测发行版信息
//   2. 检查防火墙状态，停止并禁用系统防火墙（firewalld/ufw）
//   3. 检查iptables-services是否安装和启动
//      - 如果未安装，使用包管理器安装并启动
//      - 如果已安装但未启动，启动服务
//   4. 统一检查Docker环境及DOCKER链可用性
//      - 检查端口服务是否存在docker-proxy，判断是否为Docker环境
//      - 如果是Docker环境，检查DOCKER和DOCKER-USER链是否可用
//      - 如果链不可用，重启Docker服务以重建链
//      - 此步骤确保在配置规则之前Docker网络功能正常
//   5. 管理ipset白名单（创建集合、添加IP、保存配置）
//   6. 创建ipset自启动服务（系统启动时恢复ipset规则）
//   7. 创建iptables规则（根据Docker环境选择INPUT或DOCKER-USER链）
//   8. 保存iptables规则
func configureLinux(client SSHClient, server *models.Server, port int, ipWhitelist string, callback func(message string)) (*ConfigResult, error) {
	log.Printf("[Firewall] 开始执行Linux环境验证")

	if callback != nil {
		callback("🚀 开始执行Linux环境验证...\n")
	}

	var allOutput strings.Builder

	// Step 1: 检测发行版信息
	distroInfo, err := detectDistro(client)
	if err != nil {
		log.Printf("[Firewall] 发行版检测失败: %v", err)
		if callback != nil {
			callback(fmt.Sprintf("❌ 发行版检测失败: %v\n", err))
		}
		return &ConfigResult{
			Success: false,
			Command: "",
			Output:  err.Error(),
		}, nil
	}

	if callback != nil {
		callback(fmt.Sprintf("📋 检测到发行版: %s (包管理器: %s, 防火墙: %s)\n", distroInfo.Name, distroInfo.PackageManager, distroInfo.FirewallType))
	}

	// Step 2: 检查防火墙状态，如果正在运行则停止并禁用
	if distroInfo.FirewallType == "firewalld" {
		isRunning, err := checkFirewalldStatus(client)
		if err != nil {
			log.Printf("[Firewall] 检查firewalld状态失败: %v", err)
			return &ConfigResult{
				Success: false,
				Command: "",
				Output:  fmt.Sprintf("检查firewalld状态失败: %v", err),
			}, nil
		} else if isRunning {
			log.Printf("[Firewall] firewalld正在运行，执行停止并禁用")
			if callback != nil {
				callback("⏹️ firewalld正在运行，执行停止并禁用...")
			}
			if err := stopAndDisableFirewall(client, "firewalld", callback); err != nil {
				log.Printf("[Firewall] 停止并禁用firewalld失败: %v", err)
				return &ConfigResult{
					Success: false,
					Command: "",
					Output:  fmt.Sprintf("停止并禁用firewalld失败: %v", err),
				}, nil
			}
		} else {
			log.Printf("[Firewall] firewalld未运行")
			if callback != nil {
				callback("✅ firewalld未运行\n")
			}
		}
	} else if distroInfo.FirewallType == "ufw" {
		isRunning, err := checkUfwStatus(client)
		if err != nil {
			log.Printf("[Firewall] 检查ufw状态失败: %v", err)
			return &ConfigResult{
				Success: false,
				Command: "",
				Output:  fmt.Sprintf("检查ufw状态失败: %v", err),
			}, nil
		} else if isRunning {
			log.Printf("[Firewall] ufw正在运行，执行停止并禁用")
			if callback != nil {
				callback("⏹️ ufw正在运行，执行停止并禁用...")
			}
			if err := stopAndDisableFirewall(client, "ufw", callback); err != nil {
				log.Printf("[Firewall] 停止并禁用ufw失败: %v", err)
				return &ConfigResult{
					Success: false,
					Command: "",
					Output:  fmt.Sprintf("停止并禁用ufw失败: %v", err),
				}, nil
			}
		} else {
			log.Printf("[Firewall] ufw未运行")
			if callback != nil {
				callback("✅ ufw未运行\n")
			}
		}
	}

	// Step 3: 检查iptables服务是否安装和启动
	isInstalled, isActive, err := checkIptablesServicesInstalled(client, distroInfo.IptablesServiceName)
	if err != nil {
		log.Printf("[Firewall] 检查iptables服务失败: %v", err)
		return &ConfigResult{
			Success: false,
			Command: "",
			Output:  fmt.Sprintf("检查iptables服务失败: %v", err),
		}, nil
	}

	if !isInstalled {
		log.Printf("[Firewall] iptables服务未安装，开始安装")
		if callback != nil {
			callback(fmt.Sprintf("🔄 %s未安装，开始安装...\n", distroInfo.IptablesPackageName))
		}

		if err := installIptablesServices(client, distroInfo.PackageManager, distroInfo.IptablesPackageName, callback); err != nil {
			log.Printf("[Firewall] 安装iptables服务失败: %v", err)
			return &ConfigResult{
				Success: false,
				Command: "",
				Output:  err.Error(),
			}, nil
		}

		if err := startAndEnableIptablesServices(client, distroInfo.IptablesServiceName, callback); err != nil {
			log.Printf("[Firewall] 启动iptables服务失败: %v", err)
			return &ConfigResult{
				Success: false,
				Command: "",
				Output:  fmt.Sprintf("启动%s失败: %v", distroInfo.IptablesServiceName, err),
			}, nil
		}

		// iptables首次安装启动后，默认清空INPUT链原有规则
		// 背景: 新安装的iptables可能存在默认规则或之前残留的规则，
		// 需要清空INPUT链以确保后续只添加本工具管理的规则
		log.Printf("[Firewall] iptables首次安装启动完成，清空INPUT链")
		if callback != nil {
			callback("✅ 清空INPUT链...\n")
		}
		_, _ = client.Execute("iptables -F INPUT")
		if callback != nil {
			callback("✅ INPUT链已清空\n")
		}
	} else if !isActive {
		log.Printf("[Firewall] iptables服务已安装但未启动")
		if callback != nil {
			callback(fmt.Sprintf("🔄 %s已安装但未启动\n", distroInfo.IptablesServiceName))
		}

		if err := startAndEnableIptablesServices(client, distroInfo.IptablesServiceName, callback); err != nil {
			log.Printf("[Firewall] 启动iptables服务失败: %v", err)
			return &ConfigResult{
				Success: false,
				Command: "",
				Output:  fmt.Sprintf("启动%s失败: %v", distroInfo.IptablesServiceName, err),
			}, nil
		}
	} else {
		log.Printf("[Firewall] iptables服务已安装并运行")
		if callback != nil {
			callback(fmt.Sprintf("✅ %s已安装并运行\n", distroInfo.IptablesServiceName))
		}
	}

	// Step 4: 统一检查Docker环境及DOCKER链可用性
	// 重要: 此步骤在iptables服务检查通过之后立即执行，确保在配置任何规则之前
	// Docker环境的iptables链已经正确创建，防止后续配置破坏Docker网络功能
	// 背景: 当服务器先安装Docker服务，再安装iptables持久化服务时，
	// iptables服务的启动可能会清除Docker创建的DOCKER和DOCKER-USER链，
	// 导致Docker容器网络无法访问。通过重启Docker服务可以重建这些链。
	// 返回值:
	//   isDockerEnv - 服务器是否存在Docker运行环境（决定是否需要检查链）
	//   isDocker - 目标端口是否由docker-proxy监听（决定规则添加到哪个链）
	isDockerEnv, isDocker, err := ensureDockerChains(client, port, callback)
	if err != nil {
		log.Printf("[Firewall] Docker环境检查失败: %v", err)
		return &ConfigResult{
			Success: false,
			Command: "",
			Output:  fmt.Sprintf("Docker环境检查失败: %v", err),
		}, nil
	}

	// 如果服务器不是Docker环境，则按普通服务处理（端口不由docker-proxy监听）
	if !isDockerEnv {
		isDocker = false
	}

	// Step 5: 管理ipset白名单（先创建集合并保存到文件，再启动服务）
	// 白名单为空时，自动添加127.0.0.1到ipset白名单
	if ipWhitelist == "" {
		log.Printf("[Firewall] 白名单为空，自动添加127.0.0.1到ipset白名单")
		if callback != nil {
			callback("⚠️ 自动添加127.0.0.1到ipset白名单\n")
		}
		ipWhitelist = "127.0.0.1"
	}
	
	if err := manageIpset(client, port, ipWhitelist, callback); err != nil {
		log.Printf("[Firewall] 管理ipset白名单失败: %v", err)
		return &ConfigResult{
			Success: false,
			Command: "",
			Output:  err.Error(),
		}, nil
	}

	// Step 6: 创建ipset自启动服务
	if err := createIpsetRestoreService(client, callback); err != nil {
		log.Printf("[Firewall] 创建ipset自启动服务失败: %v", err)
		return &ConfigResult{
			Success: false,
			Command: "",
			Output:  fmt.Sprintf("创建ipset自启动服务失败: %v", err),
		}, nil
	}

	// Step 7: 创建iptables规则
	if err := createIptablesRules(client, port, ipWhitelist, isDocker, callback); err != nil {
		log.Printf("[Firewall] 创建iptables规则失败: %v", err)
		return &ConfigResult{
			Success: false,
			Command: "",
			Output:  err.Error(),
		}, nil
	}

	// Step 8: 保存iptables规则
	log.Printf("[Firewall] 保存iptables规则")
	if callback != nil {
		callback("💾 保存iptables规则...\n")
	}

	_, _ = client.Execute(fmt.Sprintf("%s 2>/dev/null || echo '规则保存完成'", distroInfo.IptablesSaveCommand))
	if callback != nil {
		callback("✅ iptables规则保存完成\n")
	}

	log.Printf("[Firewall] Linux防火墙配置完成")
	if callback != nil {
		callback("🎉 防火墙配置完成！\n")
	}

	return &ConfigResult{
		Success: true,
		Command: "",
		Output:  allOutput.String(),
	}, nil
}

// deconfigureLinux 取消配置Linux防火墙
// 参数:
//   client - SSH客户端
//   port - 要取消配置的端口
//   ipWhitelist - IP白名单（空表示完整取消，非空表示只删除指定IP白名单）
//   saveCommand - iptables规则保存命令
//   callback - 进度回调函数
// 返回值:
//   error - 操作过程中的错误
// 执行流程:
//   先执行环境验证:
//     1. 检测发行版信息
//     2. 检查防火墙状态
//     3. 检查iptables服务状态
//     4. 统一检查Docker环境及DOCKER链可用性
//        - 确保在取消配置前Docker网络环境正常
//        - 如果Docker链不可用，重启Docker服务重建链
//     5. 检查ipset集合
//     6. 显示Docker环境检测结果
//     7. 检查对应链规则是否存在
//   
//   情况1: 有白名单（填写了IP）- 只删除指定IP白名单
//     1. 判断IP是否在集合中，如果存在则删除，不存在则跳过
//     2. 保存ipset配置
//   
//   情况2: 无白名单（空）- 完整取消配置
//     1. 判断是否有对应端口的规则，如果存在则删除规则（INPUT链和DOCKER-USER链）
//     2. 判断是否有对应端口的ipset集合，如果存在则删除集合
//     3. 保存iptables规则
func deconfigureLinux(client SSHClient, port int, ipWhitelist string, saveCommand string, callback func(message string)) error {
	log.Printf("[Firewall] 开始取消配置Linux防火墙，端口: %d, 白名单: %s", port, ipWhitelist)

	if callback != nil {
		callback("🚀 开始执行Linux环境验证...\n")
	}

	// Step 1: 检测发行版信息
	distroInfo, err := detectDistro(client)
	if err != nil {
		log.Printf("[Firewall] 发行版检测失败: %v", err)
		return fmt.Errorf("发行版检测失败: %v", err)
	}

	if callback != nil {
		callback(fmt.Sprintf("📋 检测到发行版: %s (包管理器: %s, 防火墙: %s)\n", distroInfo.Name, distroInfo.PackageManager, distroInfo.FirewallType))
	}

	// Step 2: 检查防火墙状态
	if distroInfo.FirewallType == "firewalld" {
		isRunning, err := checkFirewalldStatus(client)
		if err != nil {
			log.Printf("[Firewall] 检查firewalld状态失败: %v", err)
			return fmt.Errorf("检查firewalld状态失败: %v", err)
		} else if isRunning {
			log.Printf("[Firewall] firewalld正在运行")
			if callback != nil {
				callback("⏹️ firewalld正在运行\n")
			}
		} else {
			log.Printf("[Firewall] firewalld未运行")
			if callback != nil {
				callback("✅ firewalld未运行\n")
			}
		}
	} else if distroInfo.FirewallType == "ufw" {
		isRunning, err := checkUfwStatus(client)
		if err != nil {
			log.Printf("[Firewall] 检查ufw状态失败: %v", err)
			return fmt.Errorf("检查ufw状态失败: %v", err)
		} else if isRunning {
			log.Printf("[Firewall] ufw正在运行")
			if callback != nil {
				callback("⏹️ ufw正在运行\n")
			}
		} else {
			log.Printf("[Firewall] ufw未运行")
			if callback != nil {
				callback("✅ ufw未运行\n")
			}
		}
	}

	// Step 3: 检查iptables服务状态
	isInstalled, isActive, err := checkIptablesServicesInstalled(client, distroInfo.IptablesServiceName)
	if err != nil {
		log.Printf("[Firewall] 检查iptables服务失败: %v", err)
		return fmt.Errorf("检查iptables服务失败: %v", err)
	}

	if isInstalled && isActive {
		log.Printf("[Firewall] iptables服务已安装并运行")
		if callback != nil {
			callback(fmt.Sprintf("✅ %s已安装并运行\n", distroInfo.IptablesServiceName))
		}
	} else if isInstalled && !isActive {
		log.Printf("[Firewall] iptables服务已安装但未启动")
		if callback != nil {
			callback(fmt.Sprintf("🔄 %s已安装但未启动\n", distroInfo.IptablesServiceName))
		}
	} else {
		log.Printf("[Firewall] iptables服务未安装")
		if callback != nil {
			callback(fmt.Sprintf("❌ %s未安装\n", distroInfo.IptablesServiceName))
		}
		return fmt.Errorf("%s未安装", distroInfo.IptablesServiceName)
	}

	// Step 4: 统一检查Docker环境及DOCKER链可用性
	// 与configureLinux保持一致，在取消配置前也需要确保Docker链可用
	// 防止取消配置操作破坏Docker网络环境
	// 返回值:
	//   isDockerEnv - 服务器是否存在Docker运行环境
	//   isDocker - 目标端口是否由docker-proxy监听（决定规则删除在哪个链）
	isDockerEnv, isDocker, err := ensureDockerChains(client, port, callback)
	if err != nil {
		log.Printf("[Firewall] Docker环境检查失败: %v", err)
		return fmt.Errorf("Docker环境检查失败: %v", err)
	}

	// 如果服务器不是Docker环境，则按普通服务处理
	if !isDockerEnv {
		isDocker = false
	}

	setName := fmt.Sprintf("%d_allow_ip", port)
	logPrefix := fmt.Sprintf("iptables-drop-%d:", port)

	// Step 5: 检查ipset集合
	output, err := client.Execute(fmt.Sprintf(`ipset list %s 2>/dev/null || echo 'not_found'`, setName))
	if err != nil || strings.Contains(strings.ToLower(output), "not_found") || !strings.Contains(output, setName) {
		log.Printf("[Firewall] ipset集合%s不存在", setName)
		if callback != nil {
			callback(fmt.Sprintf("❌ ipset集合%s不存在\n", setName))
		}
		if ipWhitelist != "" {
			return fmt.Errorf("ipset集合%s不存在，无法删除白名单", setName)
		}
		return nil
	}

	log.Printf("[Firewall] ipset集合%s存在", setName)
	if callback != nil {
		callback(fmt.Sprintf("✅ ipset集合%s存在\n", setName))
	}

	// Step 6: 显示Docker环境检测结果（已在Step 4中完成检测）
	if isDocker {
		log.Printf("[Firewall] 端口%d服务为Docker服务", port)
		if callback != nil {
			callback(fmt.Sprintf("🐳 端口%d服务为Docker服务\n", port))
		}
	} else {
		log.Printf("[Firewall] 端口%d服务为普通服务", port)
		if callback != nil {
			callback(fmt.Sprintf("🖥️ 端口%d服务为普通服务\n", port))
		}
	}

	// Step 7: 检查对应链规则是否存在
	var chainName string
	if isDocker {
		chainName = "DOCKER-USER"
	} else {
		chainName = "INPUT"
	}

	checkCmd := fmt.Sprintf("iptables -C %s -p tcp --dport %d -m set --match-set %s src -j ACCEPT 2>/dev/null", chainName, port, setName)
	_, err = client.Execute(checkCmd)
	if err != nil {
		log.Printf("[Firewall] 端口%d的%s链规则不存在", port, chainName)
		if callback != nil {
			callback(fmt.Sprintf("⏭️ 端口%d的%s链规则不存在\n", port, chainName))
		}
	} else {
		log.Printf("[Firewall] 端口%d的%s链规则存在", port, chainName)
		if callback != nil {
			callback(fmt.Sprintf("✅ 端口%d的%s链规则存在\n", port, chainName))
		}
	}

	if ipWhitelist != "" {
		log.Printf("[Firewall] 有白名单，只删除指定IP")
		if callback != nil {
			callback("📋 有白名单，只删除指定IP...\n")
		}

		ips := strings.Split(ipWhitelist, ",")
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			if ip == "" {
				continue
			}

			output, err := client.Execute(fmt.Sprintf(`ipset test %s %s 2>/dev/null`, setName, ip))
			if err != nil || strings.Contains(strings.ToLower(output), "not in set") {
				log.Printf("[Firewall] IP%s不存在于集合%s中，跳过", ip, setName)
				if callback != nil {
					callback(fmt.Sprintf("⏭️ IP%s不存在，跳过\n", ip))
				}
				continue
			}

			log.Printf("[Firewall] 删除IP%s", ip)
			if callback != nil {
				callback(fmt.Sprintf("➖ 删除IP%s...\n", ip))
			}

			_, err = client.Execute(fmt.Sprintf("ipset del %s %s 2>/dev/null", setName, ip))
			if err != nil {
				log.Printf("[Firewall] 删除IP%s失败: %v", ip, err)
				if callback != nil {
					callback(fmt.Sprintf("❌ 删除IP%s失败: %v\n", ip, err))
				}
				return fmt.Errorf("删除IP%s失败: %v", ip, err)
			}
			log.Printf("[Firewall] IP%s删除成功", ip)
			if callback != nil {
				callback(fmt.Sprintf("✅ IP%s删除成功\n", ip))
			}
		}

		_, _ = client.Execute("ipset save > /etc/ipset.conf")
		if callback != nil {
			callback("✅ ipset配置已保存\n")
		}
	} else {
		log.Printf("[Firewall] 无白名单，完整取消配置")
		if callback != nil {
			callback("📋 无白名单，完整取消配置...\n")
		}

		// 检查规则是否存在（检查ACCEPT规则即可）
		checkCmd := fmt.Sprintf("iptables -C %s -p tcp --dport %d -m set --match-set %s src -j ACCEPT 2>/dev/null", chainName, port, setName)
		_, err = client.Execute(checkCmd)
		if err != nil {
			log.Printf("[Firewall] 端口%d的%s链规则不存在", port, chainName)
			if callback != nil {
				callback(fmt.Sprintf("⏭️ 端口%d的%s链规则不存在\n", port, chainName))
			}
		} else {
			log.Printf("[Firewall] 删除%s链规则（顺序：拒绝→记录→允许）", chainName)
			if callback != nil {
				callback(fmt.Sprintf("🔧 删除%s链规则...\n", chainName))
			}

			// 删除顺序：先删除DROP，再删除LOG，最后删除ACCEPT
			// 这样可以避免删除过程中导致SSH连接断开
			delCommands := []struct {
				cmd      string
				ruleType string
			}{
				{
					cmd:      fmt.Sprintf("iptables -D %s -p tcp --dport %d -j DROP 2>/dev/null", chainName, port),
					ruleType: "拒绝",
				},
				{
					cmd:      fmt.Sprintf("iptables -D %s -p tcp --dport %d -j LOG --log-prefix \"%s \" 2>/dev/null", chainName, port, logPrefix),
					ruleType: "记录",
				},
				{
					cmd:      fmt.Sprintf("iptables -D %s -p tcp --dport %d -m set --match-set %s src -j ACCEPT 2>/dev/null", chainName, port, setName),
					ruleType: "允许",
				},
			}

			for _, dc := range delCommands {
				_, err := client.Execute(dc.cmd)
				if err != nil {
					log.Printf("[Firewall] 删除%s规则失败: %v", dc.ruleType, err)
					if callback != nil {
						callback(fmt.Sprintf("❌ 删除%s规则失败: %v\n", dc.ruleType, err))
					}
					return fmt.Errorf("删除%s规则失败: %v", dc.ruleType, err)
				}
				log.Printf("[Firewall] %s规则删除成功", dc.ruleType)
				if callback != nil {
					callback(fmt.Sprintf("✅ %s规则删除成功\n", dc.ruleType))
				}
			}

			if callback != nil {
				callback(fmt.Sprintf("✅ %s链规则删除完成\n", chainName))
			}
		}

		// 删除ipset集合
		output, err := client.Execute(fmt.Sprintf(`ipset list %s 2>/dev/null || echo 'not_found'`, setName))
		if err == nil && !strings.Contains(strings.ToLower(output), "not_found") && strings.Contains(output, setName) {
			log.Printf("[Firewall] 删除ipset集合%s", setName)
			if callback != nil {
				callback(fmt.Sprintf("🔧 删除ipset集合%s...\n", setName))
			}

			_, err := client.Execute(fmt.Sprintf("ipset destroy %s 2>/dev/null", setName))
			if err != nil {
				log.Printf("[Firewall] 删除ipset集合失败: %v", err)
				if callback != nil {
					callback(fmt.Sprintf("❌ 删除ipset集合失败: %v\n", err))
				}
				return fmt.Errorf("删除ipset集合失败: %v", err)
			}
			log.Printf("[Firewall] ipset集合%s删除成功", setName)
			if callback != nil {
				callback(fmt.Sprintf("✅ ipset集合%s删除成功\n", setName))
			}
		} else {
			log.Printf("[Firewall] ipset集合%s不存在，跳过", setName)
			if callback != nil {
				callback(fmt.Sprintf("⏭️ ipset集合%s不存在，跳过\n", setName))
			}
		}

		_, _ = client.Execute(fmt.Sprintf("%s 2>/dev/null", saveCommand))
		if callback != nil {
			callback("✅ iptables规则保存完成\n")
		}
	}

	log.Printf("[Firewall] Linux防火墙取消配置完成")
	if callback != nil {
		callback(fmt.Sprintf("🎉 端口%d防火墙取消配置完成！\n", port))
	}

	return nil
}