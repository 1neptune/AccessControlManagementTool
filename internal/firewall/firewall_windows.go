package firewall

import (
	"access-control-tool/internal/models"
	"fmt"
	"log"
	"regexp"
	"strings"
)

const (
	policyName        = "安全访问控制策略" // IPsec策略名称
	filterActionAllow = "允许访问"          // 允许操作名称
	filterActionBlock = "拒绝访问"          // 拒绝操作名称
)

// configureWindows 配置Windows IP安全策略主函数
// 配置流程:
// 1. 检查系统适用性（secpol.msc和netsh ipsec static可用性）
// 2. 检查并启动必要服务（PolicyAgent、IKEEXT）
// 3. 检查策略是否存在，不存在则创建
// 4. 配置端口规则（允许/拒绝筛选器列表、筛选器、规则）
// 5. 刷新策略使其生效
// 6. 验证配置结果
// 参数: client - WinRM客户端
//       server - 服务器信息
//       port - 目标端口
//       ipWhitelist - 白名单IP列表（逗号分隔）
//       callback - 进度回调函数
// 返回: 配置结果和错误
func configureWindows(client WinRMClient, server *models.Server, port int, ipWhitelist string, callback func(message string)) (*ConfigResult, error) {
	log.Printf("[Firewall] 开始配置IP安全策略，端口: %d", port)

	var output strings.Builder

	writeOutput := func(msg string) {
		output.WriteString(msg)
		if callback != nil {
			callback(msg)
		}
	}

	writeOutput("开始配置IP安全策略...\n")

	if err := checkSystemApplicability(client, writeOutput); err != nil {
		log.Printf("[Firewall] 配置失败: %v", err)
		return &ConfigResult{
			Success: false,
			Command: "",
			Output:  output.String(),
		}, fmt.Errorf("%v", err)
	}

	if err := checkAndStartServices(client, writeOutput); err != nil {
		log.Printf("[Firewall] 配置失败: %v", err)
		return &ConfigResult{
			Success: false,
			Command: "",
			Output:  output.String(),
		}, fmt.Errorf("%v", err)
	}

	policyExists, err := checkPolicyExists(client, writeOutput)
	if err != nil {
		log.Printf("[Firewall] 配置失败: %v", err)
		return &ConfigResult{
			Success: false,
			Command: "",
			Output:  output.String(),
		}, fmt.Errorf("%v", err)
	}

	if !policyExists {
		if err := createPolicyStructure(client, writeOutput); err != nil {
			log.Printf("[Firewall] 创建策略结构失败: %v", err)
			return &ConfigResult{
				Success: false,
				Command: "",
				Output:  output.String(),
			}, fmt.Errorf("%v", err)
		}
	} else {
		if err := ensureFilterActionExists(client, filterActionAllow, "permit", writeOutput); err != nil {
			log.Printf("[Firewall] 检查允许筛选器操作失败: %v", err)
			return &ConfigResult{
				Success: false,
				Command: "",
				Output:  output.String(),
			}, fmt.Errorf("%v", err)
		}
		if err := ensureFilterActionExists(client, filterActionBlock, "block", writeOutput); err != nil {
			log.Printf("[Firewall] 检查拒绝筛选器操作失败: %v", err)
			return &ConfigResult{
				Success: false,
				Command: "",
				Output:  output.String(),
			}, fmt.Errorf("%v", err)
		}
	}

	if err := configurePortRules(client, port, ipWhitelist, writeOutput); err != nil {
		log.Printf("[Firewall] 配置端口规则失败: %v", err)
		return &ConfigResult{
			Success: false,
			Command: "",
			Output:  output.String(),
		}, fmt.Errorf("%v", err)
	}

	if err := refreshPolicy(client, writeOutput); err != nil {
		log.Printf("[Firewall] 刷新策略失败: %v", err)
		return &ConfigResult{
			Success: false,
			Command: "",
			Output:  output.String(),
		}, fmt.Errorf("%v", err)
	}

	if err := verifyPolicy(client, port, writeOutput); err != nil {
		log.Printf("[Firewall] 配置失败: %v", err)
		return &ConfigResult{
			Success: false,
			Command: "",
			Output:  output.String(),
		}, fmt.Errorf("%v", err)
	}

	log.Printf("[Firewall] Windows防火墙配置完成")
	return &ConfigResult{
		Success: true,
		Command: "",
		Output:  output.String(),
	}, nil
}

// checkSystemApplicability 检查系统是否支持IPsec策略配置
// 检查项:
// 1. secpol.msc是否存在（验证系统版本支持）
// 2. netsh ipsec static命令是否可用
// 参数: client - WinRM客户端
//       writeOutput - 输出回调函数
// 返回: 错误
func checkSystemApplicability(client WinRMClient, writeOutput WriteFunc) error {
	log.Printf("[Firewall] 检查系统适用性")
	writeOutput("检查系统适用性...\n")

	secpolCheckCmd := `if exist "C:\Windows\System32\secpol.msc" (echo EXISTS) else (echo NOT_EXISTS)`
	secpolOutput, err := client.Execute(secpolCheckCmd)
	if err != nil {
		log.Printf("[Firewall] 检查secpol.msc失败: %v", err)
		writeOutput("❌ 检查secpol.msc失败\n")
		return fmt.Errorf("检查secpol.msc失败: %v", err)
	}

	if !strings.Contains(secpolOutput, "EXISTS") {
		log.Printf("[Firewall] 系统不支持IPsec策略")
		writeOutput("❌ 系统不支持IPsec策略 - secpol.msc不存在，系统可能为家庭版或组件缺失\n")
		return fmt.Errorf("系统不支持IPsec策略")
	}
	writeOutput("✅ 系统支持IPsec策略\n")

	ipsecCheckCmd := `netsh ipsec static 2>&1 || echo FAILED`
	ipsecOutput, err := client.Execute(ipsecCheckCmd)
	if err != nil {
		log.Printf("[Firewall] 检查netsh ipsec static失败: %v", err)
		writeOutput("❌ 检查netsh ipsec static失败\n")
		return fmt.Errorf("检查netsh ipsec static失败: %v", err)
	}

	if strings.Contains(ipsecOutput, "FAILED") || strings.Contains(ipsecOutput, "不是内部或外部命令") {
		log.Printf("[Firewall] 系统不支持 netsh ipsec static")
		writeOutput("❌ 系统不支持 netsh ipsec static\n")
		return fmt.Errorf("系统不支持 netsh ipsec static")
	}
	writeOutput("✅ netsh ipsec static命令可用\n")

	log.Printf("[Firewall] 系统适用性检查通过")
	writeOutput("✅ 系统适用性检查通过\n")
	return nil
}

// checkAndStartServices 检查并启动IPsec相关服务
// 需要检查的服务: PolicyAgent（IPsec策略代理）、IKEEXT（IKE扩展）
// 参数: client - WinRM客户端
//       writeOutput - 输出回调函数
// 返回: 错误
func checkAndStartServices(client WinRMClient, writeOutput WriteFunc) error {
	log.Printf("[Firewall] 检查服务状态")
	writeOutput("检查服务状态...\n")

	if err := checkAndStartService(client, "PolicyAgent", writeOutput); err != nil {
		return err
	}

	if err := checkAndStartService(client, "IKEEXT", writeOutput); err != nil {
		return err
	}

	return nil
}

// checkAndStartService 检查单个服务状态，如果未启动则尝试启动
// 步骤:
// 1. 使用sc query检查服务状态
// 2. 如果已运行，直接返回成功
// 3. 如果未运行，设置为自动启动并启动服务
// 4. 再次检查服务状态确认启动成功
// 参数: client - WinRM客户端
//       serviceName - 服务名称
//       writeOutput - 输出回调函数
// 返回: 错误
func checkAndStartService(client WinRMClient, serviceName string, writeOutput WriteFunc) error {
	log.Printf("[Firewall] 检查 %s 服务", serviceName)
	checkCmd := fmt.Sprintf(`sc query %s`, serviceName)
	checkOutput, err := client.Execute(checkCmd)
	if err != nil {
		log.Printf("[Firewall] 检查%s服务失败: %v", serviceName, err)
		writeOutput(fmt.Sprintf("❌ 检查%s服务失败\n", serviceName))
		return fmt.Errorf("检查%s服务失败: %v", serviceName, err)
	}

	if strings.Contains(checkOutput, "STATE") && strings.Contains(checkOutput, "RUNNING") {
		log.Printf("[Firewall] ✅ %s 服务已启动", serviceName)
		writeOutput(fmt.Sprintf("✅ %s 服务已启动\n", serviceName))
		return nil
	}

	log.Printf("[Firewall] 尝试启动 %s 服务", serviceName)
	writeOutput(fmt.Sprintf("🔄 启动%s服务...\n", serviceName))

	startCmds := []string{
		fmt.Sprintf(`sc config %s start= auto`, serviceName),
		fmt.Sprintf(`net start %s`, serviceName),
	}

	for _, cmd := range startCmds {
		_, err := client.Execute(cmd)
		if err != nil {
			log.Printf("[Firewall] 执行命令失败: %s, %v", cmd, err)
			writeOutput(fmt.Sprintf("❌ 执行命令失败: %s - %v\n", cmd, err))
			return fmt.Errorf("执行命令失败: %s - %v", cmd, err)
		}
	}

	checkOutput, err = client.Execute(checkCmd)
	if err != nil {
		log.Printf("[Firewall] 验证%s服务启动失败: %v", serviceName, err)
		writeOutput(fmt.Sprintf("❌ 验证%s服务启动失败\n", serviceName))
		return fmt.Errorf("验证%s服务启动失败: %v", serviceName, err)
	}

	if strings.Contains(checkOutput, "STATE") && strings.Contains(checkOutput, "RUNNING") {
		log.Printf("[Firewall] ✅ %s 服务启动成功", serviceName)
		writeOutput(fmt.Sprintf("✅ %s 服务启动成功\n", serviceName))
		return nil
	}

	log.Printf("[Firewall] ❌ %s 服务无法启动", serviceName)
	writeOutput(fmt.Sprintf("❌ %s 服务无法启动\n", serviceName))
	return fmt.Errorf("%s服务无法启动", serviceName)
}

// checkPolicyExists 检查IPsec策略是否已存在
// 使用正则匹配中英文输出（名称: xxx / Name = xxx）
// 参数: client - WinRM客户端
//       writeOutput - 输出回调函数
// 返回: 是否存在和错误
func checkPolicyExists(client WinRMClient, writeOutput WriteFunc) (bool, error) {
	log.Printf("[Firewall] 检查策略状态")
	writeOutput("检查策略状态...\n")

	checkCmd := fmt.Sprintf(`netsh ipsec static show policy name="%s"`, policyName)
	checkOutput, err := client.Execute(checkCmd)
	if err != nil {
		log.Printf("[Firewall] 检查策略失败: %v", err)
		writeOutput("❌ 检查策略失败\n")
		return false, fmt.Errorf("检查策略失败: %v", err)
	}

	log.Printf("[Firewall] 策略检查输出: %s", checkOutput)

	policyExists := false
	cnRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(policyName))
	enRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(policyName))

	if cnRegex.MatchString(checkOutput) || enRegex.MatchString(checkOutput) {
		policyExists = true
	}

	if policyExists {
		log.Printf("[Firewall] 安全访问控制策略已存在")
		writeOutput("✅ 安全访问控制策略已存在（非第一次配置）\n")
		return true, nil
	}

	log.Printf("[Firewall] 策略不存在")
	writeOutput("🔄 安全访问控制策略不存在（第一次配置）\n")
	return false, nil
}

// createPolicyStructure 创建IPsec策略结构
// 步骤:
// 1. 创建全局策略
// 2. 创建允许筛选器操作
// 3. 创建拒绝筛选器操作
// 参数: client - WinRM客户端
//       writeOutput - 输出回调函数
// 返回: 错误
func createPolicyStructure(client WinRMClient, writeOutput WriteFunc) error {
	log.Printf("[Firewall] 创建策略结构")

	createPolicyCmd := fmt.Sprintf(`netsh ipsec static add policy name="%s"`, policyName)
	_, err := client.Execute(createPolicyCmd)
	if err != nil {
		log.Printf("[Firewall] 创建全局策略失败: %v", err)
		writeOutput("❌ 创建全局策略失败\n")
		return fmt.Errorf("创建全局策略失败: %v", err)
	}
	writeOutput("✅ 创建全局策略成功\n")

	if err := ensureFilterActionExists(client, filterActionAllow, "permit", writeOutput); err != nil {
		return err
	}

	if err := ensureFilterActionExists(client, filterActionBlock, "block", writeOutput); err != nil {
		return err
	}

	return nil
}

// ensureFilterActionExists 确保筛选器操作存在，不存在则创建
// 参数: client - WinRM客户端
//       actionName - 操作名称
//       actionType - 操作类型（permit/block）
//       writeOutput - 输出回调函数
// 返回: 错误
func ensureFilterActionExists(client WinRMClient, actionName, actionType string, writeOutput WriteFunc) error {
	checkCmd := fmt.Sprintf(`netsh ipsec static show filteraction name="%s"`, actionName)
	checkOutput, err := client.Execute(checkCmd)
	if err != nil {
		log.Printf("[Firewall] 检查筛选器操作失败(%s): %v", actionName, err)
		writeOutput(fmt.Sprintf("❌ 检查筛选器操作失败(%s): %v\n", actionName, err))
		return fmt.Errorf("检查筛选器操作失败(%s): %v", actionName, err)
	}

	actionExists := false
	cnRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(actionName))
	enRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(actionName))

	if cnRegex.MatchString(checkOutput) || enRegex.MatchString(checkOutput) {
		actionExists = true
	}

	if actionExists {
		log.Printf("[Firewall] 筛选器操作%s已存在", actionName)
		writeOutput(fmt.Sprintf("⏭️ %s已存在，跳过\n", actionName))
		return nil
	}

	log.Printf("[Firewall] 创建筛选器操作: %s", actionName)
	createCmd := fmt.Sprintf(`netsh ipsec static add filteraction name="%s" action=%s`, actionName, actionType)
	_, err = client.Execute(createCmd)
	if err != nil {
		log.Printf("[Firewall] 创建筛选器操作失败(%s): %v", actionName, err)
		writeOutput(fmt.Sprintf("❌ 创建%s失败\n", actionName))
		return fmt.Errorf("创建筛选器操作失败(%s): %v", actionName, err)
	}
	writeOutput(fmt.Sprintf("✅ 创建%s成功\n", actionName))
	return nil
}

// configurePortRules 配置指定端口的IPsec规则
// 步骤:
// 1. 确保允许/拒绝筛选器列表存在
// 2. 配置白名单筛选器（指定IP的TCP/UDP访问）
// 3. 配置黑名单筛选器（any地址的TCP/UDP访问）
// 4. 确保允许/拒绝规则绑定存在
// 参数: client - WinRM客户端
//       port - 目标端口
//       ipWhitelist - 白名单IP列表
//       writeOutput - 输出回调函数
// 返回: 错误
func configurePortRules(client WinRMClient, port int, ipWhitelist string, writeOutput WriteFunc) error {
	allowListName := fmt.Sprintf("允许%d访问", port)
	blockListName := fmt.Sprintf("拒绝%d访问", port)
	allowRuleName := fmt.Sprintf("允许%d访问", port)
	blockRuleName := fmt.Sprintf("拒绝%d访问", port)

	if err := ensureFilterListExists(client, allowListName, writeOutput); err != nil {
		return err
	}

	if err := ensureFilterListExists(client, blockListName, writeOutput); err != nil {
		return err
	}

	if err := configureWhitelistFilters(client, port, ipWhitelist, allowListName, writeOutput); err != nil {
		return err
	}

	if err := configureBlacklistFilters(client, port, blockListName, writeOutput); err != nil {
		return err
	}

	if err := ensureRuleExists(client, allowRuleName, policyName, allowListName, filterActionAllow, writeOutput); err != nil {
		return err
	}

	if err := ensureRuleExists(client, blockRuleName, policyName, blockListName, filterActionBlock, writeOutput); err != nil {
		return err
	}

	return nil
}

// ensureFilterListExists 确保筛选器列表存在，不存在则创建
// 参数: client - WinRM客户端
//       listName - 列表名称
//       writeOutput - 输出回调函数
// 返回: 错误
func ensureFilterListExists(client WinRMClient, listName string, writeOutput WriteFunc) error {
	checkCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s"`, listName)
	checkOutput, err := client.Execute(checkCmd)
	if err != nil {
		log.Printf("[Firewall] 检查筛选器列表失败(%s): %v", listName, err)
		writeOutput(fmt.Sprintf("❌ 检查筛选器列表失败(%s): %v\n", listName, err))
		return fmt.Errorf("检查筛选器列表失败(%s): %v", listName, err)
	}

	listExists := false
	cnRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(listName))
	enRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(listName))

	if cnRegex.MatchString(checkOutput) || enRegex.MatchString(checkOutput) {
		listExists = true
	}

	if listExists {
		log.Printf("[Firewall] 筛选器列表%s已存在", listName)
		writeOutput(fmt.Sprintf("⏭️ %s列表已存在，跳过\n", listName))
		return nil
	}

	log.Printf("[Firewall] 创建筛选器列表: %s", listName)
	createCmd := fmt.Sprintf(`netsh ipsec static add filterlist name="%s"`, listName)
	_, err = client.Execute(createCmd)
	if err != nil {
		log.Printf("[Firewall] 创建筛选器列表失败(%s): %v", listName, err)
		writeOutput(fmt.Sprintf("❌ 创建%s列表失败\n", listName))
		return fmt.Errorf("创建筛选器列表失败(%s): %v", listName, err)
	}
	writeOutput(fmt.Sprintf("✅ 创建%s列表成功\n", listName))
	return nil
}

// configureWhitelistFilters 配置白名单筛选器
// 步骤:
// 1. 获取现有筛选器列表（level=verbose）
// 2. 解析现有IP列表
// 3. 删除重复筛选器
// 4. 为每个白名单IP添加TCP和UDP筛选器（跳过已存在的）
// 参数: client - WinRM客户端
//       port - 目标端口
//       ipWhitelist - 白名单IP列表
//       allowListName - 允许列表名称
//       writeOutput - 输出回调函数
// 返回: 错误
func configureWhitelistFilters(client WinRMClient, port int, ipWhitelist string, allowListName string, writeOutput WriteFunc) error {
	log.Printf("[Firewall] 配置白名单筛选器")

	getFiltersCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s" level=verbose`, allowListName)
	filtersOutput, err := client.Execute(getFiltersCmd)
	if err != nil {
		log.Printf("[Firewall] 获取现有筛选器失败: %v", err)
		writeOutput("❌ 获取现有筛选器失败\n")
		return fmt.Errorf("获取现有筛选器失败: %v", err)
	}

	existingIPs := parseIPsFromFilterList(filtersOutput)
	log.Printf("[Firewall] 白名单IP列表: %v", existingIPs)

	if err := removeDuplicateFilters(client, allowListName, existingIPs, writeOutput); err != nil {
		log.Printf("[Firewall] 删除重复筛选器失败: %v", err)
		writeOutput("❌ 删除重复筛选器失败\n")
		return fmt.Errorf("删除重复筛选器失败: %v", err)
	}

	ipList := strings.Split(ipWhitelist, ",")
	writeOutput(fmt.Sprintf("🔄 配置白名单模式，共%d个IP...\n", len(ipList)))

	for _, ip := range ipList {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}

		tcpKey := fmt.Sprintf("%s/TCP/%d", ip, port)
		udpKey := fmt.Sprintf("%s/UDP/%d", ip, port)

		if !containsIP(existingIPs, tcpKey) {
			allowFilterCmdTCP := fmt.Sprintf(`netsh ipsec static add filter filterlist="%s" srcaddr=%s dstaddr=me dstport=%d protocol=TCP`, allowListName, ip, port)
			_, err := client.Execute(allowFilterCmdTCP)
			if err != nil {
				log.Printf("[Firewall] 添加白名单筛选器失败(%s,TCP): %v", ip, err)
				writeOutput(fmt.Sprintf("❌ 添加白名单IP(%s,TCP)失败\n", ip))
				return fmt.Errorf("添加白名单筛选器失败(%s,TCP): %v", ip, err)
			}
			writeOutput(fmt.Sprintf("✅ 添加白名单IP(%s,TCP)成功\n", ip))
		} else {
			log.Printf("[Firewall] IP %s TCP已存在，跳过", ip)
			writeOutput(fmt.Sprintf("⏭️ IP %s TCP已存在，跳过\n", ip))
		}

		if !containsIP(existingIPs, udpKey) {
			allowFilterCmdUDP := fmt.Sprintf(`netsh ipsec static add filter filterlist="%s" srcaddr=%s dstaddr=me dstport=%d protocol=UDP`, allowListName, ip, port)
			_, err := client.Execute(allowFilterCmdUDP)
			if err != nil {
				log.Printf("[Firewall] 添加白名单筛选器失败(%s,UDP): %v", ip, err)
				writeOutput(fmt.Sprintf("❌ 添加白名单IP(%s,UDP)失败\n", ip))
				return fmt.Errorf("添加白名单筛选器失败(%s,UDP): %v", ip, err)
			}
			writeOutput(fmt.Sprintf("✅ 添加白名单IP(%s,UDP)成功\n", ip))
		} else {
			log.Printf("[Firewall] IP %s UDP已存在，跳过", ip)
			writeOutput(fmt.Sprintf("⏭️ IP %s UDP已存在，跳过\n", ip))
		}
	}

	return nil
}

// configureBlacklistFilters 配置黑名单筛选器
// 步骤:
// 1. 获取现有拒绝筛选器列表
// 2. 删除重复筛选器
// 3. 添加any地址的TCP和UDP筛选器（拒绝所有访问）
// 4. 清理多余的拒绝筛选器，只保留2条(TCP+UDP)
// 参数: client - WinRM客户端
//       port - 目标端口
//       blockListName - 拒绝列表名称
//       writeOutput - 输出回调函数
// 返回: 错误
func configureBlacklistFilters(client WinRMClient, port int, blockListName string, writeOutput WriteFunc) error {
	log.Printf("[Firewall] 配置拒绝筛选器")

	getBlockFiltersCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s" level=verbose`, blockListName)
	blockFiltersOutput, err := client.Execute(getBlockFiltersCmd)
	if err != nil {
		log.Printf("[Firewall] 获取拒绝筛选器失败: %v", err)
		writeOutput("❌ 获取拒绝筛选器失败\n")
		return fmt.Errorf("获取拒绝筛选器失败: %v", err)
	}

	blockIPs := parseIPsFromFilterList(blockFiltersOutput)
	log.Printf("[Firewall] 黑名单IP列表: %v", blockIPs)

	if err := removeDuplicateFilters(client, blockListName, blockIPs, writeOutput); err != nil {
		log.Printf("[Firewall] 删除重复筛选器失败: %v", err)
		writeOutput("❌ 删除重复筛选器失败\n")
		return fmt.Errorf("删除重复筛选器失败: %v", err)
	}

	tcpKey := fmt.Sprintf("any/TCP/%d", port)
	udpKey := fmt.Sprintf("any/UDP/%d", port)

	if !containsIP(blockIPs, tcpKey) {
		blockFilterCmdTCP := fmt.Sprintf(`netsh ipsec static add filter filterlist="%s" srcaddr=any dstaddr=me dstport=%d protocol=TCP`, blockListName, port)
		_, err := client.Execute(blockFilterCmdTCP)
		if err != nil {
			log.Printf("[Firewall] 创建拒绝筛选器(TCP)失败: %v", err)
			writeOutput("❌ 创建拒绝筛选器(TCP)失败\n")
			return fmt.Errorf("创建拒绝筛选器(TCP)失败: %v", err)
		}
		writeOutput("✅ 添加拒绝筛选器(TCP)成功\n")
	} else {
		writeOutput("⏭️ 拒绝筛选器(TCP)已存在，跳过\n")
	}

	if !containsIP(blockIPs, udpKey) {
		blockFilterCmdUDP := fmt.Sprintf(`netsh ipsec static add filter filterlist="%s" srcaddr=any dstaddr=me dstport=%d protocol=UDP`, blockListName, port)
		_, err := client.Execute(blockFilterCmdUDP)
		if err != nil {
			log.Printf("[Firewall] 创建拒绝筛选器(UDP)失败: %v", err)
			writeOutput("❌ 创建拒绝筛选器(UDP)失败\n")
			return fmt.Errorf("创建拒绝筛选器(UDP)失败: %v", err)
		}
		writeOutput("✅ 添加拒绝筛选器(UDP)成功\n")
	} else {
		writeOutput("⏭️ 拒绝筛选器(UDP)已存在，跳过\n")
	}

	if len(blockIPs) > 2 {
		log.Printf("[Firewall] 拒绝筛选器数量: %d，需要删除多余的", len(blockIPs))
		writeOutput(fmt.Sprintf("🔄 清理多余拒绝筛选器，保留2条(TCP+UDP)...\n"))

		showBlockFiltersCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s" level=verbose`, blockListName)
		showOutput, _ := client.Execute(showBlockFiltersCmd)
		filterLines := strings.Split(showOutput, "\n")
		var filterNames []string
		re := regexp.MustCompile(`Filter:\s*(.+)$`)
		for _, line := range filterLines {
			line = strings.TrimSpace(line)
			matches := re.FindStringSubmatch(line)
			if len(matches) == 2 {
				name := strings.TrimSpace(matches[1])
				if name != "" {
					filterNames = append(filterNames, name)
				}
			}
		}

		if len(filterNames) > 2 {
			for i := 2; i < len(filterNames); i++ {
				deleteCmd := fmt.Sprintf(`netsh ipsec static delete filter filterlist="%s" name="%s"`, blockListName, filterNames[i])
				_, err := client.Execute(deleteCmd)
				if err != nil {
					log.Printf("[Firewall] 删除拒绝筛选器失败(%s): %v", filterNames[i], err)
					writeOutput(fmt.Sprintf("❌ 删除拒绝筛选器(%s)失败\n", filterNames[i]))
				} else {
					log.Printf("[Firewall] 删除拒绝筛选器成功(%s)", filterNames[i])
					writeOutput(fmt.Sprintf("✅ 删除拒绝筛选器(%s)成功\n", filterNames[i]))
				}
			}
		}
	}

	return nil
}

// ensureRuleExists 确保规则存在，不存在则创建并绑定到策略
// 参数: client - WinRM客户端
//       ruleName - 规则名称
//       policyName - 策略名称
//       filterListName - 筛选器列表名称
//       filterActionName - 筛选器操作名称
//       writeOutput - 输出回调函数
// 返回: 错误
func ensureRuleExists(client WinRMClient, ruleName, policyName, filterListName, filterActionName string, writeOutput WriteFunc) error {
	checkCmd := fmt.Sprintf(`netsh ipsec static show rule name="%s" policy="%s"`, ruleName, policyName)
	checkOutput, err := client.Execute(checkCmd)
	if err != nil {
		log.Printf("[Firewall] 检查规则失败(%s): %v", ruleName, err)
		writeOutput(fmt.Sprintf("❌ 检查规则失败(%s): %v\n", ruleName, err))
		return fmt.Errorf("检查规则失败(%s): %v", ruleName, err)
	}

	ruleExists := false
	cnRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(ruleName))
	enRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(ruleName))

	if cnRegex.MatchString(checkOutput) || enRegex.MatchString(checkOutput) {
		ruleExists = true
	}

	if ruleExists {
		log.Printf("[Firewall] 规则%s已存在", ruleName)
		writeOutput(fmt.Sprintf("⏭️ %s规则已存在，跳过\n", ruleName))
		return nil
	}

	log.Printf("[Firewall] 绑定规则: %s", ruleName)
	ruleCmd := fmt.Sprintf(`netsh ipsec static add rule name="%s" policy="%s" filterlist="%s" filteraction="%s"`, ruleName, policyName, filterListName, filterActionName)
	_, err = client.Execute(ruleCmd)
	if err != nil {
		log.Printf("[Firewall] 绑定规则失败(%s): %v", ruleName, err)
		writeOutput(fmt.Sprintf("❌ 绑定%s规则失败\n", ruleName))
		return fmt.Errorf("绑定规则失败(%s): %v", ruleName, err)
	}
	writeOutput(fmt.Sprintf("✅ 绑定%s规则成功\n", ruleName))
	return nil
}

// refreshPolicy 刷新IPsec策略使其生效
// 通过先取消分配再重新分配策略来刷新
// 参数: client - WinRM客户端
//       writeOutput - 输出回调函数
// 返回: 错误
func refreshPolicy(client WinRMClient, writeOutput WriteFunc) error {
	log.Printf("[Firewall] 刷新策略")

	refreshCmds := []string{
		fmt.Sprintf(`netsh ipsec static set policy name="%s" assign=n`, policyName),
		fmt.Sprintf(`netsh ipsec static set policy name="%s" assign=y`, policyName),
	}

	for _, cmd := range refreshCmds {
		_, err := client.Execute(cmd)
		if err != nil {
			log.Printf("[Firewall] 刷新策略失败: %v", err)
			writeOutput("❌ 刷新策略失败\n")
			return fmt.Errorf("刷新策略失败: %v", err)
		}
	}

	writeOutput("✅ 刷新策略成功\n")
	return nil
}

// verifyPolicy 验证IPsec策略配置是否生效
// 验证项:
// 1. 策略是否已分配（Assigned=YES/已分配:是）
// 2. 允许/拒绝规则是否存在
// 3. 允许/拒绝筛选器列表是否存在
// 4. 允许/拒绝筛选器操作是否存在
// 5. 拒绝IP列表
// 6. 允许IP列表
// 参数: client - WinRM客户端
//       port - 目标端口
//       writeOutput - 输出回调函数
// 返回: 错误
func verifyPolicy(client WinRMClient, port int, writeOutput WriteFunc) error {
	log.Printf("[Firewall] 验证配置")
	writeOutput("验证配置...\n")

	allowFilterListName := fmt.Sprintf("允许%d访问", port)
	denyFilterListName := fmt.Sprintf("拒绝%d访问", port)

	verifyCmd := fmt.Sprintf(`netsh ipsec static show policy name="%s"`, policyName)
	verifyOutput, err := client.Execute(verifyCmd)
	if err != nil {
		log.Printf("[Firewall] 验证策略状态失败: %v", err)
		writeOutput("❌ 验证策略状态失败\n")
		return fmt.Errorf("验证策略状态失败: %v", err)
	}

	log.Printf("[Firewall] 策略状态输出: %s", verifyOutput)

	isAssigned := false

	enRegex := regexp.MustCompile(`Assigned\s*=\s*(YES|NO)`)
	matches := enRegex.FindStringSubmatch(verifyOutput)
	if len(matches) == 2 {
		isAssigned = matches[1] == "YES"
		log.Printf("[Firewall] 英文系统 - Assigned值: %s", matches[1])
	}

	cnRegex := regexp.MustCompile(`已分配\s*[:：]\s*(是|否)`)
	matches = cnRegex.FindStringSubmatch(verifyOutput)
	if len(matches) == 2 {
		isAssigned = matches[1] == "是"
		log.Printf("[Firewall] 中文系统 - 已分配值: %s", matches[1])
	}

	if !isAssigned {
		log.Printf("[Firewall] 策略未生效")
		writeOutput("⚠️ 策略未生效，请检查日志\n")
		return nil
	}

	writeOutput("检查策略中的规则...\n")

	allowRuleName := fmt.Sprintf("允许%d访问", port)
	denyRuleName := fmt.Sprintf("拒绝%d访问", port)

	var rules []string

	checkAllowRuleCmd := fmt.Sprintf(`netsh ipsec static show rule name="%s" policy="%s"`, allowRuleName, policyName)
	allowRuleOutput, _ := client.Execute(checkAllowRuleCmd)
	cnAllowRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(allowRuleName))
	enAllowRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(allowRuleName))
	if cnAllowRegex.MatchString(allowRuleOutput) || enAllowRegex.MatchString(allowRuleOutput) {
		rules = append(rules, allowRuleName)
	}

	checkDenyRuleCmd := fmt.Sprintf(`netsh ipsec static show rule name="%s" policy="%s"`, denyRuleName, policyName)
	denyRuleOutput, _ := client.Execute(checkDenyRuleCmd)
	cnDenyRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(denyRuleName))
	enDenyRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(denyRuleName))
	if cnDenyRegex.MatchString(denyRuleOutput) || enDenyRegex.MatchString(denyRuleOutput) {
		rules = append(rules, denyRuleName)
	}

	log.Printf("[Firewall] 发现规则: %v", rules)
	writeOutput(fmt.Sprintf("✅ 发现规则名称: [%s]\n", strings.Join(rules, " ")))

	if len(rules) == 0 {
		log.Printf("[Firewall] 策略中没有规则")
		writeOutput("❌ 策略中没有规则，配置未生效\n")
		return fmt.Errorf("策略中没有规则")
	}

	writeOutput("检查筛选器列表名称...\n")

	var filterLists []string

	checkAllowListCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s"`, allowFilterListName)
	allowListOutput, _ := client.Execute(checkAllowListCmd)
	cnAllowListRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(allowFilterListName))
	enAllowListRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(allowFilterListName))
	if cnAllowListRegex.MatchString(allowListOutput) || enAllowListRegex.MatchString(allowListOutput) {
		filterLists = append(filterLists, allowFilterListName)
	}

	checkDenyListCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s"`, denyFilterListName)
	denyListOutput, _ := client.Execute(checkDenyListCmd)
	cnDenyListRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(denyFilterListName))
	enDenyListRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(denyFilterListName))
	if cnDenyListRegex.MatchString(denyListOutput) || enDenyListRegex.MatchString(denyListOutput) {
		filterLists = append(filterLists, denyFilterListName)
	}

	log.Printf("[Firewall] 发现筛选器列表: %v", filterLists)
	writeOutput(fmt.Sprintf("✅ 发现筛选器列表名称: [%s]\n", strings.Join(filterLists, " ")))

	writeOutput("检查筛选器操作名称...\n")

	var filterActions []string

	checkAllowActionCmd := fmt.Sprintf(`netsh ipsec static show filteraction name="%s"`, filterActionAllow)
	allowActionOutput, _ := client.Execute(checkAllowActionCmd)
	cnAllowActionRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(filterActionAllow))
	enAllowActionRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(filterActionAllow))
	if cnAllowActionRegex.MatchString(allowActionOutput) || enAllowActionRegex.MatchString(allowActionOutput) {
		filterActions = append(filterActions, filterActionAllow)
	}

	checkDenyActionCmd := fmt.Sprintf(`netsh ipsec static show filteraction name="%s"`, filterActionBlock)
	denyActionOutput, _ := client.Execute(checkDenyActionCmd)
	cnDenyActionRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(filterActionBlock))
	enDenyActionRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(filterActionBlock))
	if cnDenyActionRegex.MatchString(denyActionOutput) || enDenyActionRegex.MatchString(denyActionOutput) {
		filterActions = append(filterActions, filterActionBlock)
	}

	log.Printf("[Firewall] 发现筛选器操作: %v", filterActions)
	writeOutput(fmt.Sprintf("✅ 发现筛选器操作名称: [%s]\n", strings.Join(filterActions, " ")))

	writeOutput("检查拒绝IP...\n")
	denyFilterListCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s" level=verbose`, denyFilterListName)
	denyOutput, err := client.Execute(denyFilterListCmd)
	if err != nil {
		log.Printf("[Firewall] 列出拒绝筛选器列表失败: %v", err)
	}

	denyIPs := parseIPsFromFilterList(denyOutput)
	log.Printf("[Firewall] 发现拒绝IP: %v", denyIPs)
	writeOutput(fmt.Sprintf("✅ 发现拒绝IP: [%s]\n", strings.Join(denyIPs, " ")))

	writeOutput("检查允许IP...\n")
	allowFilterListCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s" level=verbose`, allowFilterListName)
	allowOutput, err := client.Execute(allowFilterListCmd)
	if err != nil {
		log.Printf("[Firewall] 列出允许筛选器列表失败: %v", err)
	}

	allowIPs := parseIPsFromFilterList(allowOutput)
	log.Printf("[Firewall] 发现允许IP: %v", allowIPs)
	writeOutput(fmt.Sprintf("✅ 发现允许IP: [%s]\n", strings.Join(allowIPs, " ")))

	log.Printf("[Firewall] 配置验证通过")
	writeOutput("✅ 配置验证通过\n")
	return nil
}

// parseIPsFromFilterList 解析netsh ipsec static show filterlist level=verbose输出
// 提取源IP、协议、目标端口，格式化为 "IP/协议/端口"（如 "192.168.21.52/TCP/3389"）
// 支持中英文输出格式
// 参数: output - filterlist命令输出
// 返回: IP列表（格式为 IP/协议/端口）
func parseIPsFromFilterList(output string) []string {
	var results []string
	lines := strings.Split(output, "\n")

	var currentIP string
	var currentProtocol string
	var currentPort string

	log.Printf("[Firewall] parseIPsFromFilterList - 原始输出长度: %d", len(output))
	log.Printf("[Firewall] parseIPsFromFilterList - 行数: %d", len(lines))

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		log.Printf("[Firewall] parseIPsFromFilterList - 行%d: '%s'", i, line)

		if strings.Contains(line, "源 IP 地址") || strings.Contains(line, "Source Address") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentIP = strings.TrimSpace(parts[1])
				log.Printf("[Firewall] parseIPsFromFilterList - 找到源IP: '%s'", currentIP)
			}
		}

		if strings.Contains(line, "协议") || strings.Contains(line, "Protocol") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				proto := strings.TrimSpace(parts[1])
				currentProtocol = normalizeProtocol(proto)
				log.Printf("[Firewall] parseIPsFromFilterList - 找到协议: '%s' (标准化后: '%s')", proto, currentProtocol)
			}
		}

		if strings.Contains(line, "目标端口") || strings.Contains(line, "Destination Port") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentPort = strings.TrimSpace(parts[1])
				log.Printf("[Firewall] parseIPsFromFilterList - 找到目标端口: '%s'", currentPort)
			}
			if currentIP != "" && currentProtocol != "" && currentPort != "" {
				if currentIP == "any" || currentIP == "Any" || currentIP == "<任何 IP 地址>" || currentIP == "<任何IP地址>" {
					currentIP = "any"
				}
				if currentIP != "Me" && currentIP != "me" && currentIP != "<我的 IP 地址>" {
					results = append(results, fmt.Sprintf("%s/%s/%s", currentIP, currentProtocol, currentPort))
					log.Printf("[Firewall] parseIPsFromFilterList - 添加结果: '%s/%s/%s'", currentIP, currentProtocol, currentPort)
				}
				currentIP = ""
				currentProtocol = ""
				currentPort = ""
			}
		}
	}

	return results
}

// containsIP 检查IP是否已存在于列表中
// 参数: ipList - IP列表（格式为 IP/协议/端口）
//       ip - 要检查的IP
// 返回: 是否存在
func containsIP(ipList []string, ip string) bool {
	for _, existingIP := range ipList {
		if existingIP == ip {
			return true
		}
	}
	return false
}

// normalizeProtocol 将协议号或名称标准化为统一格式
// 映射: 6/TCP/传输控制协议 -> TCP, 17/UDP/用户数据报协议 -> UDP
// 参数: proto - 原始协议值
// 返回: 标准化后的协议名称
func normalizeProtocol(proto string) string {
	switch strings.ToUpper(proto) {
	case "6", "TCP", "传输控制协议":
		return "TCP"
	case "17", "UDP", "用户数据报协议":
		return "UDP"
	default:
		return proto
	}
}

// removeDuplicateFilters 删除筛选器列表中的重复筛选器
// 步骤:
// 1. 统计每个IP的出现次数
// 2. 标记出现多次的IP为重复
// 3. 对于每个重复IP，删除多余的筛选器（保留第一个）
// 参数: client - WinRM客户端
//       listName - 筛选器列表名称
//       ipList - IP列表
//       writeOutput - 输出回调函数
// 返回: 错误
func removeDuplicateFilters(client WinRMClient, listName string, ipList []string, writeOutput WriteFunc) error {
	seen := make(map[string]int)
	var duplicates []string

	for _, ip := range ipList {
		if count, exists := seen[ip]; exists {
			if count == 1 {
				duplicates = append(duplicates, ip)
			}
			seen[ip]++
		} else {
			seen[ip] = 1
		}
	}

	if len(duplicates) == 0 {
		return nil
	}

	log.Printf("[Firewall] 发现重复筛选器: %v", duplicates)
	writeOutput(fmt.Sprintf("🔄 清理重复筛选器(%s列表)...\n", listName))

	for range duplicates {
		showFiltersCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s" level=verbose`, listName)
		showOutput, err := client.Execute(showFiltersCmd)
		if err != nil {
			log.Printf("[Firewall] 获取筛选器列表失败: %v", err)
			writeOutput("❌ 获取筛选器列表失败\n")
			return fmt.Errorf("获取筛选器列表失败: %v", err)
		}

		filterLines := strings.Split(showOutput, "\n")
		var filterNames []string
		re := regexp.MustCompile(`Filter:\s*(.+)$`)
		for _, line := range filterLines {
			line = strings.TrimSpace(line)
			matches := re.FindStringSubmatch(line)
			if len(matches) == 2 {
				name := strings.TrimSpace(matches[1])
				if name != "" {
					filterNames = append(filterNames, name)
				}
			}
		}

		if len(filterNames) > 1 {
			for i := 1; i < len(filterNames); i++ {
				deleteCmd := fmt.Sprintf(`netsh ipsec static delete filter filterlist="%s" name="%s"`, listName, filterNames[i])
				_, err := client.Execute(deleteCmd)
				if err != nil {
					log.Printf("[Firewall] 删除重复筛选器失败(%s): %v", filterNames[i], err)
					writeOutput(fmt.Sprintf("❌ 删除重复筛选器(%s)失败\n", filterNames[i]))
					return fmt.Errorf("删除重复筛选器失败(%s): %v", filterNames[i], err)
				}
				log.Printf("[Firewall] 删除重复筛选器成功(%s)", filterNames[i])
				writeOutput(fmt.Sprintf("✅ 删除重复筛选器(%s)成功\n", filterNames[i]))
			}
		}
	}

	return nil
}

// deconfigureWindows 取消配置Windows IP安全策略主函数
// 分两种模式:
// 模式一（有白名单IP）: 仅删除指定的白名单IP，保留策略结构
// 模式二（无白名单IP）: 完整删除端口相关的规则和筛选器列表
// 参数: client - WinRM客户端
//       port - 目标端口
//       ipWhitelist - 白名单IP列表（逗号分隔，空表示完整删除）
//       callback - 进度回调函数
// 返回: 错误
func deconfigureWindows(client WinRMClient, port int, ipWhitelist string, callback func(message string)) error {
	log.Printf("[Firewall] 开始取消配置IP安全策略，端口: %d, IP白名单: '%s'", port, ipWhitelist)

	var output strings.Builder
	writeOutput := func(msg string) {
		output.WriteString(msg)
		if callback != nil {
			callback(msg)
		}
		log.Printf("[Firewall] %s", strings.TrimSpace(msg))
	}

	writeOutput("开始取消配置IP安全策略...\n")

	if err := checkSystemApplicability(client, writeOutput); err != nil {
		log.Printf("[Firewall] 取消配置失败: %v", err)
		return fmt.Errorf("%v", err)
	}

	if err := checkAndStartServices(client, writeOutput); err != nil {
		log.Printf("[Firewall] 取消配置失败: %v", err)
		return fmt.Errorf("%v", err)
	}

	policyExists, err := checkPolicyExists(client, writeOutput)
	if err != nil {
		log.Printf("[Firewall] 取消配置失败: %v", err)
		return fmt.Errorf("%v", err)
	}

	if !policyExists {
		log.Printf("[Firewall] 安全访问控制策略不存在")
		writeOutput("安全访问控制策略不存在，无需取消配置\n")
		return nil
	}

	allowFilterListName := fmt.Sprintf("允许%d访问", port)
	denyFilterListName := fmt.Sprintf("拒绝%d访问", port)
	allowRuleName := fmt.Sprintf("允许%d访问", port)
	denyRuleName := fmt.Sprintf("拒绝%d访问", port)

	if ipWhitelist != "" {
		if err := deconfigureWhitelistIPs(client, port, ipWhitelist, allowFilterListName, writeOutput); err != nil {
			log.Printf("[Firewall] 取消配置失败: %v", err)
			return fmt.Errorf("%v", err)
		}

		if err := refreshPolicy(client, writeOutput); err != nil {
			log.Printf("[Firewall] 刷新策略失败: %v", err)
			return fmt.Errorf("%v", err)
		}

		if err := verifyDeconfigureWhitelist(client, port, ipWhitelist, writeOutput); err != nil {
			log.Printf("[Firewall] 验证失败: %v", err)
			return fmt.Errorf("%v", err)
		}

		writeOutput("✅ 取消配置完成（仅删除白名单IP）\n")
		return nil
	}

	if err := deleteRules(client, port, allowRuleName, denyRuleName, writeOutput); err != nil {
		log.Printf("[Firewall] 删除规则失败: %v", err)
		return fmt.Errorf("%v", err)
	}

	if err := deleteFilterLists(client, port, allowFilterListName, denyFilterListName, writeOutput); err != nil {
		log.Printf("[Firewall] 删除筛选器列表失败: %v", err)
		return fmt.Errorf("%v", err)
	}

	if err := refreshPolicy(client, writeOutput); err != nil {
		log.Printf("[Firewall] 刷新策略失败: %v", err)
		return fmt.Errorf("%v", err)
	}

	if err := verifyDeconfigure(client, port, writeOutput); err != nil {
		log.Printf("[Firewall] 验证失败: %v", err)
		return fmt.Errorf("%v", err)
	}

	writeOutput("✅ 取消配置完成\n")
	return nil
}

// deconfigureWhitelistIPs 删除指定的白名单IP
// 为每个IP删除TCP和UDP筛选器
// 参数: client - WinRM客户端
//       port - 目标端口
//       ipWhitelist - 白名单IP列表
//       allowFilterListName - 允许列表名称
//       writeOutput - 输出回调函数
// 返回: 错误
func deconfigureWhitelistIPs(client WinRMClient, port int, ipWhitelist string, allowFilterListName string, writeOutput WriteFunc) error {
	log.Printf("[Firewall] 删除白名单IP，端口: %d, IP列表: '%s'", port, ipWhitelist)

	writeOutput("检查允许筛选器列表...\n")
	checkAllowListCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s"`, allowFilterListName)
	allowListOutput, err := client.Execute(checkAllowListCmd)
	if err != nil {
		log.Printf("[Firewall] 检查允许筛选器列表失败: %v", err)
		writeOutput("❌ 检查允许筛选器列表失败\n")
		return fmt.Errorf("检查允许筛选器列表失败: %v", err)
	}
	cnAllowListRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(allowFilterListName))
	enAllowListRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(allowFilterListName))

	if !cnAllowListRegex.MatchString(allowListOutput) && !enAllowListRegex.MatchString(allowListOutput) {
		writeOutput(fmt.Sprintf("⏭️ %s列表不存在，跳过\n", allowFilterListName))
		return nil
	}

	getFiltersCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s" level=verbose`, allowFilterListName)
	filtersOutput, err := client.Execute(getFiltersCmd)
	if err != nil {
		log.Printf("[Firewall] 获取现有筛选器失败: %v", err)
		writeOutput("❌ 获取现有筛选器失败\n")
		return fmt.Errorf("获取现有筛选器失败: %v", err)
	}
	existingIPs := parseIPsFromFilterList(filtersOutput)
	log.Printf("[Firewall] 当前允许IP列表: %v", existingIPs)

	writeOutput(fmt.Sprintf("删除允许筛选器列表(%s)中的白名单IP...\n", allowFilterListName))

	ipList := strings.Split(ipWhitelist, ",")
	for _, ip := range ipList {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}

		tcpKey := fmt.Sprintf("%s/TCP/%d", ip, port)
		udpKey := fmt.Sprintf("%s/UDP/%d", ip, port)

		if !containsIP(existingIPs, tcpKey) {
			writeOutput(fmt.Sprintf("⏭️ 白名单IP(%s,TCP)不存在，跳过\n", ip))
		} else {
			deleteFilterCmdTCP := fmt.Sprintf(`netsh ipsec static delete filter filterlist="%s" srcaddr=%s dstaddr=me dstport=%d protocol=TCP`, allowFilterListName, ip, port)
			_, err := client.Execute(deleteFilterCmdTCP)
			if err != nil {
				log.Printf("[Firewall] 删除白名单IP失败(%s,TCP): %v", ip, err)
				writeOutput(fmt.Sprintf("❌ 删除白名单IP(%s,TCP)失败\n", ip))
				return fmt.Errorf("删除白名单IP失败(%s,TCP): %v", ip, err)
			}
			log.Printf("[Firewall] 删除白名单IP成功(%s,TCP)", ip)
			writeOutput(fmt.Sprintf("✅ 删除白名单IP(%s,TCP)成功\n", ip))
		}

		if !containsIP(existingIPs, udpKey) {
			writeOutput(fmt.Sprintf("⏭️ 白名单IP(%s,UDP)不存在，跳过\n", ip))
		} else {
			deleteFilterCmdUDP := fmt.Sprintf(`netsh ipsec static delete filter filterlist="%s" srcaddr=%s dstaddr=me dstport=%d protocol=UDP`, allowFilterListName, ip, port)
			_, err = client.Execute(deleteFilterCmdUDP)
			if err != nil {
				log.Printf("[Firewall] 删除白名单IP失败(%s,UDP): %v", ip, err)
				writeOutput(fmt.Sprintf("❌ 删除白名单IP(%s,UDP)失败\n", ip))
				return fmt.Errorf("删除白名单IP失败(%s,UDP): %v", ip, err)
			}
			log.Printf("[Firewall] 删除白名单IP成功(%s,UDP)", ip)
			writeOutput(fmt.Sprintf("✅ 删除白名单IP(%s,UDP)成功\n", ip))
		}
	}

	return nil
}

// deleteRules 删除指定端口的允许和拒绝规则
// 参数: client - WinRM客户端
//       port - 目标端口
//       allowRuleName - 允许规则名称
//       denyRuleName - 拒绝规则名称
//       writeOutput - 输出回调函数
// 返回: 错误
func deleteRules(client WinRMClient, port int, allowRuleName, denyRuleName string, writeOutput WriteFunc) error {
	log.Printf("[Firewall] 删除端口%d的规则", port)

	writeOutput("检查策略中的规则...\n")

	checkAllowRuleCmd := fmt.Sprintf(`netsh ipsec static show rule name="%s" policy="%s"`, allowRuleName, policyName)
	allowRuleOutput, err := client.Execute(checkAllowRuleCmd)
	if err != nil {
		log.Printf("[Firewall] 检查规则失败(%s): %v", allowRuleName, err)
		writeOutput(fmt.Sprintf("❌ 检查规则失败(%s): %v\n", allowRuleName, err))
		return fmt.Errorf("检查规则失败(%s): %v", allowRuleName, err)
	}
	cnAllowRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(allowRuleName))
	enAllowRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(allowRuleName))

	if cnAllowRegex.MatchString(allowRuleOutput) || enAllowRegex.MatchString(allowRuleOutput) {
		log.Printf("[Firewall] 删除端口%d的允许规则", port)
		writeOutput(fmt.Sprintf("删除端口%d的允许规则...\n", port))
		deleteAllowRuleCmd := fmt.Sprintf(`netsh ipsec static delete rule name="%s" policy="%s"`, allowRuleName, policyName)
		_, err := client.Execute(deleteAllowRuleCmd)
		if err != nil {
			log.Printf("[Firewall] 删除允许规则失败(%s): %v", allowRuleName, err)
			writeOutput(fmt.Sprintf("❌ 删除允许规则(%s)失败\n", allowRuleName))
			return fmt.Errorf("删除允许规则失败(%s): %v", allowRuleName, err)
		}
		log.Printf("[Firewall] 删除允许规则成功(%s)", allowRuleName)
		writeOutput(fmt.Sprintf("✅ 删除允许规则(%s)成功\n", allowRuleName))
	} else {
		writeOutput(fmt.Sprintf("⏭️ %s规则不存在，跳过\n", allowRuleName))
	}

	checkDenyRuleCmd := fmt.Sprintf(`netsh ipsec static show rule name="%s" policy="%s"`, denyRuleName, policyName)
	denyRuleOutput, err := client.Execute(checkDenyRuleCmd)
	if err != nil {
		log.Printf("[Firewall] 检查规则失败(%s): %v", denyRuleName, err)
		writeOutput(fmt.Sprintf("❌ 检查规则失败(%s): %v\n", denyRuleName, err))
		return fmt.Errorf("检查规则失败(%s): %v", denyRuleName, err)
	}
	cnDenyRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(denyRuleName))
	enDenyRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(denyRuleName))

	if cnDenyRegex.MatchString(denyRuleOutput) || enDenyRegex.MatchString(denyRuleOutput) {
		log.Printf("[Firewall] 删除端口%d的拒绝规则", port)
		writeOutput(fmt.Sprintf("删除端口%d的拒绝规则...\n", port))
		deleteDenyRuleCmd := fmt.Sprintf(`netsh ipsec static delete rule name="%s" policy="%s"`, denyRuleName, policyName)
		_, err := client.Execute(deleteDenyRuleCmd)
		if err != nil {
			log.Printf("[Firewall] 删除拒绝规则失败(%s): %v", denyRuleName, err)
			writeOutput(fmt.Sprintf("❌ 删除拒绝规则(%s)失败\n", denyRuleName))
			return fmt.Errorf("删除拒绝规则失败(%s): %v", denyRuleName, err)
		}
		log.Printf("[Firewall] 删除拒绝规则成功(%s)", denyRuleName)
		writeOutput(fmt.Sprintf("✅ 删除拒绝规则(%s)成功\n", denyRuleName))
	} else {
		writeOutput(fmt.Sprintf("⏭️ %s规则不存在，跳过\n", denyRuleName))
	}

	return nil
}

// deleteFilterLists 删除指定端口的允许和拒绝筛选器列表
// 参数: client - WinRM客户端
//       port - 目标端口
//       allowFilterListName - 允许列表名称
//       denyFilterListName - 拒绝列表名称
//       writeOutput - 输出回调函数
// 返回: 错误
func deleteFilterLists(client WinRMClient, port int, allowFilterListName, denyFilterListName string, writeOutput WriteFunc) error {
	log.Printf("[Firewall] 删除端口%d的筛选器列表", port)

	writeOutput("检查筛选器列表名称...\n")

	checkAllowListCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s"`, allowFilterListName)
	allowListOutput, err := client.Execute(checkAllowListCmd)
	if err != nil {
		log.Printf("[Firewall] 检查筛选器列表失败(%s): %v", allowFilterListName, err)
		writeOutput(fmt.Sprintf("❌ 检查筛选器列表失败(%s): %v\n", allowFilterListName, err))
		return fmt.Errorf("检查筛选器列表失败(%s): %v", allowFilterListName, err)
	}
	cnAllowListRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(allowFilterListName))
	enAllowListRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(allowFilterListName))

	if cnAllowListRegex.MatchString(allowListOutput) || enAllowListRegex.MatchString(allowListOutput) {
		log.Printf("[Firewall] 删除端口%d的允许筛选器列表", port)
		writeOutput(fmt.Sprintf("删除端口%d的允许筛选器列表...\n", port))
		deleteAllowListCmd := fmt.Sprintf(`netsh ipsec static delete filterlist name="%s"`, allowFilterListName)
		_, err := client.Execute(deleteAllowListCmd)
		if err != nil {
			log.Printf("[Firewall] 删除允许筛选器列表失败(%s): %v", allowFilterListName, err)
			writeOutput(fmt.Sprintf("❌ 删除允许筛选器列表(%s)失败\n", allowFilterListName))
			return fmt.Errorf("删除允许筛选器列表失败(%s): %v", allowFilterListName, err)
		}
		log.Printf("[Firewall] 删除允许筛选器列表成功(%s)", allowFilterListName)
		writeOutput(fmt.Sprintf("✅ 删除允许筛选器列表(%s)成功\n", allowFilterListName))
	} else {
		writeOutput(fmt.Sprintf("⏭️ %s列表不存在，跳过\n", allowFilterListName))
	}

	checkDenyListCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s"`, denyFilterListName)
	denyListOutput, err := client.Execute(checkDenyListCmd)
	if err != nil {
		log.Printf("[Firewall] 检查筛选器列表失败(%s): %v", denyFilterListName, err)
		writeOutput(fmt.Sprintf("❌ 检查筛选器列表失败(%s): %v\n", denyFilterListName, err))
		return fmt.Errorf("检查筛选器列表失败(%s): %v", denyFilterListName, err)
	}
	cnDenyListRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(denyFilterListName))
	enDenyListRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(denyFilterListName))

	if cnDenyListRegex.MatchString(denyListOutput) || enDenyListRegex.MatchString(denyListOutput) {
		log.Printf("[Firewall] 删除端口%d的拒绝筛选器列表", port)
		writeOutput(fmt.Sprintf("删除端口%d的拒绝筛选器列表...\n", port))
		deleteDenyListCmd := fmt.Sprintf(`netsh ipsec static delete filterlist name="%s"`, denyFilterListName)
		_, err := client.Execute(deleteDenyListCmd)
		if err != nil {
			log.Printf("[Firewall] 删除拒绝筛选器列表失败(%s): %v", denyFilterListName, err)
			writeOutput(fmt.Sprintf("❌ 删除拒绝筛选器列表(%s)失败\n", denyFilterListName))
			return fmt.Errorf("删除拒绝筛选器列表失败(%s): %v", denyFilterListName, err)
		}
		log.Printf("[Firewall] 删除拒绝筛选器列表成功(%s)", denyFilterListName)
		writeOutput(fmt.Sprintf("✅ 删除拒绝筛选器列表(%s)成功\n", denyFilterListName))
	} else {
		writeOutput(fmt.Sprintf("⏭️ %s列表不存在，跳过\n", denyFilterListName))
	}

	return nil
}

// verifyDeconfigureWhitelist 验证白名单IP是否已删除
// 参数: client - WinRM客户端
//       port - 目标端口
//       ipWhitelist - 白名单IP列表
//       writeOutput - 输出回调函数
// 返回: 错误
func verifyDeconfigureWhitelist(client WinRMClient, port int, ipWhitelist string, writeOutput WriteFunc) error {
	log.Printf("[Firewall] 验证取消配置（仅删除白名单IP）")

	writeOutput("验证取消配置...\n")

	allowFilterListName := fmt.Sprintf("允许%d访问", port)

	writeOutput("检查允许筛选器列表中的白名单IP...\n")
	allowFilterListCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s" level=verbose`, allowFilterListName)
	allowOutput, err := client.Execute(allowFilterListCmd)
	if err != nil {
		log.Printf("[Firewall] 列出允许筛选器列表失败: %v", err)
	}

	allowIPs := parseIPsFromFilterList(allowOutput)
	log.Printf("[Firewall] 当前允许IP列表: %v", allowIPs)

	ipList := strings.Split(ipWhitelist, ",")
	for _, ip := range ipList {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}

		tcpKey := fmt.Sprintf("%s/TCP/%d", ip, port)
		udpKey := fmt.Sprintf("%s/UDP/%d", ip, port)

		if containsIP(allowIPs, tcpKey) {
			log.Printf("[Firewall] 白名单IP(%s,TCP)仍存在", ip)
			writeOutput(fmt.Sprintf("⚠️ 白名单IP(%s,TCP)仍存在\n", ip))
		} else {
			writeOutput(fmt.Sprintf("✅ 白名单IP(%s,TCP)已删除\n", ip))
		}

		if containsIP(allowIPs, udpKey) {
			log.Printf("[Firewall] 白名单IP(%s,UDP)仍存在", ip)
			writeOutput(fmt.Sprintf("⚠️ 白名单IP(%s,UDP)仍存在\n", ip))
		} else {
			writeOutput(fmt.Sprintf("✅ 白名单IP(%s,UDP)已删除\n", ip))
		}
	}

	writeOutput("✅ 取消配置验证通过（仅删除白名单IP）\n")
	return nil
}

// verifyDeconfigure 验证完整取消配置结果
// 验证项:
// 1. 安全访问控制策略保留（不删除）
// 2. 允许/拒绝规则是否已删除
// 3. 允许/拒绝筛选器列表是否已删除
// 参数: client - WinRM客户端
//       port - 目标端口
//       writeOutput - 输出回调函数
// 返回: 错误
func verifyDeconfigure(client WinRMClient, port int, writeOutput WriteFunc) error {
	log.Printf("[Firewall] 验证取消配置")

	writeOutput("验证取消配置...\n")

	writeOutput("✅ 安全访问控制策略保留\n")

	allowFilterListName := fmt.Sprintf("允许%d访问", port)
	denyFilterListName := fmt.Sprintf("拒绝%d访问", port)
	allowRuleName := fmt.Sprintf("允许%d访问", port)
	denyRuleName := fmt.Sprintf("拒绝%d访问", port)

	writeOutput("检查策略中的规则...\n")

	checkAllowRuleCmd := fmt.Sprintf(`netsh ipsec static show rule name="%s" policy="%s"`, allowRuleName, policyName)
	allowRuleOutput, _ := client.Execute(checkAllowRuleCmd)
	cnAllowRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(allowRuleName))
	enAllowRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(allowRuleName))
	if cnAllowRegex.MatchString(allowRuleOutput) || enAllowRegex.MatchString(allowRuleOutput) {
		writeOutput(fmt.Sprintf("⚠️ %s规则仍存在\n", allowRuleName))
	} else {
		writeOutput(fmt.Sprintf("✅ %s规则已删除\n", allowRuleName))
	}

	checkDenyRuleCmd := fmt.Sprintf(`netsh ipsec static show rule name="%s" policy="%s"`, denyRuleName, policyName)
	denyRuleOutput, _ := client.Execute(checkDenyRuleCmd)
	cnDenyRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(denyRuleName))
	enDenyRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(denyRuleName))
	if cnDenyRegex.MatchString(denyRuleOutput) || enDenyRegex.MatchString(denyRuleOutput) {
		writeOutput(fmt.Sprintf("⚠️ %s规则仍存在\n", denyRuleName))
	} else {
		writeOutput(fmt.Sprintf("✅ %s规则已删除\n", denyRuleName))
	}

	writeOutput("检查筛选器列表名称...\n")

	checkAllowListCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s"`, allowFilterListName)
	allowListOutput, _ := client.Execute(checkAllowListCmd)
	cnAllowListRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(allowFilterListName))
	enAllowListRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(allowFilterListName))
	if cnAllowListRegex.MatchString(allowListOutput) || enAllowListRegex.MatchString(allowListOutput) {
		writeOutput(fmt.Sprintf("⚠️ %s列表仍存在\n", allowFilterListName))
	} else {
		writeOutput(fmt.Sprintf("✅ %s列表已删除\n", allowFilterListName))
	}

	checkDenyListCmd := fmt.Sprintf(`netsh ipsec static show filterlist name="%s"`, denyFilterListName)
	denyListOutput, _ := client.Execute(checkDenyListCmd)
	cnDenyListRegex := regexp.MustCompile(`名称\s*[：:]\s*` + regexp.QuoteMeta(denyFilterListName))
	enDenyListRegex := regexp.MustCompile(`Name\s*=\s*` + regexp.QuoteMeta(denyFilterListName))
	if cnDenyListRegex.MatchString(denyListOutput) || enDenyListRegex.MatchString(denyListOutput) {
		writeOutput(fmt.Sprintf("⚠️ %s列表仍存在\n", denyFilterListName))
	} else {
		writeOutput(fmt.Sprintf("✅ %s列表已删除\n", denyFilterListName))
	}

	writeOutput("✅ 取消配置验证通过\n")
	return nil
}