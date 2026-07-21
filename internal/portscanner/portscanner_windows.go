package portscanner

import (
	"access-control-tool/internal/smb"
	"log"
	"strconv"
	"strings"
)

// scanWindowsPorts 扫描Windows服务器上的端口信息
// 扫描流程:
// 1. 执行netstat -ano命令获取端口监听和连接状态
// 2. 解析输出，提取非本地回环的监听端口和已建立连接的远程IP
// 3. 通过wmic process get ProcessId,Name获取进程名称映射
// 4. 将进程名称关联到对应的端口
// 5. 按端口号排序后返回结果
// 参数: client - SMB客户端实例
// 返回: 端口信息列表和错误
func scanWindowsPorts(client *smb.Client) ([]PortInfo, error) {
	log.Printf("[PortScanner] 开始扫描Windows端口")

	log.Printf("[PortScanner] 执行命令: netstat -ano")
	output, err := client.Execute("netstat -ano")
	if err != nil {
		log.Printf("[PortScanner] Windows端口扫描失败: %v", err)
		return nil, err
	}

	log.Printf("[PortScanner] 命令执行成功，输出长度: %d 字节", len(output))

	portMap := make(map[int]*PortInfo)
	pidToPort := make(map[string][]int)
	lines := strings.Split(output, "\n")
	log.Printf("[PortScanner] 解析输出，共 %d 行", len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}

		state := parts[3]
		localAddr := parts[1]
		remoteAddr := parts[2]
		pid := parts[4]

		if strings.HasPrefix(localAddr, "127.0.0.1:") || strings.HasPrefix(localAddr, "[::1]:") {
			continue
		}

		portStr := localAddr
		if idx := strings.LastIndex(portStr, ":"); idx != -1 {
			portStr = portStr[idx+1:]
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}

		if portMap[port] == nil {
			portMap[port] = &PortInfo{Port: port, ServiceName: "", ConnectedIPs: make([]string, 0)}
		}

		if state == "LISTENING" {
			log.Printf("[PortScanner] 发现监听端口: %d", port)
			pidToPort[pid] = append(pidToPort[pid], port)
		} else if state == "ESTABLISHED" {
			remoteIP := remoteAddr
			if idx := strings.LastIndex(remoteIP, ":"); idx != -1 {
				remoteIP = remoteIP[:idx]
			}
			if remoteIP != "0.0.0.0" && remoteIP != "::" && !contains(portMap[port].ConnectedIPs, remoteIP) {
				portMap[port].ConnectedIPs = append(portMap[port].ConnectedIPs, remoteIP)
				log.Printf("[PortScanner] 端口 %d 发现ESTABLISHED连接IP: %s", port, remoteIP)
			}
		}
	}

	log.Printf("[PortScanner] 开始获取端口服务名称，共 %d 个PID需要查询", len(pidToPort))

	if len(pidToPort) > 0 {
		output, _ := client.Execute(`wmic process get ProcessId,Name /format:list`)
		pidToProcess := make(map[string]string)
		var currentName string

		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || line == "ProcessId" || line == "Name" {
				currentName = ""
				continue
			}

			if strings.HasPrefix(line, "Name=") {
				currentName = strings.TrimSpace(strings.TrimPrefix(line, "Name="))
			} else if strings.HasPrefix(line, "ProcessId=") && currentName != "" {
				pid := strings.TrimSpace(strings.TrimPrefix(line, "ProcessId="))
				if pid != "" {
					pidToProcess[pid] = currentName
				}
				currentName = ""
			}
		}
		log.Printf("[PortScanner] 解析进程列表，共发现 %d 个进程", len(pidToProcess))

		for pid, ports := range pidToPort {
			serviceName := pidToProcess[pid]
			if serviceName == "" {
				serviceName = "未知"
			}
			log.Printf("[PortScanner] PID %s 的进程名: %s", pid, serviceName)

			for _, port := range ports {
				if portMap[port] != nil && portMap[port].ServiceName == "" {
					portMap[port].ServiceName = serviceName
					log.Printf("[PortScanner] 端口 %d 服务名称: %s", port, serviceName)
				}
			}
		}
	}

	result := make([]PortInfo, 0, len(portMap))
	for _, info := range portMap {
		result = append(result, *info)
	}

	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Port > result[j].Port {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	log.Printf("[PortScanner] Windows端口扫描完成，共发现 %d 个端口", len(result))
	return result, nil
}

