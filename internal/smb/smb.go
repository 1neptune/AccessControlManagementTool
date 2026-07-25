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

// Client SMB客户端结构体，封装了SMB连接和WMI远程执行能力
// 通过SMB协议建立与Windows服务器的连接，使用WMI（Windows Management Instrumentation）
// 远程执行命令，实现对Windows服务器的管理操作
// 字段说明:
//   server - 目标服务器信息，包含主机地址、端口、用户名等
//   password - 解密后的密码，用于SMB和WMI认证
//   smbConn - SMB连接实例，用于文件读写和共享访问
//   wmiConn - WMI连接（基于DCERPC），用于执行远程命令
//   services - WMI服务客户端，提供WMI命名空间操作能力
//   comVersion - COM版本信息，用于WMI协议交互
//   wmiReady - WMI是否已初始化就绪的标志，避免重复初始化
type Client struct {
	server     *models.Server                     // 目标服务器信息
	password   string                             // 解密后的密码
	smbConn    *smb.Connection                    // SMB连接实例
	wmiConn    dcerpc.Conn                        // WMI连接（DCERPC）
	services   iwbemservices.ServicesClient       // WMI服务客户端
	comVersion *dcom.COMVersion                   // COM版本信息
	wmiReady   bool                               // WMI是否已初始化就绪
}

var mechanismsInitOnce sync.Once

// initMechanisms 初始化GSSAPI安全机制
// 使用sync.Once确保全局只初始化一次，避免重复添加导致的"mechanism already exist" panic
// 注册的安全机制:
//   - SSP.SPNEGO: 简单和受保护的GSSAPI协商机制
//   - SSP.NTLM: Windows NTLM认证协议
// 注意点:
//   - 必须在建立WMI连接前调用
//   - 使用sync.Once保证线程安全和只执行一次
func initMechanisms() {
	mechanismsInitOnce.Do(func() {
		gssapi.AddMechanism(ssp.SPNEGO)
		gssapi.AddMechanism(ssp.NTLM)
	})
}

// NewClient 创建SMB客户端实例并建立连接
// 执行步骤：解密密码 -> 建立SMB连接 -> 验证认证状态
// 参数:
//   server - 目标服务器信息，包含主机地址、端口、用户名、加密密码等
// 返回值:
//   *Client - SMB客户端实例，连接成功后可用
//   error - 创建过程中的错误，连接失败时返回
// 连接配置:
//   - 使用NTLM认证（LocalUser: true表示本地用户）
//   - 默认端口445（Windows SMB服务端口）
//   - 使用spnego.NTLMInitiator进行认证
// 注意点:
//   - 密码字段在数据库中是加密存储的，需要先调用utils.Decrypt解密
//   - 连接成功后应在使用完毕后调用Close()释放资源
//   - 需要确保目标服务器的SMB服务（445端口）可用
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
// WMI初始化是一个复杂的过程，可能因网络问题失败，因此提供重试机制
// 参数: 无
// 返回值:
//   error - 初始化过程中的错误，成功返回nil
// 重试策略:
//   - 最多重试3次
//   - 每次重试间隔递增2秒（第1次2秒，第2次4秒，第3次6秒）
//   - 如果wmiReady已为true，直接返回成功，避免重复初始化
// 注意点:
//   - WMI初始化需要连接135端口（RPC端点映射器）
//   - 如果连续3次失败，返回最后一次的错误
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

// doInitWMI 实际执行WMI初始化，建立完整的WMI连接链路
// WMI连接是一个多步骤的复杂过程，涉及多个协议层
// 参数: 无
// 返回值:
//   error - 初始化过程中的错误，成功返回nil
// WMI连接流程:
//   1. 初始化GSSAPI安全机制（通过initMechanisms）
//   2. 连接135端口（RPC端点映射器，用于查找WMI服务端点）
//   3. 创建ObjectExporter客户端，获取COM版本信息
//   4. 创建Activation客户端，远程激活WMI服务（Level1Login）
//   5. 从激活结果中提取WMI端点信息
//   6. 连接WMI端点（基于提取的端点信息）
//   7. 创建Level1Login客户端，建立位置
//   8. 执行NTLMLogin，登录到root/cimv2命名空间
//   9. 获取WMI服务客户端（ServicesClient），完成初始化
// 注意点:
//   - 每个步骤都可能失败，需要正确处理错误并清理资源
//   - 135端口必须可达，否则无法完成WMI初始化
//   - 使用defer cc.Close(ctx)确保RPC连接在函数结束时关闭
//   - WMI连接成功后会设置c.wmiReady = true
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
// 通过WMI调用Win32_Process.Create创建进程执行命令，并通过SMB读取输出
// 参数:
//   command - 要执行的命令字符串，如"ipconfig"或"netstat -ano"
// 返回值:
//   string - 命令执行输出（标准输出+标准错误）
//   error - 执行过程中的错误，超时或失败时返回
// 执行流程:
//   1. 生成随机输出文件名（格式: GOEXEC + 8位随机字母 + .txt）
//   2. 构造带输出重定向的命令，将输出写入C:\Windows\Temp目录
//   3. 初始化WMI连接（如果尚未初始化）
//   4. 通过WMI执行命令（调用Win32_Process.Create）
//   5. 等待1.5秒让命令开始执行
//   6. 轮询读取输出文件（最多8次，每次间隔0.5秒）
//   7. 读取成功后删除临时输出文件
//   8. 如果超时未读取到输出，删除临时文件并返回错误
// 注意点:
//   - 命令输出通过文件重定向获取，因此命令必须能写入文件
//   - 需要目标服务器的ADMIN$共享可访问（用于读取输出文件）
//   - 超时时间约5.5秒（1.5秒初始等待 + 8*0.5秒轮询）
//   - 临时文件会在命令执行完成后或超时后自动删除
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
// 这是执行Windows命令的核心方法，使用WMI的进程创建功能
// 参数:
//   command - 要执行的命令（已包含输出重定向，如cmd.exe /c "ipconfig > output.txt"）
// 返回值:
//   error - 执行过程中的错误，成功返回nil
// 实现细节:
//   1. 创建60秒超时的上下文，处理网络延迟或慢服务器响应
//   2. 使用gssapi安全上下文进行认证
//   3. 构建WMI查询，调用Win32_Process.Create方法
//   4. 验证进程创建结果（检查ProcessId和ReturnValue）
//   5. 返回进程创建结果，不等待命令执行完成
// 注意点:
//   - 超时时间60秒，适用于网络延迟较大的环境
//   - 只负责创建进程，不负责等待命令完成
//   - 返回值为0表示成功创建进程，但不代表命令执行成功
//   - 命令执行结果需要通过读取输出文件获取
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
// 参数:
//   path - 文件路径（相对于ADMIN$共享，如 "Temp\\output.txt"）
// 返回值:
//   string - 文件内容（已转换为UTF-8编码）
//   error - 读取过程中的错误，文件不存在或读取失败时返回
// 编码检测与转换:
//   1. 检测UTF-16LE BOM（FF FE），如果存在则使用UTF-16LE转换
//   2. 如果没有BOM但数据特征符合UTF-16LE，使用UTF-16LE转换
//   3. 否则使用GBK转换（Windows默认编码）
//   4. 如果转换失败，直接返回原始字节的字符串表示
// 注意点:
//   - 需要目标服务器的ADMIN$共享可访问（通常需要管理员权限）
//   - 文件读取后会自动断开ADMIN$共享连接
//   - 使用64KB缓冲区读取文件，适合大多数命令输出
//   - 自动处理BOM和编码问题，确保输出为UTF-8
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
// 删除命令执行产生的临时输出文件，保持服务器清洁
// 参数:
//   path - 文件路径（相对于ADMIN$共享）
// 返回值:
//   error - 删除过程中的错误，成功返回nil
// 重试策略:
//   - 最多重试3次
//   - 每次重试间隔递增1秒（第1次1秒，第2次2秒，第3次3秒）
//   - 如果3次都失败，记录日志并返回nil（跳过删除，不影响主流程）
// 注意点:
//   - 需要目标服务器的ADMIN$共享可访问
//   - 删除前需要先连接ADMIN$共享，删除后断开连接
//   - 删除失败不会中断主流程，只记录警告日志
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
// 辅助函数，用于限制数组切片的长度
// 参数:
//   a, b - 两个整数
// 返回值:
//   int - 较小的整数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// looksLikeUtf16le 判断字节数据是否符合UTF-16LE编码特征
// UTF-16LE是Windows常用的Unicode编码，每个字符占用2字节，低字节在前
// 参数:
//   data - 待检测的字节数据
// 返回值:
//   bool - 是否符合UTF-16LE特征
// 判断逻辑:
//   1. 数据长度至少4字节
//   2. 检测高零字节比例（>60%）：UTF-16LE中ASCII字符的高字节为0
//   3. 检测有效ASCII字符比例（>70%）：低字节在0x20-0x7E范围内
//   4. 同时满足两个条件则判定为UTF-16LE
// 注意点:
//   - 这是启发式检测，不是精确的编码检测
//   - 主要用于区分UTF-16LE和GBK编码
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
// UTF-16LE是Windows默认的Unicode编码，需要转换为UTF-8以便在Go中处理
// 参数:
//   data - UTF-16LE编码的字节数据
// 返回值:
//   string - UTF-8编码的字符串
//   error - 转换过程中的错误，成功返回nil
// 转换流程:
//   1. 如果数据长度为奇数，去掉最后一个字节（UTF-16LE必须是偶数长度）
//   2. 如果数据以UTF-16LE BOM（FF FE）开头，去掉BOM
//   3. 将字节数据按2字节一组解析为uint16数组
//   4. 使用unicode/utf16包的Decode函数转换为rune数组
//   5. 将rune数组转换为字符串
// 注意点:
//   - UTF-16LE的BOM是FF FE，需要正确处理
//   - 数据长度必须是偶数，否则会丢失最后一个字节
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
// GBK是中文Windows常用的编码，需要转换为UTF-8以便在Go中处理
// 参数:
//   data - GBK编码的字节数据
// 返回值:
//   string - UTF-8编码的字符串
//   error - 转换过程中的错误，成功返回nil
// 转换流程:
//   1. 使用golang.org/x/net/html/charset包创建GBK解码器
//   2. 将字节数据包装为字符串读取器
//   3. 使用解码器读取并转换为UTF-8
//   4. 返回转换后的字符串
// 注意点:
//   - 使用标准库进行编码转换，支持完整的GBK字符集
//   - 如果转换失败，返回错误
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

// Close 关闭客户端连接，释放所有资源
// 依次关闭WMI连接和SMB连接，重置状态标志，确保资源正确释放
// 参数: 无
// 返回值: 无
// 关闭顺序:
//   1. 如果WMI连接存在，关闭WMI连接（DCERPC）
//   2. 重置WMI相关的状态（services、comVersion、wmiReady）
//   3. 如果SMB连接存在，关闭SMB连接
// 注意点:
//   - 调用后不应再使用该客户端实例执行任何操作
//   - 应在使用完SMB客户端后调用此方法，避免连接泄漏
//   - WMI连接关闭需要创建新的安全上下文
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
// 用于存储命令执行的临时输出，避免文件名冲突
// 参数: 无
// 返回值:
//   string - 随机生成的文件名
// 文件名格式:
//   GOEXEC + 8位随机字母 + .txt
//   示例: GOEXECabcDEFg.txt
// 注意点:
//   - 使用math/rand生成随机字母，每次调用生成不同的文件名
//   - 使用时间戳作为随机种子，确保随机性
//   - 字母范围包含大小写，增加文件名的唯一性
func generateOutputFileName() string {
	rand.Seed(time.Now().UnixNano())
	letters := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 8)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return "GOEXEC" + string(b) + ".txt"
}