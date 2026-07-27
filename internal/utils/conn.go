package utils

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// ServiceType 服务类型枚举，用于标识端口上运行的服务类型
// 该枚举用于服务器添加/编辑时自动检测操作系统类型，以及服务类型识别
type ServiceType string

// 服务类型常量定义
const (
	ServiceUnknown ServiceType = "unknown" // 未知服务类型，无法识别端口上运行的服务
	ServiceSSH     ServiceType = "ssh"     // SSH服务，通常运行在Linux服务器的22端口
	ServiceSMB     ServiceType = "smb"     // SMB服务，通常运行在Windows服务器的445端口
)

// TestConnectivity 测试主机端口的TCP连通性
// 判断指定主机的指定端口是否可达，用于验证网络连接是否正常
// 参数:
//   host - 主机地址（IP地址或域名）
//   port - 端口号，必须在1-65535之间
// 返回值:
//   bool - 连通成功返回true，否则返回false
// 实现逻辑:
//   1. 参数校验：检查host是否为空，port是否在有效范围内（1-65535）
//   2. 构造地址字符串：格式为"host:port"
//   3. 使用net.DialTimeout建立TCP连接，超时时间5秒
//   4. 连接成功后立即关闭连接，避免资源泄漏
//   5. 记录调试日志（成功或失败）
// 注意点:
//   - 此函数只测试TCP连通性，不验证服务类型
//   - 超时时间固定为5秒，适用于大多数网络环境
//   - 使用defer conn.Close()确保连接一定会被关闭
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
// 通过读取服务端返回的协议标识来判断是否为SSH服务
// 参数:
//   host - 主机地址（IP地址或域名）
//   port - 端口号
// 返回值:
//   bool - 检测到SSH服务返回true，否则返回false
// 实现原理:
//   1. 建立TCP连接，超时时间5秒
//   2. 设置读取超时时间为3秒，避免长时间阻塞
//   3. 读取服务端发送的第一行数据
//   4. 判断是否以"SSH-"开头（SSH协议标准版本标识）
//   5. SSH服务会返回类似"SSH-2.0-OpenSSH_8.0"的字符串
// 注意点:
//   - 使用defer conn.Close()确保连接释放
//   - 读取超时时间3秒，比连接超时时间短，避免不必要的等待
//   - 仅通过协议标识判断，不进行完整的SSH握手
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
// 通过协议特征判断是否为SMB服务，用于识别Windows服务器
// 参数:
//   host - 主机地址（IP地址或域名）
//   port - 端口号
// 返回值:
//   bool - 检测到SMB服务返回true，否则返回false
// 实现原理:
//   1. 特殊处理端口445：直接判定为SMB服务（Windows默认文件共享端口）
//   2. 对于其他端口：
//      - 建立TCP连接，超时时间5秒
//      - 设置读取超时时间为3秒
//      - 发送SMB协议头（0xFF 0x53 0x4D 0x42，即"ÿSMB"）
//      - 读取响应并检查前4字节是否为SMB协议头
//   3. SMB协议头为固定格式：0xFF 'S' 'M' 'B'
// 注意点:
//   - 端口445是常见的SMB端口，直接返回true以提高检测速度
//   - 发送协议头后等待响应，验证服务是否正确响应SMB协议
//   - 使用defer conn.Close()确保连接释放
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

// DetectServiceType 检测指定主机端口上运行的服务类型
// 综合调用SSH和SMB检测函数，确定服务类型
// 参数:
//   host - 主机地址（IP地址或域名）
//   port - 端口号
// 返回值:
//   ServiceType - 服务类型枚举值（ServiceSSH、ServiceSMB、ServiceUnknown）
// 检测顺序:
//   1. 先检测SSH服务（优先判断Linux服务器）
//   2. 再检测SMB服务（判断Windows服务器）
//   3. 都不匹配则返回ServiceUnknown
// 注意点:
//   - 检测顺序决定了优先级：SSH优先于SMB
//   - 适用于添加服务器时自动识别操作系统类型
//   - 如果端口同时运行多种服务，返回第一个匹配的类型
func DetectServiceType(host string, port int) ServiceType {
	if TestSSHService(host, port) {
		return ServiceSSH
	}
	if TestSMBService(host, port) {
		return ServiceSMB
	}
	return ServiceUnknown
}