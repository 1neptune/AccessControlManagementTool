package utils

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// ServiceType 服务类型枚举
// 用于标识端口上运行的服务类型
type ServiceType string

// 服务类型常量定义
const (
	ServiceUnknown ServiceType = "unknown" // 未知服务类型
	ServiceSSH     ServiceType = "ssh"     // SSH服务（Linux远程管理）
	ServiceSMB     ServiceType = "smb"     // SMB服务（Windows文件共享）
	ServiceDCOM    ServiceType = "dcom"    // DCOM服务（Windows分布式组件对象模型）
)

// TestConnectivity 测试主机端口的TCP连通性
// 参数:
//   host - 主机地址（IP或域名）
//   port - 端口号（1-65535）
// 返回值:
//   bool - 连通成功返回true，否则返回false
// 功能:
//   - 使用net.DialTimeout建立TCP连接，超时时间5秒
//   - 连接成功后立即关闭连接，避免资源泄漏
//   - 端口范围验证：必须在1-65535之间
func TestConnectivity(host string, port int) bool {
	if host == "" || port <= 0 || port > 65535 {
		return false
	}

	address := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		log.Printf("连通性测试失败: %s:%d - %v", host, port, err)
		return false
	}
	defer conn.Close()

	log.Printf("连通性测试成功: %s:%d", host, port)
	return true
}

// TestSSHService 检测指定端口是否运行SSH服务
// 参数:
//   host - 主机地址（IP或域名）
//   port - 端口号
// 返回值:
//   bool - 检测到SSH服务返回true，否则返回false
// 原理:
//   - 建立TCP连接后读取服务端返回的第一行数据
//   - SSH服务会返回以"SSH-"开头的版本字符串（如 "SSH-2.0-OpenSSH_8.0"）
//   - 读取超时时间3秒，连接超时时间5秒
func TestSSHService(host string, port int) bool {
	address := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	return strings.HasPrefix(strings.TrimSpace(line), "SSH-")
}

// TestSMBService 检测指定端口是否运行SMB服务
// 参数:
//   host - 主机地址（IP或域名）
//   port - 端口号
// 返回值:
//   bool - 检测到SMB服务返回true，否则返回false
// 原理:
//   - 默认端口445直接判定为SMB服务（常见Windows文件共享端口）
//   - 其他端口通过发送SMB协议头（0xFF 0x53 0x4D 0x42）并检查响应来识别
//   - SMB协议头为: 0xFF 'S' 'M' 'B'
func TestSMBService(host string, port int) bool {
	if port == 445 {
		return true
	}

	address := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	smbHeader := []byte{0xFF, 0x53, 0x4D, 0x42}

	_, err = conn.Write(smbHeader)
	if err != nil {
		return false
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil || n < 4 {
		return false
	}

	if buf[0] == 0xFF && buf[1] == 0x53 && buf[2] == 0x4D && buf[3] == 0x42 {
		return true
	}

	return false
}

// TestDCOMService 检测指定端口是否运行DCOM服务
// 参数:
//   host - 主机地址（IP或域名）
//   port - 端口号
// 返回值:
//   bool - 检测到DCOM服务返回true，否则返回false
// 原理:
//   - DCOM服务默认使用端口135（RPC端点映射器端口）
//   - 通过端口号直接识别，不进行协议握手
func TestDCOMService(host string, port int) bool {
	if port == 135 {
		return true
	}
	return false
}

// DetectServiceType 检测指定主机端口上运行的服务类型
// 参数:
//   host - 主机地址（IP或域名）
//   port - 端口号
// 返回值:
//   ServiceType - 服务类型枚举值
// 检测顺序:
//   1. 先检测SSH服务（优先Linux服务器）
//   2. 再检测SMB服务（Windows文件共享）
//   3. 最后检测DCOM服务（Windows RPC）
//   4. 都不匹配则返回ServiceUnknown
func DetectServiceType(host string, port int) ServiceType {
	if TestSSHService(host, port) {
		return ServiceSSH
	}
	if TestSMBService(host, port) {
		return ServiceSMB
	}
	if TestDCOMService(host, port) {
		return ServiceDCOM
	}
	return ServiceUnknown
}