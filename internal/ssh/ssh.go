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

// Client SSH客户端结构体
// 封装了golang.org/x/crypto/ssh的Client，提供远程命令执行能力
// 字段说明:
//   client - 底层SSH客户端连接
type Client struct {
	client *ssh.Client
}

// NewClient 创建SSH客户端实例
// 参数:
//   server - 服务器信息，包含主机地址、端口、用户名、密码(加密)、密钥路径等
// 返回值:
//   *Client - SSH客户端实例
//   error - 创建过程中的错误
// 认证优先级:
//   1. SSH agent认证（通过SSH_AUTH_SOCK环境变量）
//   2. 密码认证（密码需要先解密）
//   3. 私钥认证（通过KeyPath加载）
// 连接配置:
//   - 超时时间: 30秒
//   - 忽略主机密钥验证（InsecureIgnoreHostKey）
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

// loadPrivateKey 加载私钥文件
// 参数:
//   path - 私钥文件路径
// 返回值:
//   ssh.Signer - 私钥签名器
//   error - 加载过程中的错误
// 支持的私钥格式:
//   - RSA
//   - DSA
//   - ECDSA
//   - Ed25519
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
// 参数:
//   command - 要执行的命令字符串
// 返回值:
//   string - 命令执行输出（标准输出+标准错误）
//   error - 执行过程中的错误
// 实现细节:
//   - 创建新的SSH会话执行命令
//   - 使用CombinedOutput同时捕获stdout和stderr
//   - 命令执行后自动关闭会话
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

// Close 关闭SSH连接
// 安全说明:
//   - 使用前检查client是否为nil，避免空指针引用
//   - 关闭后底层连接释放，客户端不再可用
func (c *Client) Close() {
	if c.client != nil {
		log.Printf("[SSH] 关闭SSH连接")
		c.client.Close()
		log.Printf("[SSH] SSH连接已关闭")
	}
}
