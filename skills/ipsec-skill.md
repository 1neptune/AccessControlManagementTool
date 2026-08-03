# IPsec Skill - Windows 防火墙访问控制

---

## 功能说明

在 Windows 服务器上配置基于 IPsec 的端口访问控制。通过远程执行 PowerShell 命令，使用 `netsh ipsec static` 命令体系，实现：自动检查系统适用性、启动依赖服务、创建策略结构、配置端口规则、刷新并验证配置。支持中英文系统环境。

---

## 配置端口访问控制

为指定端口设置访问控制，只有白名单中的 IP 可以访问该端口，其他 IP 全部拒绝。

- **白名单为空值时**：只配置拒绝规则，拒绝所有外部访问，系统回环 127.0.0.1 自动通过无需配置。
  - 例：配置端口 3389，白名单为空 → 服务器本机 127.0.0.1 可以访问 3389 端口，其他所有外部 IP 全部被拒绝，远程桌面无法从外部连接。
- **白名单有具体 IP 时**：为每个白名单 IP 创建允许规则（TCP+UDP），同时创建拒绝规则拦截非白名单访问。支持逗号分隔多 IP。
  - 例1：配置端口 3389，白名单为 "192.168.1.100,10.0.0.5" → 只有这两个 IP 可以访问 3389 端口，其他 IP 全部被拒绝。
  - 例2：配置端口 445，白名单为 "192.168.1.0" → 只有 192.168.1.0 可以访问 445 端口，其他 IP 全部被拒绝。

## 取消端口访问控制

移除指定端口的访问控制，恢复端口为默认状态。

- **白名单为空值时**：完整删除该端口的所有访问控制规则和筛选器列表，策略本身保留。
  - 例：取消端口 3389 的访问控制，白名单为空 → 删除端口 3389 的所有 IPsec 规则和筛选器列表。
- **白名单有具体 IP 时**：只从允许筛选器列表中删除指定的 IP（TCP+UDP 筛选器），保留其余白名单 IP 和所有规则不变。
  - 例：取消端口 3389 白名单 IP "10.0.0.5" → 从筛选器列表中删除指定 IP。

---

## 常量定义

| 名称 | 值 | 说明 |
|------|-----|------|
| 策略名称 | `安全访问控制策略` | IPsec 策略名称 |
| 允许操作 | `允许访问` | 允许筛选器操作名称（action=permit） |
| 拒绝操作 | `拒绝访问` | 拒绝筛选器操作名称（action=block） |

---

## 配置流程（6步，每步必须先判断再执行）

### Step 1: 检查系统适用性

```
判断1: secpol.msc 是否存在？(检查 C:\Windows\System32\secpol.msc)
  不存在 → 返回错误（系统不支持 IPsec 策略）
  存在   → 判断2: netsh ipsec static 命令是否可用？
            不可用 → 返回错误（系统不支持 netsh ipsec static）
            可用   → 继续
```

### Step 2: 检查并启动依赖服务

```
判断1: PolicyAgent（IPsec 策略代理）是否正在运行？(sc query PolicyAgent)
  未运行 → sc config PolicyAgent start= auto → net start PolicyAgent
  已运行 → 跳过

判断2: IKEEXT（IKE 扩展）是否正在运行？(sc query IKEEXT)
  未运行 → sc config IKEEXT start= auto → net start IKEEXT
  已运行 → 跳过
```

### Step 3: 检查策略是否存在

```
判断: 策略 "安全访问控制策略" 是否存在？
  netsh ipsec static show policy name="安全访问控制策略"
  正则匹配输出: "名称:.*安全访问控制策略" 或 "Name.*=.*安全访问控制策略"

  不存在 → 创建策略结构:
    1. netsh ipsec static add policy name="安全访问控制策略"
    2. netsh ipsec static add filteraction name="允许访问" action=permit
    3. netsh ipsec static add filteraction name="拒绝访问" action=block
  已存在 → 跳过
```

### Step 4: 配置端口规则

```
判断0: 白名单是否为空？

  空（只配置拒绝规则）:
    判断1: 拒绝筛选器列表 "拒绝{port}访问" 是否已存在？(show filterlist name="拒绝{port}访问")
      不存在 → 创建: netsh ipsec static add filterlist name="拒绝{port}访问"
      已存在 → 跳过创建

    判断2: 拒绝筛选器列表中是否已有 TCP 拒绝筛选器？(show filterlist name="拒绝{port}访问" level=verbose → 匹配 srcaddr=any protocol=TCP dstport={port})
      不存在 → 添加: netsh ipsec static add filter filterlist="拒绝{port}访问" srcaddr=any dstaddr=me dstport={port} protocol=TCP
      已存在 → 跳过

    判断3: 拒绝筛选器列表中是否已有 UDP 拒绝筛选器？(show filterlist name="拒绝{port}访问" level=verbose → 匹配 srcaddr=any protocol=UDP dstport={port})
      不存在 → 添加: netsh ipsec static add filter filterlist="拒绝{port}访问" srcaddr=any dstaddr=me dstport={port} protocol=UDP
      已存在 → 跳过

    判断4: 拒绝筛选器列表中筛选器数量是否超过2条？
      超过2条 → 自动删除多余的，只保留第一条TCP和第一条UDP

    判断5: 拒绝规则 "拒绝{port}访问" 是否已存在？(show rule name="拒绝{port}访问" policy="安全访问控制策略")
      不存在 → 绑定: netsh ipsec static add rule name="拒绝{port}访问" policy="安全访问控制策略" filterlist="拒绝{port}访问" filteraction="拒绝访问"
      已存在 → 跳过

  非空（配置允许规则 + 拒绝规则）:
    判断1: 允许筛选器列表 "允许{port}访问" 是否已存在？(show filterlist name="允许{port}访问")
      不存在 → 创建: netsh ipsec static add filterlist name="允许{port}访问"
      已存在 → 获取现有筛选器列表(level=verbose) → 解析已有IP(格式: IP/协议/端口) → 用于去重

    逐个白名单IP判断2: 是否已在允许筛选器列表中？(解析已有IP列表匹配 {ip}/TCP/{port} 和 {ip}/UDP/{port})
      不在 → 添加(TCP+UDP):
        netsh ipsec static add filter filterlist="允许{port}访问" srcaddr={ip} dstaddr=me dstport={port} protocol=TCP
        netsh ipsec static add filter filterlist="允许{port}访问" srcaddr={ip} dstaddr=me dstport={port} protocol=UDP
      已在 → 跳过

    判断3: 拒绝筛选器列表 "拒绝{port}访问" 是否已存在？(show filterlist name="拒绝{port}访问")
      不存在 → 创建: netsh ipsec static add filterlist name="拒绝{port}访问"
      已存在 → 跳过创建

    判断4: 拒绝筛选器列表中是否已有 TCP 拒绝筛选器？(show filterlist name="拒绝{port}访问" level=verbose → 匹配 srcaddr=any protocol=TCP dstport={port})
      不存在 → 添加: netsh ipsec static add filter filterlist="拒绝{port}访问" srcaddr=any dstaddr=me dstport={port} protocol=TCP
      已存在 → 跳过

    判断5: 拒绝筛选器列表中是否已有 UDP 拒绝筛选器？(show filterlist name="拒绝{port}访问" level=verbose → 匹配 srcaddr=any protocol=UDP dstport={port})
      不存在 → 添加: netsh ipsec static add filter filterlist="拒绝{port}访问" srcaddr=any dstaddr=me dstport={port} protocol=UDP
      已存在 → 跳过

    判断6: 拒绝筛选器列表中筛选器数量是否超过2条？
      超过2条 → 自动删除多余的，只保留第一条TCP和第一条UDP

    判断7: 允许规则 "允许{port}访问" 是否已存在？(show rule name="允许{port}访问" policy="安全访问控制策略")
      不存在 → 绑定: netsh ipsec static add rule name="允许{port}访问" policy="安全访问控制策略" filterlist="允许{port}访问" filteraction="允许访问"
      已存在 → 跳过

    判断8: 拒绝规则 "拒绝{port}访问" 是否已存在？(show rule name="拒绝{port}访问" policy="安全访问控制策略")
      不存在 → 绑定: netsh ipsec static add rule name="拒绝{port}访问" policy="安全访问控制策略" filterlist="拒绝{port}访问" filteraction="拒绝访问"
      已存在 → 跳过
```

### Step 5: 刷新策略

```
执行（必须先取消分配再重新分配）:
  netsh ipsec static set policy name="安全访问控制策略" assign=n
  netsh ipsec static set policy name="安全访问控制策略" assign=y
```

### Step 6: 验证配置

```
判断1: 策略是否已分配？(show policy → 匹配 "已分配:是" 或 "Assigned=YES")
  未分配 → 返回错误
  已分配 → 继续

判断2: 允许规则是否存在？(show rule name="允许{port}访问" policy="安全访问控制策略")
  白名单为空时跳过此判断
  不存在 → 返回错误
  存在   → 继续

判断3: 拒绝规则是否存在？(show rule name="拒绝{port}访问" policy="安全访问控制策略")
  不存在 → 返回错误
  存在   → 继续

判断4: 允许筛选器列表是否存在？(show filterlist name="允许{port}访问")
  白名单为空时跳过此判断
  不存在 → 返回错误
  存在   → 继续

判断5: 拒绝筛选器列表是否存在？(show filterlist name="拒绝{port}访问")
  不存在 → 返回错误
  存在   → 继续

判断6: 允许筛选器操作是否存在？(show filteraction name="允许访问")
  不存在 → 返回错误
  存在   → 继续

判断7: 拒绝筛选器操作是否存在？(show filteraction name="拒绝访问")
  不存在 → 返回错误
  存在   → 继续

判断8: 白名单不为空时，允许筛选器列表中的IP数量是否匹配？
  不匹配 → 返回错误
  匹配   → 验证通过
```

---

## 取消配置流程（6步，每步必须先判断再执行）

### Step 1-3: 与配置流程相同的环境验证

```
Step 1: 检查系统适用性（同配置）
Step 2: 检查并启动依赖服务（同配置）
Step 3: 检查策略是否存在
        判断: 不存在 → 直接返回（无需取消）
```

### Step 4: 删除端口规则和筛选器

```
判断1: 白名单是否为空？

  非空（只删除指定IP）:
    判断2: 允许筛选器列表 "允许{port}访问" 是否存在？
      不存在 → 返回错误（无法删除不存在的白名单）
      已存在 → 获取现有筛选器列表(level=verbose) → 解析现有IP列表(格式: IP/协议/端口)

    逐个IP判断: 是否在现有筛选器列表中？
      在列表中   → 删除TCP和UDP筛选器:
        netsh ipsec static delete filter filterlist="允许{port}访问" srcaddr={ip} dstaddr=me dstport={port} protocol=TCP
        netsh ipsec static delete filter filterlist="允许{port}访问" srcaddr={ip} dstaddr=me dstport={port} protocol=UDP
      不在列表中 → 跳过

  空（完整删除）:
    判断3: 规则是否存在？
      存在 → 删除规则:
        netsh ipsec static delete rule name="允许{port}访问" policy="安全访问控制策略"
        netsh ipsec static delete rule name="拒绝{port}访问" policy="安全访问控制策略"
      不存在 → 跳过

    判断4: 筛选器列表是否存在？
      存在 → 删除筛选器列表:
        netsh ipsec static delete filterlist name="允许{port}访问"
        netsh ipsec static delete filterlist name="拒绝{port}访问"
      不存在 → 跳过
```

### Step 5: 刷新策略

```
执行（必须先取消分配再重新分配）:
  netsh ipsec static set policy name="安全访问控制策略" assign=n
  netsh ipsec static set policy name="安全访问控制策略" assign=y
```

### Step 6: 验证取消配置

```
非空（只删除指定IP）:
  判断: 指定IP是否已从允许筛选器列表中删除？
    仍在 → 返回错误
    已删 → 验证通过

空（完整删除）:
  判断1: 允许规则是否已删除？ → 已删除 → 继续
  判断2: 拒绝规则是否已删除？ → 已删除 → 继续
  判断3: 允许筛选器列表是否已删除？ → 已删除 → 继续
  判断4: 拒绝筛选器列表是否已删除？ → 已删除 → 验证通过
```

---

## IPsec 策略结构

```
策略: 安全访问控制策略
├── 筛选器操作:
│   ├── 允许访问 (permit)
│   └── 拒绝访问 (block)
├── 端口 {port} 规则:
│   ├── 允许{port}访问 (规则)
│   │   ├── 筛选器列表: 允许{port}访问
│   │   │   ├── {ip1}/TCP/{port}
│   │   │   ├── {ip1}/UDP/{port}
│   │   │   ├── {ip2}/TCP/{port}
│   │   │   └── {ip2}/UDP/{port}
│   │   └── 筛选器操作: 允许访问
│   └── 拒绝{port}访问 (规则)
│       ├── 筛选器列表: 拒绝{port}访问
│       │   ├── any/TCP/{port}
│       │   └── any/UDP/{port}
│       └── 筛选器操作: 拒绝访问
```

---

## 中英文系统兼容

所有 `show` 命令的输出解析都使用正则匹配中英文两种格式：

| 中文格式 | 英文格式 |
|---------|---------|
| `名称: xxx` | `Name = xxx` |
| `已分配: 是` | `Assigned = YES` |
| `源 IP 地址: xxx` | `Source Address: xxx` |
| `协议: TCP` | `Protocol: TCP` |
| `目标端口: xxx` | `Destination Port: xxx` |

协议标准化：

| 原始值 | 标准化结果 |
|--------|-----------|
| `6` / `TCP` / `传输控制协议` | `TCP` |
| `17` / `UDP` / `用户数据报协议` | `UDP` |

---

## 铁律（不可违反）

1. **策略不删除**: 取消配置时只删除规则和筛选器列表，策略本身 "安全访问控制策略" 始终保留
2. **刷新必须先取消再分配**: 必须先 `assign=n` 再 `assign=y`，不能直接 `assign=y`
3. **白名单为空只配拒绝**: 白名单为空时只创建拒绝规则，不创建允许规则和允许筛选器列表
4. **筛选器去重**: 添加前必须获取现有筛选器列表(level=verbose)并解析已有IP，跳过已存在的
5. **拒绝筛选器最多2条**: 拒绝列表最多保留1条TCP + 1条UDP，超过自动清理
6. **删除必须验证**: 删除操作后必须验证IP或规则确实已删除
7. **中英文兼容**: 所有输出解析必须同时匹配中英文两种格式
8. **命名固定**: 策略="安全访问控制策略"，允许操作="允许访问"，拒绝操作="拒绝访问"，筛选器列表="允许{port}访问"/"拒绝{port}访问"
