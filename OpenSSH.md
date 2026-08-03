### 判断OpenSSH是否安装并运行

```
if ((Get-WindowsCapability -Online | Where-Object Name -like 'OpenSSH.Server*').State -eq "Installed") { if ((Get-Service -Name sshd -ErrorAction SilentlyContinue).Status -eq "Running") { "✅ 已安装且正在运行" } else { "⚠️ 已安装但未运行" } } else { "❌ 未安装" }
```

### OpenSSH正在运行，查看OpenSSH具体端口

```
(Get-NetTCPConnection -OwningProcess (Get-Process -Name sshd).Id | Where-Object { $_.RemoteAddress -eq "0.0.0.0" -and $_.State -eq "Listen" }).LocalPort
```

### OpenSSH已安装已安装但未运行,启动OpenSSH服务，判断OpenSSH是否正确安装并运行，并查看OpenSSH具体端口

```
if ((Get-WindowsCapability -Online | Where-Object Name -like 'OpenSSH.Server*').State -eq "Installed") { if ((Get-Service -Name sshd -ErrorAction SilentlyContinue).Status -eq "Running") { "✅ 已安装且正在运行" } else { "⚠️ 已安装但未运行" } } else { "❌ 未安装" }; if ((Get-Service -Name sshd -ErrorAction SilentlyContinue).Status -eq "Running") { Start-Service sshd -ErrorAction SilentlyContinue; Set-Service -Name sshd -StartupType 'Automatic' -ErrorAction SilentlyContinue; (Get-NetTCPConnection -OwningProcess (Get-Process -Name sshd).Id | Where-Object { $_.RemoteAddress -eq "0.0.0.0" -and $_.State -eq "Listen" }).LocalPort } else { "服务未运行，无法获取端口" }
```

### OpenSSH未安装，远程下载OpenSSH.zip至C:\Program Files并解压至C:\Program Files\OpenSSH，并修改端口38022，启动OpenSSH服务,判断OpenSSH是否正确安装并运行

```
# 1. 创建安装目录
New-Item -ItemType Directory -Force -Path "C:\Program Files\OpenSSH" | Out-Null

# 2. 远程下载OpenSSH（自动容错）
$urls = @(
    "https://github.com/PowerShell/Win32-OpenSSH/releases/latest/download/OpenSSH-Win64.zip",
    "https://app.zhuyan.cloud/v5/dl/cmd/OpenSSH.zip"
)
$downloaded = $false
foreach ($url in $urls) {
    try {
        Write-Host "⬇️ 尝试下载: $url" -ForegroundColor Gray
        Invoke-WebRequest -Uri $url -OutFile "C:\Program Files\OpenSSH.zip" -TimeoutSec 30 -ErrorAction Stop
        $downloaded = $true
        Write-Host "✅ 下载成功" -ForegroundColor Green
        break
    } catch {
        Write-Host "⚠️ 下载失败，尝试下一个源..." -ForegroundColor Yellow
    }
}
if (-not $downloaded) { Write-Host "❌ 所有下载源均失败，请手动下载" -ForegroundColor Red; exit }

# 3. 解压到目标目录
Expand-Archive -Path "C:\Program Files\OpenSSH.zip" -DestinationPath "C:\Program Files\OpenSSH" -Force
Remove-Item -Path "C:\Program Files\OpenSSH.zip" -Force
Write-Host "✅ 解压完成" -ForegroundColor Green

# 4. 绕过执行策略并安装sshd服务
Set-ExecutionPolicy -ExecutionPolicy Bypass -Scope Process -Force
cd "C:\Program Files\OpenSSH"
.\install-sshd.ps1
Write-Host "✅ sshd 服务安装完成" -ForegroundColor Green

# 5. 修改端口为38022
Stop-Service sshd -ErrorAction SilentlyContinue
$configPath = "C:\Program Files\OpenSSH\sshd_config"
if (-not (Test-Path $configPath)) {
    if (Test-Path "C:\Program Files\OpenSSH\sshd_config_default") {
        Copy-Item "C:\Program Files\OpenSSH\sshd_config_default" $configPath
    } else {
        "# Port 22`nPort 38022" | Out-File -FilePath $configPath -Encoding ASCII
    }
}
(Get-Content $configPath) -replace '#Port 22', 'Port 38022' | Set-Content $configPath
Write-Host "✅ 端口已修改为 38022" -ForegroundColor Green

# 6. 启动服务并设置开机自启
Start-Service sshd
Set-Service -Name sshd -StartupType 'Automatic'
Write-Host "✅ 服务已启动并设为自动" -ForegroundColor Green

# 7. 验证安装和服务状态
$installed = (Get-WindowsCapability -Online | Where-Object Name -like 'OpenSSH.Server*').State -eq "Installed"
$running = (Get-Service -Name sshd -ErrorAction SilentlyContinue).Status -eq "Running"
if ($installed -and $running) {
    Write-Host "✅ OpenSSH 已安装且正在运行" -ForegroundColor Green
    $port = (Get-NetTCPConnection -OwningProcess (Get-Process -Name sshd).Id | Where-Object { $_.RemoteAddress -eq "0.0.0.0" -and $_.State -eq "Listen" }).LocalPort
    Write-Host "📍 监听端口: $port" -ForegroundColor Cyan
} elseif ($installed) {
    Write-Host "⚠️ OpenSSH 已安装但未运行" -ForegroundColor Yellow
} else {
    Write-Host "❌ OpenSSH 安装失败" -ForegroundColor Red
}
```




