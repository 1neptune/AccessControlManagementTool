package portscanner

import (
	"access-control-tool/internal/models"
	"access-control-tool/internal/ssh"
	"fmt"
	"log"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// scanLinuxPorts 扫描Linux服务器上的监听端口
// 参数:
//
//	client - SSH客户端
//
// 返回值:
//
//	[]PortInfo - 端口信息列表
//	error - 扫描过程中的错误
//
// 扫描范围:
//   - TCP监听端口：ss -tlnp / netstat -tlnp
//   - UDP监听端口：ss -ulnp / netstat -ulnp
//   - 包含监听全局地址的端口（0.0.0.0, ::, :::, *）
//   - 包含监听非回环具体IP地址的端口（如 192.168.100.53:5437）
//   - 排除仅监听本地回环地址的端口（127.0.0.1, ::1, ::ffff:127.0.0.1 等）
//   - 支持IPv4和IPv6协议
//   - 支持ss和netstat两种命令输出格式
//   - 以"协议+端口"为唯一键去重：同一端口的TCP和UDP监听分别作为独立记录
//   - 获取已建立连接的客户端IP列表（仅TCP）
func scanLinuxPorts(client *ssh.Client) ([]PortInfo, error) {
	log.Printf("[PortScanner] 开始扫描Linux端口（TCP/UDP，包含非回环地址监听）")

	// portMap 以"协议/端口号"作为key存储端口信息，实现协议+端口双重去重
	// 同一端口同时在TCP和UDP上监听时，分别保留一条独立记录
	portMap := make(map[string]*PortInfo)

	// 执行命令获取TCP监听端口信息（优先ss，失败降级到netstat）
	log.Printf("[PortScanner] 执行命令扫描TCP监听: ss -tlnp / netstat -tlnp")
	tcpOutput, err := client.Execute("ss -tlnp 2>/dev/null || netstat -tlnp 2>/dev/null")
	if err != nil {
		log.Printf("[PortScanner] Linux TCP端口扫描失败: %v", err)
		return nil, err
	}
	log.Printf("[PortScanner] TCP命令执行成功，输出长度: %d 字节", len(tcpOutput))

	// 执行命令获取UDP监听端口信息（优先ss，失败降级到netstat）
	log.Printf("[PortScanner] 执行命令扫描UDP监听: ss -ulnp / netstat -ulnp")
	udpOutput, err := client.Execute("ss -ulnp 2>/dev/null || netstat -ulnp 2>/dev/null")
	if err != nil {
		log.Printf("[PortScanner] Linux UDP端口扫描失败: %v", err)
		return nil, err
	}
	log.Printf("[PortScanner] UDP命令执行成功，输出长度: %d 字节", len(udpOutput))

	parseListenLines(tcpOutput, "tcp", portMap)
	parseListenLines(udpOutput, "udp", portMap)

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
		if result[i].Port != result[j].Port {
			return result[i].Port < result[j].Port
		}
		return result[i].Protocol < result[j].Protocol
	})

	log.Printf("[PortScanner] Linux端口扫描完成，共发现 %d 个监听端口（TCP/UDP，按协议+端口去重）", len(result))
	return result, nil
}

// parseListenLines 解析指定协议的监听端口命令行，将结果写入portMap
// 参数:
//
//	output   - 命令输出内容
//	protocol - 协议类型（tcp/udp）
//	portMap  - 端口信息映射表（key为"协议/端口号"）
func parseListenLines(output string, protocol string, portMap map[string]*PortInfo) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		// 跳过空行和表头行（ss输出以"State"开头，netstat输出以"Active"开头）
		if line == "" || strings.HasPrefix(line, "State") || strings.HasPrefix(line, "Active") {
			continue
		}
		log.Printf("[PortScanner] 处理%s行: %s", protocol, line)

		// 从当前行提取监听地址和端口（兼容ss和netstat两种格式）
		ip, portStr, matched := parseListenAddress(line, protocol)
		if !matched {
			log.Printf("[PortScanner] 跳过非%s监听端口行: %s", protocol, line)
			continue
		}

		// 排除回环地址监听的端口，这些端口仅本机访问无需配置访问控制
		// 回环地址包括：127.0.0.1、::1、::ffff:127.0.0.1（IPv4映射IPv6形式，常见于tcp6监听IPv4回环）
		if isLoopbackAddress(ip) {
			log.Printf("[PortScanner] 跳过回环地址监听端口: %s:%s", ip, portStr)
			continue
		}

		// 处理端口行，将端口信息加入portMap（按"协议/端口号"去重）
		processPortLine(line, portStr, protocol, portMap)
	}
}

// parseListenAddress 从命令输出行中提取监听的本地地址和端口
// 兼容ss和netstat两种命令的输出格式，支持IPv4/IPv6/通配符/具体IP地址
// 参数:
//
//	line      - 命令输出的单行内容
//	protocol  - 协议类型（tcp/udp），决定匹配的监听状态关键字
//
// 返回值:
//
//	ip - 监听IP地址（已去除方括号，如"0.0.0.0"、"192.168.100.53"、"::"等）
//	portStr - 端口字符串
//	matched - 是否成功匹配到监听端口
//
// 支持的命令输出示例:
//
//	ss命令(TCP):
//	  LISTEN 0 128 0.0.0.0:22 0.0.0.0:* users:(("sshd",pid=1234,fd=3))
//	  LISTEN 0 128 192.168.100.53:5437 0.0.0.0:* users:(("vastbase",pid=1925782,fd=3))
//	  LISTEN 0 128 [::]:22 [::]:* users:(("sshd",pid=1234,fd=4))
//	  LISTEN 0 128 [::ffff:127.0.0.1]:8105 [::]:* users:(("java",pid=281403,fd=39))
//	  LISTEN 0 128 :::22 :::* users:(("sshd",pid=1234,fd=4))
//	  LISTEN 0 128 *:80 *:* users:(("nginx",pid=1234,fd=6))
//	ss命令(UDP):
//	  UNCONN 0 0 0.0.0.0:53 0.0.0.0:* users:(("systemd-resolve",pid=789,fd=15))
//	  UNCONN 0 0 [::]:53 [::]:* users:(("systemd-resolve",pid=789,fd=17))
//	netstat命令(TCP):
//	  tcp 0 0 0.0.0.0:22 0.0.0.0:* LISTEN 1234/sshd
//	  tcp 0 0 192.168.100.53:5437 0.0.0.0:* LISTEN 1925782/vastbase
//	  tcp6 0 0 127.0.0.1:8105 :::* LISTEN 281403/java
//	  tcp6 0 0 :::22 :::* LISTEN 1234/sshd
//	netstat命令(UDP):
//	  udp 0 0 0.0.0.0:53 0.0.0.0:* 789/systemd-resolve
//	  udp6 0 0 :::53 :::* 789/systemd-resolve
func parseListenAddress(line string, protocol string) (ip, portStr string, matched bool) {
	if protocol == "udp" {
		// 匹配ss命令UDP格式：UNCONN Recv-Q Send-Q LocalAddress PeerAddress ...
		reSS := regexp.MustCompile(`UNCONN\s+\d+\s+\d+\s+(\S+)\s+\S+`)
		if matches := reSS.FindStringSubmatch(line); len(matches) == 2 {
			return splitAddrPort(matches[1])
		}

		// 匹配netstat命令UDP格式：udp[6] Recv-Q Send-Q LocalAddress PeerAddress [State] Pid/Program
		// 注意: netstat -ulnp 输出没有独立状态列（UDP无状态），进程列直接跟在远程地址后
		reNetstat := regexp.MustCompile(`udp(?:6)?\s+\d+\s+\d+\s+(\S+)\s+\S+`)
		if matches := reNetstat.FindStringSubmatch(line); len(matches) == 2 {
			return splitAddrPort(matches[1])
		}

		return "", "", false
	}

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
//
//	addr - 本地地址字符串
//
// 返回值:
//
//	ip - IP地址（已去除方括号）
//	portStr - 端口字符串
//	ok - 是否成功拆分
//
// 支持的地址格式示例:
//
//	"0.0.0.0:22"                  -> ip="0.0.0.0",            port="22"
//	"192.168.100.53:5437"         -> ip="192.168.100.53",     port="5437"
//	"127.0.0.1:631"               -> ip="127.0.0.1",          port="631"
//	"[::]:22"                     -> ip="::",                 port="22"
//	"[::1]:631"                   -> ip="::1",                port="631"
//	"[::ffff:127.0.0.1]:8105"     -> ip="::ffff:127.0.0.1",   port="8105"
//	":::22"                       -> ip="::",                 port="22"
//	"*:22"                        -> ip="*",                  port="22"
func splitAddrPort(addr string) (ip, portStr string, ok bool) {
	// 处理带方括号的IPv6地址，如 [::]:22 或 [::1]:631 或 [::ffff:127.0.0.1]:8105
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
	// 对于IPv4映射IPv6地址如 "::ffff:127.0.0.1:8105"，最后一个冒号前为 "::ffff:127.0.0.1"，后为 "8105"
	idx := strings.LastIndex(addr, ":")
	if idx == -1 {
		return "", "", false
	}
	return addr[:idx], addr[idx+1:], true
}

// isLoopbackAddress 判断IP地址是否为本地回环地址
// 回环地址仅用于本机内部通信，无需配置访问控制，应被排除
// 参数:
//
//	ip - IP地址字符串（已去除方括号）
//
// 返回值:
//
//	bool - 是否为回环地址
//
// 识别范围（覆盖IPv4/IPv6/混合映射形式）:
//   - 127.0.0.1 以及整个 127.0.0.0/8 回环网段（如 127.0.0.1）
//   - ::1             - IPv6回环地址
//   - ::ffff:127.0.0.1 - IPv4映射IPv6回环地址（tcp6 监听 IPv4 回环地址时的常见格式）
//   - localhost       - 主机名形式的回环地址
//
// 实现说明:
//   - 优先使用标准库 net.IP.IsLoopback() 判断，能自动识别所有IPv4/IPv6/IPv4映射的回环地址。
//     例如 "::ffff:127.0.0.1" 经 net.ParseIP 解析后，To4() 可还原为 127.x.x.x，从而正确判定为回环。
//   - 通配符地址（"0.0.0.0"、"::"、"*"）不属于回环地址：
//     它们是全局监听地址，需要纳入配置范围，IsLoopback() 对其返回 false，不会被误伤。
func isLoopbackAddress(ip string) bool {
	// 主机名形式的回环地址无法用IP解析，单独处理
	if ip == "localhost" {
		return true
	}

	// 使用标准库解析IP并判断是否为回环地址
	// ParseIP 解析失败（如 "*"）或通配符地址（0.0.0.0、::）均返回 false
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.IsLoopback()
}

// fetchConnectedIPs 获取已建立连接的客户端IP列表（仅TCP）
// 参数:
//
//	client - SSH客户端
//	portMap - 端口信息映射表（key为"协议/端口号"），用于填充ConnectedIPs字段
//
// 返回值:
//
//	error - 执行过程中的错误
//
// 支持的命令格式:
//
//	ss -tn: ESTAB 0 0 172.19.6.75:22 192.168.21.52:57436
//	netstat -tn: tcp 0 0 192.168.3.47:46422 192.168.21.52:58155 ESTABLISHED
func fetchConnectedIPs(client *ssh.Client, portMap map[string]*PortInfo) error {
	// 分别扫描TCP和UDP已建立连接（UDP仅匹配已与对端建立关联的socket）
	tcpOutput, err := client.Execute("ss -tn 2>/dev/null || netstat -tn 2>/dev/null")
	if err != nil {
		return err
	}
	log.Printf("[PortScanner] TCP连接状态输出长度: %d 字节", len(tcpOutput))

	udpOutput, err := client.Execute("ss -un 2>/dev/null || netstat -un 2>/dev/null")
	if err != nil {
		log.Printf("[PortScanner] UDP连接状态获取失败: %v", err)
	}
	log.Printf("[PortScanner] UDP连接状态输出长度: %d 字节", len(udpOutput))

	parseConnections(tcpOutput, "tcp", portMap)
	parseConnections(udpOutput, "udp", portMap)

	return nil
}

// parseConnections 解析已建立连接的命令输出，将远端客户端IP加入portMap对应端口
// 参数:
//
//	output   - 命令输出内容（ss或netstat格式）
//	protocol - 协议类型（tcp/udp）
//	portMap  - 端口信息映射表（key为"协议/端口号"）
//
// 支持的输出格式:
//
//	ss:
//	  ESTAB 0 0 192.168.30.63:1521 192.168.30.54:50582
//	  ESTAB 0 0 [::1]:1521 [::1]:50582
//	netstat tcp:
//	  tcp  0 0 192.168.30.63:1521 192.168.30.54:50582 ESTABLISHED 2471393/oracleV80
//	  tcp6 0 0 192.168.30.63:1521 192.168.30.54:50582 ESTABLISHED 2471393/oracleV80
//	netstat udp:
//	  udp  0 0 192.168.30.63:1521 192.168.30.54:50582
//	  udp6 0 0 192.168.30.63:1521 192.168.30.54:50582
func parseConnections(output string, protocol string, portMap map[string]*PortInfo) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "State") || strings.HasPrefix(line, "Active") {
			continue
		}

		log.Printf("[PortScanner] 处理%s连接行: %s", protocol, line)

		// 通用地址匹配：本地地址(IP:端口) 远端地址(IP:端口)
		// 兼容IPv4/IPv6（IPv6地址带[]，如[::1]:1521）
		addrRe := `([0-9a-fA-F:.]+|\[[0-9a-fA-F:.]+\]):(\d+)\s+([0-9a-fA-F:.]+|\[[0-9a-fA-F:.]+\]):(\d+)`

		var re *regexp.Regexp
		if protocol == "tcp" {
			// 匹配ss命令格式: ESTAB 0 0 local remote（TCP状态固定为ESTAB）
			reSS := regexp.MustCompile(`ESTAB(?:LISHED)?\s+\d+\s+\d+\s+` + addrRe)
			if matches := reSS.FindStringSubmatch(line); len(matches) == 5 {
				localPort, _ := strconv.Atoi(matches[2])
				addConnectedIP(portMap, protocol, localPort, matches[3])
				continue
			}
			// 匹配netstat命令格式: tcp[6] 0 0 local remote ESTABLISHED pid/proc
			re = regexp.MustCompile(`tcp(?:6)?\s+\d+\s+\d+\s+` + addrRe + `\s+ESTABLISHED`)
		} else {
			// 匹配ss命令格式: ESTAB 0 0 local remote（UDP有对端时状态为ESTAB）
			reSS := regexp.MustCompile(`ESTAB(?:LISHED)?\s+\d+\s+\d+\s+` + addrRe)
			if matches := reSS.FindStringSubmatch(line); len(matches) == 5 {
				localPort, _ := strconv.Atoi(matches[2])
				addConnectedIP(portMap, protocol, localPort, matches[3])
				continue
			}
			// 匹配netstat命令格式: udp[6] 0 0 local remote [ESTABLISHED] pid/proc
			// UDP未建立关联时无状态列且远端为0.0.0.0:*，无法匹配(\d+)，自然跳过
			re = regexp.MustCompile(`udp(?:6)?\s+\d+\s+\d+\s+` + addrRe)
		}

		if matches := re.FindStringSubmatch(line); len(matches) == 5 {
			localPort, _ := strconv.Atoi(matches[2])
			addConnectedIP(portMap, protocol, localPort, matches[3])
		}
	}
}

// addConnectedIP 将远程客户端IP添加到端口的ConnectedIPs列表中（去重）
// 参数:
//
//	portMap - 端口信息映射表（key为"协议/端口号"）
//	protocol - 协议类型（tcp/udp）
//	localPort - 本地端口号
//	remoteIP - 远程客户端IP地址
func addConnectedIP(portMap map[string]*PortInfo, protocol string, localPort int, remoteIP string) {
	key := fmt.Sprintf("%s/%d", protocol, localPort)
	if portMap[key] == nil {
		return
	}

	if strings.HasPrefix(remoteIP, "[::1]") || remoteIP == "127.0.0.1" || remoteIP == "::1" || remoteIP == "0.0.0.0" || remoteIP == "::" || remoteIP == "*" {
		return
	}

	if strings.Contains(remoteIP, "[") && strings.Contains(remoteIP, "]") {
		start := strings.Index(remoteIP, "[") + 1
		end := strings.Index(remoteIP, "]")
		if start < end {
			remoteIP = remoteIP[start:end]
		}
	}

	// 处理IPv4映射IPv6地址（IPv4-Mapped IPv6），如 ::ffff:192.168.21.52 -> 192.168.21.52
	remoteIP = strings.TrimPrefix(remoteIP, "::ffff:")

	for _, ip := range portMap[key].ConnectedIPs {
		if ip == remoteIP {
			return
		}
	}

	portMap[key].ConnectedIPs = append(portMap[key].ConnectedIPs, remoteIP)
	log.Printf("[PortScanner] 端口 %d (%s) 发现连接IP: %s", localPort, protocol, remoteIP)
}

// refreshConnectedIPsLinux Linux平台刷新指定端口的连接IP
// 参数:
//
//	server - 服务器信息
//	port - 端口号
//
// 返回值:
//
//	[]string - 客户端IP列表
//	error - 执行过程中的错误
func refreshConnectedIPsLinux(server *models.Server, port int) ([]string, error) {
	return GetConnectedIPsForPort(server, port)
}

// GetConnectedIPsForPort 获取指定端口的已建立连接客户端IP列表
// 参数:
//
//	server - 服务器信息
//	port - 端口号
//
// 返回值:
//
//	[]string - 客户端IP列表
//	error - 执行过程中的错误
//
// 用途:
//   - 用于配置窗口切换端口时快速刷新关联业务IP
//   - 避免重新扫描所有端口，提高性能
func GetConnectedIPsForPort(server *models.Server, port int) ([]string, error) {
	log.Printf("[PortScanner] 获取端口 %d (TCP/UDP) 的连接IP", port)

	client, err := ssh.NewClient(server)
	if err != nil {
		log.Printf("[PortScanner] 创建SSH客户端失败: %v", err)
		return nil, err
	}
	defer client.Close()

	// 分别扫描TCP和UDP已建立连接
	tcpOutput, err := client.Execute("ss -tn 2>/dev/null || netstat -tn 2>/dev/null")
	if err != nil {
		log.Printf("[PortScanner] 执行命令失败: %v", err)
		return nil, err
	}

	udpOutput, err := client.Execute("ss -un 2>/dev/null || netstat -un 2>/dev/null")
	if err != nil {
		log.Printf("[PortScanner] UDP连接状态获取失败: %v", err)
	}

	log.Printf("[PortScanner] TCP连接状态输出长度: %d 字节, UDP连接状态输出长度: %d 字节", len(tcpOutput), len(udpOutput))

	connectedIPs := make([]string, 0)
	seenIPs := make(map[string]bool)

	// 解析TCP连接（ss格式 + netstat tcp/tcp6格式）
	parsePortConnections(tcpOutput, "tcp", port, &connectedIPs, seenIPs)
	// 解析UDP连接（ss格式 + netstat udp/udp6格式）
	parsePortConnections(udpOutput, "udp", port, &connectedIPs, seenIPs)

	log.Printf("[PortScanner] 端口 %d 发现 %d 个连接IP", port, len(connectedIPs))
	return connectedIPs, nil
}

// parsePortConnections 解析指定端口的已建立连接客户端IP（支持TCP/UDP及其IPv6形式）
// 参数:
//
//	output       - 命令输出内容（ss或netstat格式）
//	protocol     - 协议类型（tcp/udp）
//	port         - 目标端口号
//	connectedIPs - 客户端IP列表（会去重追加）
//	seenIPs      - 已出现IP集合
func parsePortConnections(output string, protocol string, port int, connectedIPs *[]string, seenIPs map[string]bool) {
	// 通用地址匹配：本地地址(IP:端口) 远端地址(IP:端口)
	// 兼容IPv4/IPv6（IPv6地址带[]，如[::1]:1521）
	addrRe := `([0-9a-fA-F:.]+|\[[0-9a-fA-F:.]+\]):(\d+)\s+([0-9a-fA-F:.]+|\[[0-9a-fA-F:.]+\]):(\d+)`

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "State") || strings.HasPrefix(line, "Active") {
			continue
		}

		log.Printf("[PortScanner] 处理%s连接行: %s", protocol, line)

		var re *regexp.Regexp
		if protocol == "tcp" {
			// ss格式: ESTAB 0 0 local remote
			reSS := regexp.MustCompile(`ESTAB(?:LISHED)?\s+\d+\s+\d+\s+` + addrRe)
			if matches := reSS.FindStringSubmatch(line); len(matches) == 5 {
				appendPortConnectedIP(matches[2], matches[3], port, connectedIPs, seenIPs)
				continue
			}
			// netstat格式: tcp[6] 0 0 local remote ESTABLISHED pid/proc
			re = regexp.MustCompile(`tcp(?:6)?\s+\d+\s+\d+\s+` + addrRe + `\s+ESTABLISHED`)
		} else {
			// ss格式: ESTAB 0 0 local remote（UDP有对端时状态为ESTAB）
			reSS := regexp.MustCompile(`ESTAB(?:LISHED)?\s+\d+\s+\d+\s+` + addrRe)
			if matches := reSS.FindStringSubmatch(line); len(matches) == 5 {
				appendPortConnectedIP(matches[2], matches[3], port, connectedIPs, seenIPs)
				continue
			}
			// netstat格式: udp[6] 0 0 local remote [ESTABLISHED] pid/proc
			re = regexp.MustCompile(`udp(?:6)?\s+\d+\s+\d+\s+` + addrRe)
		}

		if matches := re.FindStringSubmatch(line); len(matches) == 5 {
			appendPortConnectedIP(matches[2], matches[3], port, connectedIPs, seenIPs)
		}
	}
}

// appendPortConnectedIP 将匹配到的连接行中的远端IP追加到列表（校验端口并去重）
// 参数:
//
//	localPortStr - 本地端口字符串
//	remoteIP     - 远端IP（可能带方括号）
//	port         - 目标端口号
//	connectedIPs - 客户端IP列表
//	seenIPs      - 已出现IP集合
func appendPortConnectedIP(localPortStr, remoteIP string, port int, connectedIPs *[]string, seenIPs map[string]bool) {
	localPort, err := strconv.Atoi(localPortStr)
	if err != nil {
		return
	}
	if localPort != port {
		return
	}
	if seenIPs[remoteIP] {
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

	// 处理IPv4映射IPv6地址（IPv4-Mapped IPv6），如 ::ffff:192.168.21.52 -> 192.168.21.52
	remoteIP = strings.TrimPrefix(remoteIP, "::ffff:")

	// 可能映射地址为通配/回环，再次过滤
	if remoteIP == "127.0.0.1" || remoteIP == "0.0.0.0" {
		return
	}
	*connectedIPs = append(*connectedIPs, remoteIP)
	seenIPs[remoteIP] = true
}

// processPortLine 处理端口行，提取端口号、协议和服务名称
// 参数:
//
//	line     - 原始行内容
//	portStr  - 端口号字符串
//	protocol - 协议类型（tcp/udp）
//	portMap  - 端口信息映射表（key为"协议/端口号"）
func processPortLine(line, portStr string, protocol string, portMap map[string]*PortInfo) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return
	}

	key := fmt.Sprintf("%s/%d", protocol, port)

	if portMap[key] == nil {
		portMap[key] = &PortInfo{
			Port:         port,
			Protocol:     protocol,
			ServiceName:  extractServiceName(line),
			ConnectedIPs: make([]string, 0),
		}
		log.Printf("[PortScanner] 发现全局监听端口: %d (%s)", port, protocol)
	} else {
		if name := extractServiceName(line); name != "" && portMap[key].ServiceName == "" {
			portMap[key].ServiceName = name
		}
	}
}

// extractServiceName 从命令输出行中提取服务名称
// 参数:
//
//	line - 原始行内容
//
// 返回值:
//
//	string - 服务名称，未找到返回空字符串
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
