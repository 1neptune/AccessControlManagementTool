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

// scanLinuxPorts 扫描Linux服务器上的监听端口
// 参数:
//   client - SSH客户端
// 返回值:
//   []PortInfo - 端口信息列表
//   error - 扫描过程中的错误
// 扫描范围:
//   - 包含监听全局地址的端口（0.0.0.0, ::, :::, *）
//   - 包含监听非回环具体IP地址的端口（如 192.168.100.53:5437）
//   - 排除仅监听本地回环地址的端口（127.0.0.1, ::1）
//   - 支持IPv4和IPv6协议
//   - 支持ss和netstat两种命令输出格式
//   - 端口号全局去重：同一端口在多个地址监听（如0.0.0.0:5437和192.168.100.53:5437）时合并为一条记录
//   - 获取已建立连接的客户端IP列表
func scanLinuxPorts(client *ssh.Client) ([]PortInfo, error) {
	log.Printf("[PortScanner] 开始扫描Linux端口（包含非回环地址监听）")

	// 执行ss或netstat命令获取TCP监听端口信息
	// 优先使用ss命令（性能更好），失败时降级到netstat
	log.Printf("[PortScanner] 执行命令: ss -tlnp 或 netstat -tlnp")
	listenOutput, err := client.Execute("ss -tlnp 2>/dev/null || netstat -tlnp 2>/dev/null")
	if err != nil {
		log.Printf("[PortScanner] Linux端口扫描失败: %v", err)
		return nil, err
	}

	log.Printf("[PortScanner] 命令执行成功，输出长度: %d 字节", len(listenOutput))

	// portMap 以端口号作为key存储端口信息，实现端口号去重
	// 同一端口号在多个地址监听时，仅保留第一条记录（processPortLine内部处理）
	portMap := make(map[int]*PortInfo)

	for _, line := range strings.Split(listenOutput, "\n") {
		line = strings.TrimSpace(line)
		// 跳过空行和表头行（ss输出以"State"开头，netstat输出以"Active"开头）
		if line == "" || strings.HasPrefix(line, "State") || strings.HasPrefix(line, "Active") {
			continue
		}
		log.Printf("[PortScanner] 处理行: %s", line)

		// 从当前行提取监听地址和端口（兼容ss和netstat两种格式）
		ip, portStr, matched := parseListenAddress(line)
		if !matched {
			log.Printf("[PortScanner] 跳过非监听端口行: %s", line)
			continue
		}

		// 排除回环地址（127.0.0.1, ::1）监听的端口，这些端口仅本机访问无需配置访问控制
		if isLoopbackAddress(ip) {
			log.Printf("[PortScanner] 跳过回环地址监听端口: %s:%s", ip, portStr)
			continue
		}

		// 处理端口行，将端口信息加入portMap（按端口号自动去重）
		processPortLine(line, portStr, portMap)
	}

	log.Printf("[PortScanner] 获取已建立连接的客户端IP")
	err = fetchConnectedIPs(client, portMap)
	if err != nil {
		log.Printf("[PortScanner] 获取连接IP失败: %v", err)
	}

	// 将map转换为切片并按端口号升序排序，便于用户查看
	result := make([]PortInfo, 0, len(portMap))
	for _, info := range portMap {
		result = append(result, *info)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Port < result[j].Port
	})

	log.Printf("[PortScanner] Linux端口扫描完成，共发现 %d 个监听端口（已去重）", len(result))
	return result, nil
}

// parseListenAddress 从命令输出行中提取监听的本地地址和端口
// 兼容ss和netstat两种命令的输出格式，支持IPv4/IPv6/通配符/具体IP地址
// 参数:
//   line - 命令输出的单行内容
// 返回值:
//   ip - 监听IP地址（已去除方括号，如"0.0.0.0"、"192.168.100.53"、"::"等）
//   portStr - 端口字符串
//   matched - 是否成功匹配到监听端口
// 支持的命令输出示例:
//
//	ss命令:
//	  LISTEN 0 128 0.0.0.0:22 0.0.0.0:* users:(("sshd",pid=1234,fd=3))
//	  LISTEN 0 128 192.168.100.53:5437 0.0.0.0:* users:(("vastbase",pid=1925782,fd=3))
//	  LISTEN 0 128 [::]:22 [::]:* users:(("sshd",pid=1234,fd=4))
//	  LISTEN 0 128 :::22 :::* users:(("sshd",pid=1234,fd=4))
//	  LISTEN 0 128 *:80 *:* users:(("nginx",pid=1234,fd=6))
//	netstat命令:
//	  tcp 0 0 0.0.0.0:22 0.0.0.0:* LISTEN 1234/sshd
//	  tcp 0 0 192.168.100.53:5437 0.0.0.0:* LISTEN 1925782/vastbase
//	  tcp6 0 0 :::22 :::* LISTEN 1234/sshd
func parseListenAddress(line string) (ip, portStr string, matched bool) {
	// 匹配ss命令格式：LISTEN Recv-Q Send-Q LocalAddress PeerAddress ...
	// (\S+) 捕获本地地址（含端口），如 0.0.0.0:22 或 192.168.100.53:5437 或 [::]:22
	reSS := regexp.MustCompile(`LISTEN(?:ING)?\s+\d+\s+\d+\s+(\S+)\s+\S+`)
	if matches := reSS.FindStringSubmatch(line); len(matches) == 2 {
		return splitAddrPort(matches[1])
	}

	// 匹配netstat命令格式：tcp[6] Recv-Q Send-Q LocalAddress PeerAddress LISTEN ...
	// (\S+) 捕获本地地址（含端口）
	reNetstat := regexp.MustCompile(`tcp(?:6)?\s+\d+\s+\d+\s+(\S+)\s+\S+\s+LISTEN`)
	if matches := reNetstat.FindStringSubmatch(line); len(matches) == 2 {
		return splitAddrPort(matches[1])
	}

	return "", "", false
}

// splitAddrPort 将本地地址拆分为IP和端口两部分
// 处理IPv4、IPv6（含方括号）、通配符等多种地址格式
// 参数:
//   addr - 本地地址字符串
// 返回值:
//   ip - IP地址（已去除方括号）
//   portStr - 端口字符串
//   ok - 是否成功拆分
// 支持的地址格式示例:
//
//	"0.0.0.0:22"          -> ip="0.0.0.0",        port="22"
//	"192.168.100.53:5437" -> ip="192.168.100.53", port="5437"
//	"127.0.0.1:631"       -> ip="127.0.0.1",      port="631"
//	"[::]:22"             -> ip="::",             port="22"
//	"[::1]:631"           -> ip="::1",            port="631"
//	":::22"               -> ip="::",             port="22"
//	"*:22"                -> ip="*",              port="22"
func splitAddrPort(addr string) (ip, portStr string, ok bool) {
	// 处理带方括号的IPv6地址，如 [::]:22 或 [::1]:631
	if strings.HasPrefix(addr, "[") {
		endIdx := strings.Index(addr, "]")
		if endIdx == -1 {
			return "", "", false
		}
		ip = addr[1:endIdx]
		rest := addr[endIdx+1:]
		// 方括号后应紧跟 ":端口"
		if strings.HasPrefix(rest, ":") {
			return ip, rest[1:], true
		}
		return "", "", false
	}

	// 普通格式：按最后一个冒号拆分IP和端口
	// 对于IPv6全局地址如 ":::22"，最后一个冒号前为 "::"（IPv6任意地址），后为 "22"
	idx := strings.LastIndex(addr, ":")
	if idx == -1 {
		return "", "", false
	}
	return addr[:idx], addr[idx+1:], true
}

// isLoopbackAddress 判断IP地址是否为本地回环地址
// 回环地址仅用于本机内部通信，无需配置访问控制
// 参数:
//   ip - IP地址字符串（已去除方括号）
// 返回值:
//   bool - 是否为回环地址
// 回环地址列表:
//   127.0.0.1 - IPv4回环地址
//   ::1       - IPv6回环地址
//   localhost - 主机名形式的回环地址
func isLoopbackAddress(ip string) bool {
	return ip == "127.0.0.1" || ip == "::1" || ip == "localhost"
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