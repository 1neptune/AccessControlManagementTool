package smb

import (
	"access-control-tool/internal/models"
	"access-control-tool/internal/utils"
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/jfjallid/go-smb/smb"
	"github.com/jfjallid/go-smb/spnego"
	"github.com/oiweiwei/go-msrpc/dcerpc"
	"github.com/oiweiwei/go-msrpc/msrpc/dcom"
	"github.com/oiweiwei/go-msrpc/msrpc/dcom/iactivation/v0"
	"github.com/oiweiwei/go-msrpc/msrpc/dcom/iobjectexporter/v0"
	"github.com/oiweiwei/go-msrpc/msrpc/dcom/wmi"
	"github.com/oiweiwei/go-msrpc/msrpc/dcom/wmi/iwbemlevel1login/v0"
	"github.com/oiweiwei/go-msrpc/msrpc/dcom/wmi/iwbemservices/v0"
	"github.com/oiweiwei/go-msrpc/msrpc/dcom/wmio"
	"github.com/oiweiwei/go-msrpc/msrpc/dcom/wmio/query"
	"github.com/oiweiwei/go-msrpc/ssp"
	"github.com/oiweiwei/go-msrpc/ssp/credential"
	"github.com/oiweiwei/go-msrpc/ssp/gssapi"
	"golang.org/x/net/html/charset"
)

// Client 封装SMB连接和WMI执行能力的客户端
// 通过SMB协议建立连接，使用WMI远程执行命令
type Client struct {
	server     *models.Server        // 目标服务器信息
	password   string                // 解密后的密码
	smbConn    *smb.Connection       // SMB连接实例
	wmiConn    dcerpc.Conn           // WMI连接（DCERPC）
	services   iwbemservices.ServicesClient // WMI服务客户端
	comVersion *dcom.COMVersion      // COM版本信息
	wmiReady   bool                  // WMI是否已初始化就绪
}

var mechanismsInitOnce sync.Once

// initMechanisms 初始化GSSAPI安全机制
// 使用sync.Once确保全局只初始化一次，避免"mechanism already exist" panic
func initMechanisms() {
	mechanismsInitOnce.Do(func() {
		gssapi.AddMechanism(ssp.SPNEGO)
		gssapi.AddMechanism(ssp.NTLM)
	})
}

// NewClient 创建SMB客户端实例
// 步骤：解密密码 -> 建立SMB连接 -> 验证认证状态
// 参数: server - 目标服务器信息
// 返回: Client实例和错误
func NewClient(server *models.Server) (*Client, error) {
	password, err := utils.Decrypt(server.Password)
	if err != nil {
		return nil, fmt.Errorf("密码解密失败: %v", err)
	}

	log.Printf("[GoExec] 创建客户端，服务器: %s:%d", server.Host, server.SSHPort)

	smbOptions := smb.Options{
		Host: server.Host,
		Port: server.SSHPort,
		Initiator: &spnego.NTLMInitiator{
			User:      server.Username,
			Password:  password,
			Domain:    "",
			LocalUser: true,
		},
	}

	smbConn, err := smb.NewConnection(smbOptions)
	if err != nil {
		log.Printf("[GoExec] SMB连接失败: %v", err)
		return nil, fmt.Errorf("SMB连接失败(%s:%d): %v", server.Host, server.SSHPort, err)
	}

	if !smbConn.IsAuthenticated() {
		return nil, fmt.Errorf("SMB认证失败")
	}

	log.Printf("[GoExec] SMB连接成功")
	return &Client{server: server, password: password, smbConn: smbConn}, nil
}

// initWMI 初始化WMI连接（带重试机制）
// 最多重试3次，每次间隔递增2秒
// 如果wmiReady已为true，直接返回成功
func (c *Client) initWMI() error {
	if c.wmiReady {
		return nil
	}

	var lastErr error
	for retry := 0; retry < 3; retry++ {
		lastErr = c.doInitWMI()
		if lastErr == nil {
			c.wmiReady = true
			return nil
		}
		log.Printf("[GoExec] WMI初始化第%d次失败，等待重试: %v", retry+1, lastErr)
		time.Sleep(time.Duration(retry+1) * 2 * time.Second)
	}

	return lastErr
}

// doInitWMI 实际执行WMI初始化
// WMI连接流程:
// 1. 连接135端口（RPC端点映射器）
// 2. 创建ObjectExporter客户端，获取COM版本
// 3. 创建Activation客户端，远程激活WMI服务
// 4. 连接WMI端点
// 5. 登录WMI命名空间（root/cimv2）
// 6. 获取WMI服务客户端
func (c *Client) doInitWMI() error {
	initMechanisms()

	ctx := gssapi.NewSecurityContext(context.Background())

	log.Printf("[GoExec] 连接135端口")
	cc, err := dcerpc.Dial(ctx, net.JoinHostPort(c.server.Host, "135"),
		dcerpc.WithCredentials(credential.NewFromPassword(c.server.Username, c.password)),
		dcerpc.WithTargetName(c.server.Host))
	if err != nil {
		log.Printf("[GoExec] 连接135端口失败: %v", err)
		return fmt.Errorf("连接135端口失败: %v", err)
	}
	defer cc.Close(ctx)

	log.Printf("[GoExec] 135端口连接成功")

	cli, err := iobjectexporter.NewObjectExporterClient(ctx, cc, dcerpc.WithSign(), dcerpc.WithTargetName(c.server.Host))
	if err != nil {
		log.Printf("[GoExec] 创建ObjectExporter客户端失败: %v", err)
		return fmt.Errorf("创建ObjectExporter客户端失败: %v", err)
	}

	srv, err := cli.ServerAlive2(ctx, &iobjectexporter.ServerAlive2Request{})
	if err != nil {
		log.Printf("[GoExec] ServerAlive2失败: %v", err)
		return fmt.Errorf("ServerAlive2失败: %v", err)
	}

	log.Printf("[GoExec] ServerAlive2成功, COM版本: %d", srv.COMVersion)
	c.comVersion = srv.COMVersion

	iact, err := iactivation.NewActivationClient(ctx, cc, dcerpc.WithSign(), dcerpc.WithTargetName(c.server.Host))
	if err != nil {
		log.Printf("[GoExec] 创建Activation客户端失败: %v", err)
		return fmt.Errorf("创建Activation客户端失败: %v", err)
	}
	act, err := iact.RemoteActivation(ctx, &iactivation.RemoteActivationRequest{
		ORPCThis:                   &dcom.ORPCThis{Version: srv.COMVersion},
		ClassID:                    wmi.Level1LoginClassID.GUID(),
		IIDs:                       []*dcom.IID{iwbemlevel1login.Level1LoginIID},
		RequestedProtocolSequences: []uint16{7},
	})

	if err != nil {
		log.Printf("[GoExec] RemoteActivation失败: %v", err)
		return fmt.Errorf("RemoteActivation失败: %v", err)
	}

	if act.HResult != 0 {
		log.Printf("[GoExec] RemoteActivation返回错误码: %d", act.HResult)
		return fmt.Errorf("RemoteActivation返回错误码: %d", act.HResult)
	}

	log.Printf("[GoExec] RemoteActivation成功")

	std := act.InterfaceData[0].GetStandardObjectReference().Std

	var newOpts []dcerpc.Option
	for _, bind := range act.OXIDBindings.GetStringBindings() {
		stringBinding, err := dcerpc.ParseStringBinding(bind.String())
		if err != nil {
			continue
		}
		if stringBinding.ProtocolSequence == dcerpc.ProtocolSequenceIPTCP {
			stringBinding.NetworkAddress = c.server.Host
			newOpts = append(newOpts, dcerpc.WithEndpoint(stringBinding.String()))
		}
	}

	if len(newOpts) == 0 {
		newOpts = append(newOpts, dcerpc.WithEndpoint(fmt.Sprintf("ncacn_ip_tcp:%s", c.server.Host)))
	}

	log.Printf("[GoExec] 连接WMI端点")
	wcc, err := dcerpc.Dial(ctx, c.server.Host, append(newOpts,
		dcerpc.WithCredentials(credential.NewFromPassword(c.server.Username, c.password)),
		dcerpc.WithTargetName(c.server.Host))...)
	if err != nil {
		log.Printf("[GoExec] 连接WMI端点失败: %v", err)
		return fmt.Errorf("连接WMI端点失败: %v", err)
	}

	log.Printf("[GoExec] WMI端点连接成功")
	c.wmiConn = wcc

	l1login, err := iwbemlevel1login.NewLevel1LoginClient(ctx, wcc,
		dcom.WithIPID(std.IPID),
		dcerpc.WithSign(),
		dcerpc.WithTargetName(c.server.Host))
	if err != nil {
		log.Printf("[GoExec] 创建Level1Login客户端失败: %v", err)
		wcc.Close(ctx)
		return fmt.Errorf("创建Level1Login客户端失败: %v", err)
	}

	_, err = l1login.EstablishPosition(ctx, &iwbemlevel1login.EstablishPositionRequest{
		This: &dcom.ORPCThis{Version: srv.COMVersion},
	})
	if err != nil {
		log.Printf("[GoExec] EstablishPosition失败: %v", err)
		wcc.Close(ctx)
		return fmt.Errorf("EstablishPosition失败: %v", err)
	}

	log.Printf("[GoExec] EstablishPosition成功")

	login, err := l1login.NTLMLogin(ctx, &iwbemlevel1login.NTLMLoginRequest{
		This:            &dcom.ORPCThis{Version: srv.COMVersion},
		NetworkResource: "//./root/cimv2",
	})
	if err != nil {
		log.Printf("[GoExec] NTLMLogin失败: %v", err)
		wcc.Close(ctx)
		return fmt.Errorf("NTLMLogin失败: %v", err)
	}

	log.Printf("[GoExec] NTLMLogin成功")

	ns := login.Namespace

	svcs, err := iwbemservices.NewServicesClient(ctx, wcc,
		dcom.WithIPID(ns.InterfacePointer().IPID()),
		dcerpc.WithSign(),
		dcerpc.WithTargetName(c.server.Host))
	if err != nil {
		log.Printf("[GoExec] 创建Services客户端失败: %v", err)
		wcc.Close(ctx)
		return fmt.Errorf("创建Services客户端失败: %v", err)
	}

	c.services = svcs
	log.Printf("[GoExec] WMI初始化完成")
	return nil
}

// Execute 在远程Windows服务器上执行命令
// 执行流程:
// 1. 生成随机输出文件名
// 2. 构造带输出重定向的命令
// 3. 初始化WMI连接
// 4. 通过WMI执行命令
// 5. 等待命令执行完成（轮询读取输出文件）
// 6. 删除临时输出文件
// 参数: command - 要执行的命令
// 返回: 命令输出和错误
func (c *Client) Execute(command string) (string, error) {
	log.Printf("[GoExec] 准备执行命令: %s", command)

	outputFile := generateOutputFileName()
	outputPath := fmt.Sprintf("Temp\\%s", outputFile)
	fullOutputPath := fmt.Sprintf("C:\\Windows\\Temp\\%s", outputFile)

	log.Printf("[GoExec] 输出文件: %s", fullOutputPath)

	cmdWithOutput := fmt.Sprintf(`cmd.exe /c "(%s) > %s 2>&1"`, command, fullOutputPath)
	log.Printf("[GoExec] 带输出重定向的命令: %s", cmdWithOutput)

	if err := c.initWMI(); err != nil {
		log.Printf("[GoExec] WMI初始化失败: %v", err)
		return "", fmt.Errorf("WMI初始化失败: %v", err)
	}

	err := c.executeViaWMI(cmdWithOutput)
	if err != nil {
		log.Printf("[GoExec] WMI执行命令失败: %v", err)
		return "", fmt.Errorf("WMI执行命令失败: %v", err)
	}

	time.Sleep(1500 * time.Millisecond)

	for i := 0; i < 8; i++ {
		output, err := c.readRemoteFile(outputPath)
		if err == nil && len(output) > 0 {
			log.Printf("[GoExec] 命令执行完成, 输出长度: %d", len(output))
			if len(output) > 2000 {
				log.Printf("[GoExec] 输出内容(前2000字节): %s", output[:2000])
			} else {
				log.Printf("[GoExec] 输出内容: %s", output)
			}
			c.deleteRemoteFile(outputPath)
			return output, nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("[GoExec] 输出文件读取超时")
	c.deleteRemoteFile(outputPath)
	return "", fmt.Errorf("命令执行超时，未获取到输出")
}

// executeViaWMI 通过WMI调用Win32_Process.Create执行命令
// 使用60秒超时上下文处理网络延迟或慢服务器响应
// 参数: command - 要执行的命令（已包含输出重定向）
func (c *Client) executeViaWMI(command string) error {
	log.Printf("[GoExec] 开始通过WMI执行命令")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ctx = gssapi.NewSecurityContext(ctx)

	builder := query.NewBuilder(ctx, c.services, c.comVersion)

	args := wmio.Values{
		"CommandLine": command,
		"WorkingDir":  "C:\\",
	}

	out, err := builder.Spawn("Win32_Process").Method("Create").Values(args).Exec().Object()
	if err != nil {
		log.Printf("[GoExec] Win32_Process.Create执行失败: %v", err)
		return fmt.Errorf("Win32_Process.Create执行失败: %v", err)
	}

	values := out.Values()
	if pid, ok := values["ProcessId"].(uint32); pid != 0 {
		log.Printf("[GoExec] 进程创建成功, PID: %d", pid)
	} else if !ok {
		return fmt.Errorf("进程创建失败")
	}

	if ret, ok := values["ReturnValue"].(uint32); ret != 0 {
		log.Printf("[GoExec] 进程返回非零退出码: %d", ret)
	} else if !ok {
		return fmt.Errorf("无效的调用响应")
	}

	log.Printf("[GoExec] Win32_Process.Create执行成功")
	return nil
}

// readRemoteFile 通过SMB读取远程服务器上的文件
// 连接ADMIN$共享，读取文件内容，并自动检测编码进行转换
// 支持的编码: UTF-16LE（带BOM或特征检测）、GBK
// 参数: path - 文件路径（相对于ADMIN$，如 "Temp\\xxx.txt"）
// 返回: 文件内容和错误
func (c *Client) readRemoteFile(path string) (string, error) {
	log.Printf("[GoExec] 通过SMB读取文件: %s", path)

	err := c.smbConn.TreeConnect("ADMIN$")
	if err != nil {
		log.Printf("[GoExec] 连接ADMIN$失败: %v", err)
		return "", fmt.Errorf("连接ADMIN$失败: %v", err)
	}
	defer func() {
		if err := c.smbConn.TreeDisconnect("ADMIN$"); err != nil {
			log.Printf("[GoExec] 断开ADMIN$失败: %v", err)
		}
	}()

	file, err := c.smbConn.OpenFile("ADMIN$", path)
	if err != nil {
		log.Printf("[GoExec] 打开文件失败: %v", err)
		return "", fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.CloseFile()

	buf := make([]byte, 65536)
	n, err := file.ReadFile(buf, 0)
	if err != nil && err != io.EOF {
		log.Printf("[GoExec] 读取文件失败: %v", err)
		return "", fmt.Errorf("读取文件失败: %v", err)
	}

	if n == 0 {
		log.Printf("[GoExec] 文件为空")
		return "", nil
	}

	log.Printf("[GoExec] 原始字节数据(前64字节): %x", buf[:min(n, 64)])

	var output string

	if len(buf[:n]) >= 2 && buf[0] == 0xFF && buf[1] == 0xFE {
		log.Printf("[GoExec] 检测到UTF-16LE BOM, 使用UTF-16LE转换")
		output, err = utf16leToUtf8(buf[:n])
	} else if looksLikeUtf16le(buf[:n]) {
		log.Printf("[GoExec] 数据特征符合UTF-16LE, 使用UTF-16LE转换")
		output, err = utf16leToUtf8(buf[:n])
	} else {
		log.Printf("[GoExec] 使用GBK转换")
		output, err = gbkToUtf8(buf[:n])
	}

	if err != nil {
		log.Printf("[GoExec] 编码转换失败: %v", err)
		output = string(buf[:n])
	}

	log.Printf("[GoExec] 文件读取成功, 长度: %d", len(output))
	return output, nil
}

// deleteRemoteFile 通过SMB删除远程服务器上的文件
// 最多重试3次，每次间隔递增1秒
// 参数: path - 文件路径（相对于ADMIN$）
func (c *Client) deleteRemoteFile(path string) error {
	log.Printf("[GoExec] 通过SMB删除文件: %s", path)

	for retry := 0; retry < 3; retry++ {
		err := c.smbConn.TreeConnect("ADMIN$")
		if err != nil {
			log.Printf("[GoExec] 连接ADMIN$失败: %v", err)
			time.Sleep(time.Duration(retry+1) * time.Second)
			continue
		}

		err = c.smbConn.DeleteFile("ADMIN$", path)
		if disconnectErr := c.smbConn.TreeDisconnect("ADMIN$"); disconnectErr != nil {
			log.Printf("[GoExec] 断开ADMIN$失败: %v", disconnectErr)
		}

		if err != nil {
			log.Printf("[GoExec] 删除文件失败(第%d次): %v", retry+1, err)
			time.Sleep(time.Duration(retry+1) * time.Second)
			continue
		}

		log.Printf("[GoExec] 文件删除成功")
		return nil
	}

	log.Printf("[GoExec] 文件删除多次失败，跳过")
	return nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// looksLikeUtf16le 判断字节数据是否符合UTF-16LE编码特征
// 通过检测高零字节比例（>60%）和有效ASCII字符比例（>70%）来判断
// 参数: data - 待检测的字节数据
// 返回: 是否符合UTF-16LE特征
func looksLikeUtf16le(data []byte) bool {
	if len(data) < 4 {
		return false
	}

	nullCount := 0
	validUtf16Pairs := 0
	invalidPairs := 0

	for i := 0; i < len(data); i += 2 {
		if i+1 >= len(data) {
			break
		}

		if data[i] == 0x00 && data[i+1] == 0x00 {
			continue
		}

		if data[i+1] == 0x00 {
			nullCount++
			if data[i] >= 0x20 && data[i] <= 0x7E {
				validUtf16Pairs++
			} else {
				invalidPairs++
			}
		}
	}

	totalPairs := validUtf16Pairs + invalidPairs
	if totalPairs == 0 {
		return false
	}

	nullRatio := float64(nullCount) / float64(totalPairs)
	validRatio := float64(validUtf16Pairs) / float64(totalPairs)

	return nullRatio > 0.6 && validRatio > 0.7
}

// utf16leToUtf8 将UTF-16LE编码的字节数据转换为UTF-8字符串
// 自动处理BOM头（FF FE）
// 参数: data - UTF-16LE编码的字节数据
// 返回: UTF-8字符串和错误
func utf16leToUtf8(data []byte) (string, error) {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}

	if len(data) == 0 {
		return "", nil
	}

	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		data = data[2:]
	}

	u16 := make([]uint16, 0, len(data)/2)
	for i := 0; i < len(data); i += 2 {
		u16 = append(u16, uint16(data[i])|uint16(data[i+1])<<8)
	}

	return string(utf16.Decode(u16)), nil
}

// gbkToUtf8 将GBK编码的字节数据转换为UTF-8字符串
// 使用golang.org/x/net/html/charset包进行转换
// 参数: data - GBK编码的字节数据
// 返回: UTF-8字符串和错误
func gbkToUtf8(data []byte) (string, error) {
	reader, err := charset.NewReaderLabel("gbk", strings.NewReader(string(data)))
	if err != nil {
		return "", err
	}
	result, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

// Close 关闭客户端连接
// 依次关闭WMI连接和SMB连接，重置状态标志
func (c *Client) Close() {
	if c.wmiConn != nil {
		c.wmiConn.Close(gssapi.NewSecurityContext(context.Background()))
		c.wmiConn = nil
		c.services = nil
		c.comVersion = nil
		c.wmiReady = false
		log.Printf("[GoExec] WMI连接已关闭")
	}
	if c.smbConn != nil {
		c.smbConn.Close()
		log.Printf("[GoExec] SMB连接已关闭")
	}
}

// generateOutputFileName 生成随机的输出文件名
// 格式: GOEXEC + 8位随机字母 + .txt
func generateOutputFileName() string {
	rand.Seed(time.Now().UnixNano())
	letters := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 8)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return "GOEXEC" + string(b) + ".txt"
}