package detector

import (
	"access-control-tool/internal/models"
	"access-control-tool/internal/smb"
	"log"
	"strings"
)

// DetectWindowsOSAndInfo 检测Windows操作系统及详细信息
// 使用wmic命令获取系统信息，包括系统名称、版本、内核版本、架构和主机名
// 参数: server - 目标服务器信息
// 返回: 系统类型(windows)、系统名称、内核版本、架构、主机名和错误
func DetectWindowsOSAndInfo(server *models.Server) (string, string, string, string, string, error) {
	log.Printf("[Detector] 开始检测Windows操作系统及详细信息，服务器: %s:%d", server.Host, server.SSHPort)

	client, err := smb.NewClient(server)
	if err != nil {
		log.Printf("[Detector] SMB客户端创建失败: %v", err)
		return "", "", "", "", "", err
	}
	defer client.Close()

	log.Printf("[Detector] SMB客户端创建成功，执行wmic命令获取系统信息")

	output, err := client.Execute(`C:\Windows\System32\wbem\wmic.exe os get Caption,Version,OSArchitecture /format:list && C:\Windows\System32\wbem\wmic.exe computersystem get Name /format:list`)
	if err != nil {
		log.Printf("[Detector] wmic命令执行失败: %v", err)
		return "", "", "", "", "", err
	}
	log.Printf("[Detector] wmic输出: %s", output)

	osName := parseWMICValue(output, "Caption")
	kernelVersion := parseWMICValue(output, "Version")
	archCode := parseWMICValue(output, "OSArchitecture")
	hostname := parseWMICValue(output, "Name")

	if osName == "" {
		osName = "Windows"
	}

	arch := mapWMICArch(archCode)

	log.Printf("[Detector] Windows系统信息检测完成 - 系统: windows, 版本: %s, 内核版本: %s, 架构: %s, 主机名: %s", osName, kernelVersion, arch, hostname)
	return "windows", osName, kernelVersion, arch, hostname, nil
}

// parseWMICValue 从wmic输出中解析指定键的值
// wmic输出格式为 "Key=Value"，逐行查找匹配的键并返回对应的值
// 参数: output - wmic命令输出
//       key - 要查找的键名
// 返回: 键对应的值（空字符串表示未找到）
func parseWMICValue(output string, key string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+"=") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+"="))
		}
	}
	return ""
}

// mapWMICArch 将wmic返回的架构代码映射为标准架构名称
// 支持的映射:
// - 包含"64" -> x86_64
// - 包含"32" -> x86
// - 包含"ARM" -> ARM64
// 参数: arch - wmic返回的架构字符串
// 返回: 标准架构名称
func mapWMICArch(arch string) string {
	arch = strings.TrimSpace(arch)
	if strings.Contains(arch, "64") {
		return "x86_64"
	}
	if strings.Contains(arch, "32") {
		return "x86"
	}
	if strings.Contains(arch, "ARM") {
		return "ARM64"
	}
	return "Unknown"
}

// parseCmdValue 从cmd命令输出中解析指定环境变量的值
// 格式为 "VAR=value"，逐行查找匹配的变量名
// 参数: output - cmd命令输出
//       key - 变量名
// 返回: 变量值（空字符串表示未找到）
func parseCmdValue(output string, key string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+"=") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+"="))
		}
	}
	return ""
}

// parseHostname 从命令输出中解析主机名
// 查找不包含"="、"["、":"的非空行作为主机名
// 参数: output - 命令输出
// 返回: 主机名（空字符串表示未找到）
func parseHostname(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(line, "=") && !strings.Contains(line, "[") && !strings.Contains(line, ":") {
			return line
		}
	}
	return ""
}

// parseVerVersion 从ver命令输出中解析Windows版本号
// ver命令输出格式为 "Microsoft Windows [版本 10.0.19045]"
// 参数: output - ver命令输出
// 返回: 版本号（如 "10.0.19045"）
func parseVerVersion(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		idx := strings.Index(line, "[Version ")
		if idx < 0 {
			idx = strings.Index(line, "[版本 ")
		}
		if idx < 0 {
			idx = strings.Index(line, "[")
		}
		if idx >= 0 {
			start := idx + 1
			end := strings.Index(line[start:], "]")
			if end > 0 {
				content := strings.TrimSpace(line[start : start+end])
				content = strings.TrimPrefix(content, "Version ")
				content = strings.TrimPrefix(content, "版本 ")
				return strings.TrimSpace(content)
			}
		}
	}
	return ""
}

// parseOSName 从systeminfo命令输出中解析操作系统名称
// 支持中英文输出格式: "OS Name: xxx" 或 "OS 名称: xxx"
// 参数: output - systeminfo命令输出
// 返回: 操作系统名称（空字符串表示未找到）
func parseOSName(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "OS Name:") || strings.HasPrefix(line, "OS 名称:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

// parseRegValue 从reg query命令输出中解析注册表值
// 查找包含指定值名和"REG_SZ"的行，提取值内容
// 参数: output - reg query命令输出
//       valueName - 注册表值名称
// 返回: 注册表值（空字符串表示未找到）
func parseRegValue(output string, valueName string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, valueName) {
			continue
		}
		idx := strings.Index(line, "REG_SZ")
		if idx < 0 {
			continue
		}
		return strings.TrimSpace(line[idx+len("REG_SZ"):])
	}
	return ""
}

// mapVersionToOSName 根据内核版本号映射操作系统名称
// 通过版本号前缀判断具体的Windows版本，如10.0.22xxx对应Windows 11
// 参数: version - 内核版本号（如 "10.0.19045"）
// 返回: 操作系统名称
func mapVersionToOSName(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "Windows"
	}

	if strings.HasPrefix(version, "10.0.26") {
		return "Windows 11 24H2"
	}
	if strings.HasPrefix(version, "10.0.22") {
		return "Windows 11 22H2"
	}
	if strings.HasPrefix(version, "10.0.20") {
		return "Windows 10 21H2"
	}
	if strings.HasPrefix(version, "10.0.19") {
		return "Windows 10 22H2"
	}
	if strings.HasPrefix(version, "10.0.18") {
		return "Windows 10 1809"
	}
	if strings.HasPrefix(version, "10.0.17") {
		return "Windows 10 1709"
	}
	if strings.HasPrefix(version, "10.0.16") {
		return "Windows 10 1607"
	}
	if strings.HasPrefix(version, "10.0.15") {
		return "Windows 10 1511"
	}
	if strings.HasPrefix(version, "10.0.14") {
		return "Windows Server 2016"
	}
	if strings.HasPrefix(version, "10.0.10") {
		return "Windows 10"
	}
	if strings.HasPrefix(version, "6.3") {
		return "Windows 8.1"
	}
	if strings.HasPrefix(version, "6.2") {
		return "Windows 8"
	}
	if strings.HasPrefix(version, "6.1") {
		return "Windows 7"
	}
	if strings.HasPrefix(version, "6.0") {
		return "Windows Vista"
	}
	if strings.HasPrefix(version, "5.2") {
		return "Windows Server 2003"
	}
	if strings.HasPrefix(version, "5.1") {
		return "Windows XP"
	}
	if strings.HasPrefix(version, "5.0") {
		return "Windows 2000"
	}

	return "Windows " + version
}

// mapArchCode 将PROCESSOR_ARCHITECTURE环境变量值映射为标准架构名称
// 支持的映射:
// - AMD64 -> x86_64
// - X86 -> x86
// - ARM64 -> ARM64
// - ARM -> ARM
// - IA64 -> IA64
// 参数: code - PROCESSOR_ARCHITECTURE值
// 返回: 标准架构名称
func mapArchCode(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "AMD64":
		return "x86_64"
	case "X86":
		return "x86"
	case "ARM64":
		return "ARM64"
	case "ARM":
		return "ARM"
	case "IA64":
		return "IA64"
	default:
		if code == "" {
			return "Unknown"
		}
		return code
	}
}

// parseValueFromLine 从冒号分隔的行中解析值部分
// 格式为 "key: value"，返回冒号后的内容
// 参数: line - 单行文本
// 返回: 值部分（空字符串表示格式不正确）
func parseValueFromLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	parts := strings.SplitN(line, ":", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

// DetectSystemInfoWindows 检测Windows系统基础信息（简化版）
// 使用ver、echo PROCESSOR_ARCHITECTURE和hostname命令获取信息
// 参数: server - 目标服务器信息
// 返回: 内核版本、架构、主机名、空字符串和错误
func DetectSystemInfoWindows(server *models.Server) (string, string, string, string, error) {
	log.Printf("[Detector] 检测Windows系统信息")

	client, err := smb.NewClient(server)
	if err != nil {
		log.Printf("[Detector] SMB客户端创建失败: %v", err)
		return "", "", "", "", err
	}
	defer client.Close()

	output, err := client.Execute(`ver && echo PROCESSOR_ARCHITECTURE=%PROCESSOR_ARCHITECTURE% && hostname`)
	if err != nil {
		log.Printf("[Detector] 获取Windows系统信息失败: %v", err)
		return "", "", "", "", err
	}

	kernelVersion := parseVerVersion(output)
	archCode := parseCmdValue(output, "PROCESSOR_ARCHITECTURE")
	hostname := parseHostname(output)
	arch := mapArchCode(archCode)

	log.Printf("[Detector] Windows系统信息检测完成 - 内核版本: %s, 架构: %s, 主机名: %s", kernelVersion, arch, hostname)
	return kernelVersion, arch, hostname, "", nil
}