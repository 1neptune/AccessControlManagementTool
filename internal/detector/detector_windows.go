package detector

import (
	"access-control-tool/internal/models"
	"access-control-tool/internal/smb"
	"log"
	"strings"
)

// DetectWindowsOSAndInfo 检测Windows操作系统及详细信息（完整版）
// 使用wmic命令获取系统信息，包括系统名称、版本、内核版本、架构和主机名
// 参数:
//   server - 目标服务器信息，包含主机地址、端口、用户名、密码等
// 返回值:
//   string - 系统类型（固定返回"windows"）
//   string - 系统名称（如 "Microsoft Windows Server 2019"）
//   string - 内核版本（如 "10.0.17763"）
//   string - 系统架构（如 "x86_64"）
//   string - 主机名（服务器的主机名称）
//   error - 检测过程中的错误，连接失败或命令执行失败时返回
// 执行流程:
//   1. 创建SMB客户端并建立连接
//   2. 执行wmic命令获取系统信息：
//      - wmic os get Caption,Version,OSArchitecture /format:list
//      - wmic computersystem get Name /format:list
//   3. 解析命令输出，提取系统名称、内核版本、架构和主机名
//   4. 将架构代码映射为标准名称
//   5. 返回检测结果
// 注意点:
//   - 使用defer client.Close()确保SMB连接在函数结束时关闭
//   - wmic命令需要管理员权限才能执行
//   - 如果系统名称为空，默认使用"Windows"
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

// parseWMICValue 从wmic命令输出中解析指定键的值
// wmic命令使用/format:list时输出格式为 "Key=Value"，逐行查找匹配的键
// 参数:
//   output - wmic命令的输出文本
//   key - 要查找的键名（如 "Caption"、"Version"、"OSArchitecture"）
// 返回值:
//   string - 键对应的值（空字符串表示未找到）
// 解析逻辑:
//   1. 将输出按换行符分割为行
//   2. 遍历每行，查找以"key="开头的行
//   3. 提取"key="之后的内容，去除首尾空白
//   4. 返回提取的值
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
// wmic返回的OSArchitecture字段值可能为"64-bit"、"32-bit"、"ARM64"等
// 参数:
//   arch - wmic返回的架构字符串（如 "64-bit"、"32-bit"）
// 返回值:
//   string - 标准架构名称（如 "x86_64"、"x86"、"ARM64"）
// 映射规则:
//   - 包含"64" -> x86_64
//   - 包含"32" -> x86
//   - 包含"ARM" -> ARM64
//   - 其他 -> Unknown
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
// 参数:
//   output - cmd命令的输出文本（如 "echo PROCESSOR_ARCHITECTURE=%PROCESSOR_ARCHITECTURE%" 的输出）
//   key - 环境变量名（如 "PROCESSOR_ARCHITECTURE"）
// 返回值:
//   string - 环境变量的值（空字符串表示未找到）
// 解析逻辑:
//   1. 将输出按换行符分割为行
//   2. 遍历每行，查找以"key="开头的行
//   3. 提取"key="之后的内容，去除首尾空白
//   4. 返回提取的值
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
// 参数:
//   output - 命令输出文本（如 hostname 命令的输出）
// 返回值:
//   string - 主机名（空字符串表示未找到）
// 解析逻辑:
//   1. 将输出按换行符分割为行
//   2. 遍历每行，跳过空行
//   3. 查找不包含"="、"["、":"的行（排除变量赋值、版本信息等）
//   4. 返回找到的第一行作为主机名
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
// ver命令输出格式为 "Microsoft Windows [版本 10.0.19045]"（中文）或 "Microsoft Windows [Version 10.0.19045]"（英文）
// 参数:
//   output - ver命令的输出文本
// 返回值:
//   string - 版本号（如 "10.0.19045"）
// 解析逻辑:
//   1. 将输出按换行符分割为行
//   2. 遍历每行，查找包含"[Version "、"[版本 "或"["的行
//   3. 提取方括号内的内容，去除"Version "或"版本 "前缀
//   4. 返回版本号
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
// 支持中英文输出格式: "OS Name: xxx"（英文）或 "OS 名称: xxx"（中文）
// 参数:
//   output - systeminfo命令的输出文本
// 返回值:
//   string - 操作系统名称（空字符串表示未找到）
// 解析逻辑:
//   1. 将输出按换行符分割为行
//   2. 遍历每行，查找以"OS Name:"或"OS 名称:"开头的行
//   3. 使用":"分割，提取第二部分
//   4. 返回提取的名称
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
// 参数:
//   output - reg query命令的输出文本
//   valueName - 注册表值名称（如 "ProductName"）
// 返回值:
//   string - 注册表值（空字符串表示未找到）
// 解析逻辑:
//   1. 将输出按换行符分割为行
//   2. 遍历每行，查找以valueName开头的行
//   3. 在该行中查找"REG_SZ"标记
//   4. 提取"REG_SZ"之后的内容
//   5. 返回提取的值
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
// 参数:
//   version - 内核版本号（如 "10.0.19045"）
// 返回值:
//   string - 操作系统名称（如 "Windows 10 22H2"、"Windows Server 2019"）
// 映射规则:
//   - 10.0.26xxx -> Windows 11 24H2
//   - 10.0.22xxx -> Windows 11 22H2
//   - 10.0.20xxx -> Windows 10 21H2
//   - 10.0.19xxx -> Windows 10 22H2
//   - 10.0.18xxx -> Windows 10 1809
//   - 10.0.17xxx -> Windows 10 1709
//   - 10.0.16xxx -> Windows 10 1607
//   - 10.0.15xxx -> Windows 10 1511
//   - 10.0.14xxx -> Windows Server 2016
//   - 10.0.10xxx -> Windows 10
//   - 6.3xxx -> Windows 8.1
//   - 6.2xxx -> Windows 8
//   - 6.1xxx -> Windows 7
//   - 6.0xxx -> Windows Vista
//   - 5.2xxx -> Windows Server 2003
//   - 5.1xxx -> Windows XP
//   - 5.0xxx -> Windows 2000
//   - 其他 -> "Windows " + 版本号
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
// PROCESSOR_ARCHITECTURE是Windows环境变量，值可能为AMD64、X86、ARM64等
// 参数:
//   code - PROCESSOR_ARCHITECTURE环境变量的值（如 "AMD64"、"X86"）
// 返回值:
//   string - 标准架构名称（如 "x86_64"、"x86"、"ARM64"）
// 映射规则:
//   - AMD64 -> x86_64
//   - X86 -> x86
//   - ARM64 -> ARM64
//   - ARM -> ARM
//   - IA64 -> IA64
//   - 空字符串 -> Unknown
//   - 其他 -> 原值
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
// 参数:
//   line - 单行文本（如 "OS Name: Microsoft Windows Server 2019"）
// 返回值:
//   string - 值部分（如 "Microsoft Windows Server 2019"）
// 解析逻辑:
//   1. 去除首尾空白
//   2. 如果行为空，返回空字符串
//   3. 使用":"分割，最多分割2部分
//   4. 如果分割成功，返回第二部分（去除首尾空白）
//   5. 否则返回空字符串
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
// 使用ver、echo PROCESSOR_ARCHITECTURE和hostname命令获取信息，比wmic更快速
// 参数:
//   server - 目标服务器信息，包含主机地址、端口、用户名、密码等
// 返回值:
//   string - 内核版本（如 "10.0.19045"）
//   string - 系统架构（如 "x86_64"）
//   string - 主机名（服务器的主机名称）
//   string - 空字符串（保留兼容，暂无使用）
//   error - 检测过程中的错误，连接失败或命令执行失败时返回
// 执行流程:
//   1. 创建SMB客户端并建立连接
//   2. 执行命令获取系统信息：
//      - ver（获取Windows版本）
//      - echo PROCESSOR_ARCHITECTURE=%PROCESSOR_ARCHITECTURE%（获取架构）
//      - hostname（获取主机名）
//   3. 解析命令输出，提取内核版本、架构和主机名
//   4. 将架构代码映射为标准名称
//   5. 返回检测结果
// 注意点:
//   - 使用defer client.Close()确保SMB连接在函数结束时关闭
//   - 此函数是简化版，比DetectWindowsOSAndInfo更快速但信息较少
//   - 不获取操作系统名称，只获取内核版本、架构和主机名
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