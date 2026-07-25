package portscanner

import (
	"access-control-tool/internal/models"
	"access-control-tool/internal/ssh"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// scanLinuxPorts 扫描Linux服务器上监听全局地址的端口
// 参数:
//   client - SSH客户端
// 返回值:
//   []PortInfo - 端口信息列表
//   error - 扫描过程中的错误
// 白名单机制:
//   - 只扫描监听全局地址的端口（0.0.0.0, ::, :::, *）
//   - 排除仅监听本地地址的端口（127.0.0.1, ::1）
//   - 支持IPv4和IPv6协议
//   - 支持ss和netstat两种命令输出格式
//   - 获取已建立连接的客户端IP列表
func scanLinuxPorts(client *ssh.Client) ([]PortInfo, error) {
	log.Printf("[PortScanner] 开始扫描Linux端口（白名单模式）")

	log.Printf("[PortScanner] 执行命令: ss -tlnp 或 netstat -tlnp")
	listenOutput, err := client.Execute("ss -tlnp 2>/dev/null || netstat -tlnp 2>/dev/null")
	if err != nil {
		log.Printf("[PortScanner] Linux端口扫描失败: %v", err)
		return nil, err
	}

	log.Printf("[PortScanner] 命令执行成功，输出长度: %d 字节", len(listenOutput))

	portMap := make(map[int]*PortInfo)

	for _, line := range strings.Split(listenOutput, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "State") || strings.HasPrefix(line, "Active") {
			continue
		}
		log.Printf("[PortScanner] 处理行: %s", line)

		// 匹配ss命令的IPv4全局监听格式（0.0.0.0:port）
		reSSIPv4 := regexp.MustCompile(`(?:LISTEN|LISTENING)\s+.*?0\.0\.0\.0:(\d+)`)
		if matches := reSSIPv4.FindStringSubmatch(line); len(matches) == 2 {
			processPortLine(line, matches[1], portMap)
			continue
		}

		// 匹配ss命令的IPv6全局监听格式（:::port）
		reSSIPv6 := regexp.MustCompile(`(?:LISTEN|LISTENING)\s+.*?:::(\d+)`)
		if matches := reSSIPv6.FindStringSubmatch(line); len(matches) == 2 {
			processPortLine(line, matches[1], portMap)
			continue
		}

		// 匹配ss命令的IPv6全局监听格式（[::]:port）
		reSSIPv6Bracket := regexp.MustCompile(`(?:LISTEN|LISTENING)\s+.*?\[::\]:(\d+)`)
		if matches := reSSIPv6Bracket.FindStringSubmatch(line); len(matches) == 2 {
			processPortLine(line, matches[1], portMap)
			continue
		}

		// 匹配ss命令的通配符格式（*:port）
		reSSWildcard := regexp.MustCompile("(?:LISTEN|LISTENING)\\s+.*?\\*:(\\d+)")
		if matches := reSSWildcard.FindStringSubmatch(line); len(matches) == 2 {
			processPortLine(line, matches[1], portMap)
			continue
		}

		// 匹配netstat命令的IPv4全局监听格式
		reNetstatIPv4 := regexp.MustCompile(`tcp\s+.*?0\.0\.0\.0:(\d+)\s+0\.0\.0\.0:\*\s+LISTEN`)
		if matches := reNetstatIPv4.FindStringSubmatch(line); len(matches) == 2 {
			processPortLine(line, matches[1], portMap)
			continue
		}

		// 匹配netstat命令的IPv6全局监听格式
		reNetstatIPv6 := regexp.MustCompile("tcp6\\s+.*?:::(\\d+)\\s+:::\\*\\s+LISTEN")
		if matches := reNetstatIPv6.FindStringSubmatch(line); len(matches) == 2 {
			processPortLine(line, matches[1], portMap)
			continue
		}

		log.Printf("[PortScanner] 跳过非全局监听端口: %s", line)
	}

	log.Printf("[PortScanner] 获取已建立连接的客户端IP")
	err = fetchConnectedIPs(client, portMap)
	if err != nil {
		log.Printf("[PortScanner] 获取连接IP失败: %v", err)
	}

	result := make([]PortInfo, 0, len(portMap))
	for _, info := range portMap {
		result = append(result, *info)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Port < result[j].Port
	})

	log.Printf("[PortScanner] Linux端口扫描完成，共发现 %d 个全局监听端口", len(result))
	return result, nil
}

// fetchConnectedIPs 获取已建立连接的客户端IP列表
// 参数:
//   client - SSH客户端
//   portMap - 端口信息映射表，用于填充ConnectedIPs字段
// 返回值:
//   error - 执行过程中的错误
// 支持的命令格式:
//   ss -tn: ESTAB 0 0 172.19.6.75:22 192.168.21.52:57436
//   netstat -tn: tcp 0 0 192.168.3.47:46422 192.168.21.52:58155 ESTABLISHED
func fetchConnectedIPs(client *ssh.Client, portMap map[int]*PortInfo) error {
	output, err := client.Execute("ss -tn 2>/dev/null || netstat -tn 2>/dev/null")
	if err != nil {
		return err
	}

	log.Printf("[PortScanner] 连接状态输出长度: %d 字节", len(output))

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "State") || strings.HasPrefix(line, "Active") {
			continue
		}

		log.Printf("[PortScanner] 处理连接行: %s", line)

		// 匹配ss命令格式: ESTAB 0 0 172.19.6.75:22 192.168.21.52:57436
		reSS := regexp.MustCompile(`ESTAB(?:LISHED)?\s+\d+\s+\d+\s+([0-9a-f.:]+):(\d+)\s+([0-9a-f.:]+):(\d+)`)
		if matches := reSS.FindStringSubmatch(line); len(matches) == 5 {
			log.Printf("[PortScanner] ss格式匹配成功: local=%s:%s, remote=%s:%s", matches[1], matches[2], matches[3], matches[4])
			localPort, err := strconv.Atoi(matches[2])
			if err != nil {
				continue
			}
			remoteIP := matches[3]
			addConnectedIP(portMap, localPort, remoteIP)
			continue
		}

		// 匹配netstat命令格式: tcp 0 0 192.168.3.47:46422 192.168.21.52:58155 ESTABLISHED
		reNetstat := regexp.MustCompile(`tcp(?:6)?\s+\d+\s+\d+\s+([0-9a-f.:]+):(\d+)\s+([0-9a-f.:]+):(\d+)\s+ESTABLISHED`)
		if matches := reNetstat.FindStringSubmatch(line); len(matches) == 6 {
			log.Printf("[PortScanner] netstat格式匹配成功: local=%s:%s, remote=%s:%s", matches[1], matches[2], matches[3], matches[4])
			localPort, err := strconv.Atoi(matches[2])
			if err != nil {
				continue
			}
			remoteIP := matches[3]
			addConnectedIP(portMap, localPort, remoteIP)
			continue
		}

		log.Printf("[PortScanner] 未匹配到连接行格式")
	}

	return nil
}

// addConnectedIP 将远程客户端IP添加到端口的ConnectedIPs列表中（去重）
// 参数:
//   portMap - 端口信息映射表
//   localPort - 本地端口号
//   remoteIP - 远程客户端IP地址
func addConnectedIP(portMap map[int]*PortInfo, localPort int, remoteIP string) {
	if portMap[localPort] == nil {
		return
	}

	if strings.HasPrefix(remoteIP, "[::1]") || remoteIP == "127.0.0.1" || remoteIP == "::1" {
		return
	}

	if strings.Contains(remoteIP, "[") && strings.Contains(remoteIP, "]") {
		start := strings.Index(remoteIP, "[") + 1
		end := strings.Index(remoteIP, "]")
		if start < end {
			remoteIP = remoteIP[start:end]
		}
	}

	for _, ip := range portMap[localPort].ConnectedIPs {
		if ip == remoteIP {
			return
		}
	}

	portMap[localPort].ConnectedIPs = append(portMap[localPort].ConnectedIPs, remoteIP)
	log.Printf("[PortScanner] 端口 %d 发现连接IP: %s", localPort, remoteIP)
}

// refreshConnectedIPsLinux Linux平台刷新指定端口的连接IP
// 参数:
//   server - 服务器信息
//   port - 端口号
// 返回值:
//   []string - 客户端IP列表
//   error - 执行过程中的错误
func refreshConnectedIPsLinux(server *models.Server, port int) ([]string, error) {
	return GetConnectedIPsForPort(server, port)
}

// GetConnectedIPsForPort 获取指定端口的已建立连接客户端IP列表
// 参数:
//   server - 服务器信息
//   port - 端口号
// 返回值:
//   []string - 客户端IP列表
//   error - 执行过程中的错误
// 用途:
//   - 用于配置窗口切换端口时快速刷新关联业务IP
//   - 避免重新扫描所有端口，提高性能
func GetConnectedIPsForPort(server *models.Server, port int) ([]string, error) {
	log.Printf("[PortScanner] 获取端口 %d 的连接IP", port)

	client, err := ssh.NewClient(server)
	if err != nil {
		log.Printf("[PortScanner] 创建SSH客户端失败: %v", err)
		return nil, err
	}
	defer client.Close()

	output, err := client.Execute("ss -tn 2>/dev/null || netstat -tn 2>/dev/null")
	if err != nil {
		log.Printf("[PortScanner] 执行命令失败: %v", err)
		return nil, err
	}

	log.Printf("[PortScanner] 连接状态输出长度: %d 字节", len(output))

	connectedIPs := make([]string, 0)
	seenIPs := make(map[string]bool)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "State") || strings.HasPrefix(line, "Active") {
			continue
		}

		log.Printf("[PortScanner] 处理连接行: %s", line)

		reSS := regexp.MustCompile(`ESTAB(?:LISHED)?\s+\d+\s+\d+\s+([0-9a-f.:]+):(\d+)\s+([0-9a-f.:]+):(\d+)`)
		if matches := reSS.FindStringSubmatch(line); len(matches) == 5 {
			log.Printf("[PortScanner] ss格式匹配成功: local=%s:%s, remote=%s:%s", matches[1], matches[2], matches[3], matches[4])
			localPort, err := strconv.Atoi(matches[2])
			if err != nil {
				continue
			}
			if localPort != port {
				continue
			}
			remoteIP := matches[3]
			if seenIPs[remoteIP] {
				continue
			}
			if strings.HasPrefix(remoteIP, "[::1]") || remoteIP == "127.0.0.1" || remoteIP == "::1" {
				continue
			}
			if strings.Contains(remoteIP, "[") && strings.Contains(remoteIP, "]") {
				start := strings.Index(remoteIP, "[") + 1
				end := strings.Index(remoteIP, "]")
				if start < end {
					remoteIP = remoteIP[start:end]
				}
			}
			connectedIPs = append(connectedIPs, remoteIP)
			seenIPs[remoteIP] = true
			continue
		}

		reNetstat := regexp.MustCompile(`tcp(?:6)?\s+\d+\s+\d+\s+([0-9a-f.:]+):(\d+)\s+([0-9a-f.:]+):(\d+)\s+ESTABLISHED`)
		if matches := reNetstat.FindStringSubmatch(line); len(matches) == 6 {
			log.Printf("[PortScanner] netstat格式匹配成功: local=%s:%s, remote=%s:%s", matches[1], matches[2], matches[3], matches[4])
			localPort, err := strconv.Atoi(matches[2])
			if err != nil {
				continue
			}
			if localPort != port {
				continue
			}
			remoteIP := matches[3]
			if seenIPs[remoteIP] {
				continue
			}
			if strings.HasPrefix(remoteIP, "[::1]") || remoteIP == "127.0.0.1" || remoteIP == "::1" {
				continue
			}
			if strings.Contains(remoteIP, "[") && strings.Contains(remoteIP, "]") {
				start := strings.Index(remoteIP, "[") + 1
				end := strings.Index(remoteIP, "]")
				if start < end {
					remoteIP = remoteIP[start:end]
				}
			}
			connectedIPs = append(connectedIPs, remoteIP)
			seenIPs[remoteIP] = true
			continue
		}
	}

	log.Printf("[PortScanner] 端口 %d 发现 %d 个连接IP", port, len(connectedIPs))
	return connectedIPs, nil
}

// processPortLine 处理端口行，提取端口号和服务名称
// 参数:
//   line - 原始行内容
//   portStr - 端口号字符串
//   portMap - 端口信息映射表
func processPortLine(line, portStr string, portMap map[int]*PortInfo) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return
	}

	if portMap[port] == nil {
		portMap[port] = &PortInfo{
			Port:        port,
			ServiceName: extractServiceName(line),
			ConnectedIPs: make([]string, 0),
		}
		log.Printf("[PortScanner] 发现全局监听端口: %d", port)
	} else {
		if name := extractServiceName(line); name != "" && portMap[port].ServiceName == "" {
			portMap[port].ServiceName = name
		}
	}
}

// extractServiceName 从命令输出行中提取服务名称
// 参数:
//   line - 原始行内容
// 返回值:
//   string - 服务名称，未找到返回空字符串
func extractServiceName(line string) string {
	// 优先匹配ss命令格式（users:(("service_name"）
	reSS := regexp.MustCompile(`users:\(\("([^"]+)"`)
	if matches := reSS.FindStringSubmatch(line); len(matches) == 2 {
		return matches[1]
	}

	// 匹配netstat命令格式（pid/service_name）
	reNetstat := regexp.MustCompile(`(\d+)/([a-zA-Z0-9_-]+)\s*$`)
	if matches := reNetstat.FindStringSubmatch(line); len(matches) == 3 {
		return matches[2]
	}

	// 匹配netstat命令的另一种格式
	reNetstatAlt := regexp.MustCompile(`LISTEN\s+.*?(\d+)/([a-zA-Z0-9_-]+)`)
	if matches := reNetstatAlt.FindStringSubmatch(line); len(matches) == 3 {
		return matches[2]
	}

	return ""
}