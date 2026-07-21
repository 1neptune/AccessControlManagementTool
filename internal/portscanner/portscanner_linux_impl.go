package portscanner

import (
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