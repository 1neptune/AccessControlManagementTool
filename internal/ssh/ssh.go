package ssh

import (
	"access-control-tool/internal/models"
	"access-control-tool/internal/utils"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Client SSH客户端结构体，封装了golang.org/x/crypto/ssh的底层Client
// 提供远程命令执行能力，用于与Linux服务器进行交互
// 字段说明:
//   client - 底层SSH客户端连接实例，通过ssh.Dial创建
type Client struct {
	client *ssh.Client
}

// NewClient 创建SSH客户端实例并建立连接
// 根据服务器信息创建SSH连接，支持多种认证方式
// 参数:
//   server - 服务器信息，包含主机地址、端口、用户名、密码(加密)、密钥路径等
// 返回值:
//   *Client - SSH客户端实例，连接成功后可用
//   error - 创建过程中的错误，连接失败时返回
// 认证优先级（按顺序尝试）:
//   1. SSH agent认证：通过SSH_AUTH_SOCK环境变量检测，适用于已配置SSH agent的环境
//   2. 密码认证：使用服务器的密码（需要先解密），最常用的认证方式
//   3. 私钥认证：通过KeyPath字段指定的私钥文件进行认证
// 连接配置:
//   - 超时时间: 30秒，适用于网络延迟较大的环境
//   - 主机密钥验证: 使用InsecureIgnoreHostKey()，跳过主机密钥验证
//   - 用户名: 使用server.Username
// 注意点:
//   - 密码字段在数据库中是加密存储的，需要先调用utils.Decrypt解密
//   - 私钥文件路径必须是本地文件系统中的有效路径
//   - 连接成功后应在使用完毕后调用Close()释放资源
//   - InsecureIgnoreHostKey()在生产环境中可能存在安全风险，应考虑使用已知主机密钥
func NewClient(server *models.Server) (*Client, error) {
	log.Printf("[SSH] 开始创建SSH客户端，服务器: %s:%d, 用户: %s", server.Host, server.SSHPort, server.Username)

	var authMethods []ssh.AuthMethod

	if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
		log.Printf("[SSH] 检测到SSH_AUTH_SOCK环境变量，尝试使用SSH agent")
		if conn, err := net.Dial("unix", sshAuthSock); err == nil {
			defer conn.Close()
			agentClient := agent.NewClient(conn)
			authMethods = append(authMethods, ssh.PublicKeysCallback(agentClient.Signers))
			log.Printf("[SSH] SSH agent认证方法已添加")
		} else {
			log.Printf("[SSH] SSH agent连接失败: %v", err)
		}
	}

	log.Printf("[SSH] 开始解密密码")
	password, err := utils.Decrypt(server.Password)
	if err != nil {
		log.Printf("[SSH] 密码解密失败: %v", err)
		return nil, fmt.Errorf("密码解密失败: %v", err)
	}
	authMethods = append(authMethods, ssh.Password(password))
	log.Printf("[SSH] 密码认证方法已添加")

	if server.KeyPath != "" {
		log.Printf("[SSH] 检测到密钥路径: %s，尝试加载私钥", server.KeyPath)
		key, err := loadPrivateKey(server.KeyPath)
		if err == nil {
			authMethods = append(authMethods, ssh.PublicKeys(key))
			log.Printf("[SSH] 密钥认证方法已添加")
		} else {
			log.Printf("[SSH] 私钥加载失败: %v", err)
		}
	}

	log.Printf("[SSH] SSH客户端配置 - 用户: %s, 超时: %ds, 认证方法数: %d", server.Username, 30, len(authMethods))
	config := &ssh.ClientConfig{
		User:            server.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", server.Host, server.SSHPort)
	log.Printf("[SSH] 开始建立连接: %s", addr)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		log.Printf("[SSH] SSH连接失败(%s): %v", addr, err)
		return nil, fmt.Errorf("SSH连接失败(%s): %v", addr, err)
	}

	log.Printf("[SSH] SSH连接成功: %s", addr)
	return &Client{client: client}, nil
}

// loadPrivateKey 加载私钥文件并解析为ssh.Signer
// 支持多种常见的私钥格式
// 参数:
//   path - 私钥文件路径，必须是本地文件系统中的有效路径
// 返回值:
//   ssh.Signer - 私钥签名器，用于SSH认证
//   error - 加载过程中的错误，文件不存在或格式错误时返回
// 支持的私钥格式:
//   - RSA: 使用ssh.ParsePrivateKey自动识别
//   - DSA: 使用ssh.ParsePrivateKey自动识别
//   - ECDSA: 使用ssh.ParsePrivateKey自动识别
//   - Ed25519: 使用ssh.ParsePrivateKey自动识别
// 注意点:
//   - 私钥文件必须有正确的读取权限（通常为600）
//   - 支持加密的私钥文件，需要在解析时提供密码
//   - 当前实现不支持加密的私钥（需要密码才能解析）
func loadPrivateKey(path string) (ssh.Signer, error) {
	log.Printf("[SSH] 读取私钥文件: %s", path)
	keyData, err := ioutil.ReadFile(path)
	if err != nil {
		log.Printf("[SSH] 私钥文件读取失败: %v", err)
		return nil, err
	}

	log.Printf("[SSH] 私钥文件读取成功，长度: %d 字节", len(keyData))
	key, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		log.Printf("[SSH] 私钥解析失败: %v", err)
		return nil, err
	}

	log.Printf("[SSH] 私钥解析成功")
	return key, nil
}

// Execute 在远程服务器上执行命令
// 通过SSH会话执行命令并获取输出
// 参数:
//   command - 要执行的命令字符串，如"ls -l"或"cat /etc/os-release"
// 返回值:
//   string - 命令执行输出（标准输出+标准错误合并）
//   error - 执行过程中的错误，命令返回非零退出码时也会返回error
// 实现细节:
//   1. 创建新的SSH会话（session）
//   2. 使用CombinedOutput方法执行命令，同时捕获stdout和stderr
//   3. 使用defer session.Close()确保会话在函数结束时关闭
//   4. 命令执行失败时，返回错误信息但仍然返回命令输出（便于调试）
// 注意点:
//   - 每次执行命令都会创建新的会话，执行完毕后自动关闭
//   - CombinedOutput会阻塞直到命令执行完成
//   - 对于长时间运行的命令，应考虑使用Session.Start配合StdoutPipe
//   - 命令输出可能包含敏感信息，需注意日志记录的安全性
func (c *Client) Execute(command string) (string, error) {
	log.Printf("[SSH] 开始创建SSH会话")
	session, err := c.client.NewSession()
	if err != nil {
		log.Printf("[SSH] 创建会话失败: %v", err)
		return "", fmt.Errorf("创建会话失败: %v", err)
	}
	defer session.Close()

	log.Printf("[SSH] 会话创建成功，执行命令: %s", command)
	output, err := session.CombinedOutput(command)
	if err != nil {
		log.Printf("[SSH] 命令执行失败，输出: %s, 错误: %v", string(output), err)
		return string(output), fmt.Errorf("命令执行失败: %v", err)
	}

	log.Printf("[SSH] 命令执行成功，输出长度: %d 字节", len(output))
	return string(output), nil
}

// Close 关闭SSH连接，释放底层资源
// 应在使用完SSH客户端后调用此方法，避免连接泄漏
// 安全说明:
//   - 使用前检查client是否为nil，避免空指针引用
//   - 关闭后底层TCP连接释放，客户端实例不再可用
//   - 调用后不应再使用该客户端实例执行任何操作
func (c *Client) Close() {
	if c.client != nil {
		log.Printf("[SSH] 关闭SSH连接")
		c.client.Close()
		log.Printf("[SSH] SSH连接已关闭")
	}
}
