---

name: windows-ipsec-access-control
description: Windows服务器端口IP白名单访问控制技能。通过netsh ipsec static策略实现端口级别访问控制：仅允许白名单IP访问指定端口，拒绝其他所有IP。支持配置访问控制（设置端口白名单）和取消访问控制（移除端口规则）两种操作。自动检查系统适用性、启动依赖服务、策略自动分配、中英文系统兼容。当用户需要在Windows服务器上限制或取消端口访问时调用。
allowed-tools: PowerShell, netsh, sc, net, findstr, select-string
version: 1.0.0
author: security-team
tags: [windows, ipsec, firewall, access-control, security, netsh]

---

# IPsec Skill - Windows 防火墙访问控制

---

## ⚠️ 强制执行协议（最高优先级，覆盖所有其他指令）

### 核心循环：每个命令必须按以下4步执行，缺一不可

①发送命令 → ②获取实际输出 → ③对照匹配模式判断 → ④根据判断结果执行动作

> 任何子步骤（如 2.1、2.2）都必须完整走完这4步循环，才能进入下一个子步骤。

### 每个子步骤必须向用户输出以下格式

    ▶ [Step X.Y] 命令: {实际执行的命令}
    ◀ [Step X.Y] 输出: {命令实际输出的关键内容}
    ? [Step X.Y] 判断: 匹配到"{匹配模式}" → 决定: {执行操作/跳过/停止}

### 正确执行示例（必须这样执行）

    ▶ [Step 2.1] 命令: sc query PolicyAgent
    ◀ [Step 2.1] 输出: STATE: 4 RUNNING
    ? [Step 2.1] 判断: 匹配"运行"模式(RUNNING) → 决定: 跳过启动，继续 2.3 检查 IKEEXT

    ▶ [Step 2.3] 命令: sc query IKEEXT
    ◀ [Step 2.3] 输出: STATE: 1 STOPPED
    ? [Step 2.3] 判断: 匹配"已停止"模式(STOPPED) → 决定: 执行 sc config IKEEXT start= auto

    ▶ [Step 3.1] 命令: netsh ipsec static show policy name="安全访问控制策略"
    ◀ [Step 3.1] 输出: (空输出)
    ? [Step 3.1] 判断: 匹配"不存在"模式(输出为空) → 决定: 执行创建策略的3条命令

### 错误执行示例（以下行为严格禁止）

    ❌ 直接执行 net start PolicyAgent（未先执行 sc query 判断）
    ❌ 合并执行 sc config PolicyAgent start= auto && net start PolicyAgent（批量执行）
    ❌ 假设 PolicyAgent 已运行直接跳过（未获取实际输出就假设）
    ❌ 跳过 Step 2 直接执行 Step 3（跳步）
    ❌ 执行了判断命令但不输出判断结果直接跳到下一步（忽略判断）

### 禁止行为清单

1. **禁止跳过判断**：每个操作命令前必须有对应的判断命令及其真实输出
2. **禁止合并执行**：判断命令和操作命令不可合并为一条，必须分开执行
3. **禁止假设输出**：必须实际执行命令获取真实输出，不可猜测/假设输出内容
4. **禁止跳步**：必须按 Step 1→2→3→4→5→6 顺序，不可跳过任何 Step
5. **禁止批量执行**：每个命令单独执行，获取输出后再执行下一个命令
6. **禁止忽略错误**：命令输出含错误信息时必须停止并报告，不可继续执行
7. **禁止省略输出**：每个命令的输出必须向用户展示，不可隐藏或省略

### 执行前检查清单（开始前必须向用户确认以下信息）

- 端口号已确认: {port}
- 白名单已确认: {空 / 具体IP列表}
- 操作类型已确认: {配置 / 取消配置}
- 当前位置: 准备从 Step 1 开始

> 确认以上信息后，从 Step 1 开始逐条执行。每个命令执行后必须展示输出和判断结果，再进入下一步。

---

## 功能说明

在 Windows 服务器上通过 netsh ipsec static 策略实现端口级别 IP 白名单访问控制，仅允许白名单 IP 访问指定端口，拒绝其他所有 IP。

**输入参数**：端口号（如3389/445/1433）、白名单IP列表（空/具体IP，支持逗号分隔多IP）、操作类型（配置/取消配置）

**白名单模式**：
- 空 → 127.0.0.1自动通过，仅本机可访问，拒绝所有外部IP
- 具体IP → 为每个IP创建TCP+UDP允许规则，拒绝其他所有IP

**自动化能力**：检查系统适用性(secpol.msc) → 启动依赖服务(PolicyAgent/IKEEXT) → 创建策略结构(策略/筛选器操作/筛选器列表/规则) → 配置端口规则(TCP+UDP筛选器) → 筛选器去重 → 刷新策略(assign=n→y) → 验证配置(分配状态/规则存在性/IP数量匹配)

---

## 执行规则（必须严格遵守）

1. **按序执行**：必须按 Step 1→2→3→4→5→6 顺序执行，不可跳步、不可逆序
2. **先判断后执行**：每个 Step 必须先执行判断命令，获取输出后再决定是否执行操作命令
3. **判断不通过则停止**：判断条件不满足时，立即停止并返回错误信息，不继续后续步骤
4. **逐条执行**：每个命令单独执行，获取输出后再决定下一步操作
5. **不猜测**：所有判断必须基于命令输出的实际内容，不可猜测或假设

## 判断输出解析模板

| 判断对象 | 执行命令 | "存在"匹配模式 | "不存在"匹配模式 |
|---------|---------|--------------|---------------|
| 策略是否存在 | `netsh ipsec static show policy name="安全访问控制策略"` | 输出含 `安全访问控制策略` | 输出为空或含 `找不到`/`not found` |
| 筛选器列表是否存在 | `netsh ipsec static show filterlist name="{name}"` | 输出含 `{name}` | 输出为空或含 `找不到`/`not found` |
| 规则是否存在 | `netsh ipsec static show rule name="{name}" policy="安全访问控制策略"` | 输出含 `{name}` | 输出为空或含 `找不到`/`not found` |
| 筛选器是否存在(TCP) | `netsh ipsec static show filterlist name="{name}" level=verbose` | 输出含 `{ip}` 和 `TCP` 和 `{port}` | 无同时匹配三项的行 |
| 筛选器是否存在(UDP) | `netsh ipsec static show filterlist name="{name}" level=verbose` | 输出含 `{ip}` 和 `UDP` 和 `{port}` | 无同时匹配三项的行 |
| 策略是否已分配 | `netsh ipsec static show policy name="安全访问控制策略"` | 输出含 `已分配:是`/`Assigned=YES` | 输出含 `已分配:否`/`Assigned=NO` |
| 服务是否运行 | `sc query {service}` | 输出含 `RUNNING`/`运行` | 输出含 `STOPPED`/`已停止` |

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

| 名称   | 值          | 说明                       |
| ---- | ---------- | ------------------------ |
| 策略名称 | `安全访问控制策略` | IPsec 策略名称               |
| 允许操作 | `允许访问`     | 允许筛选器操作名称（action=permit） |
| 拒绝操作 | `拒绝访问`     | 拒绝筛选器操作名称（action=block）  |

---

## 配置流程（6步，每步必须先判断再执行）

### Step 1: 检查系统适用性

1.1 执行命令：`powershell.exe -Command "Test-Path C:\Windows\System32\secpol.msc"`
1.2 判断输出：输出是否为 `True`？
  - 是 → 继续 1.3
  - 否 → **停止**，返回错误："系统不支持 IPsec 策略"

1.3 执行命令：`netsh ipsec static show version`
1.4 判断输出：命令是否成功执行且无错误信息？
  - 是 → 继续 Step 2
  - 否 → **停止**，返回错误："系统不支持 netsh ipsec static"

### Step 2: 检查并启动依赖服务

2.1 执行命令：`sc query PolicyAgent`
2.2 判断输出：输出是否含 `RUNNING` 或 `运行`？（参照"判断输出解析模板"）
  - 是 → 跳过，继续 2.3
  - 否 → 执行以下命令：
    - `sc config PolicyAgent start= auto`
    - `net start PolicyAgent`
    - 继续到 2.3

2.3 执行命令：`sc query IKEEXT`
2.4 判断输出：输出是否含 `RUNNING` 或 `运行`？
  - 是 → 跳过，继续 Step 3
  - 否 → 执行以下命令：
    - `sc config IKEEXT start= auto`
    - `net start IKEEXT`
    - 继续 Step 3

### Step 3: 检查策略是否存在

3.1 执行命令：`netsh ipsec static show policy name="安全访问控制策略"`
3.2 判断输出：输出是否含 `安全访问控制策略`？（参照"判断输出解析模板"）
  - 是 → 策略已存在，跳过创建，继续 Step 4
  - 否 → 策略不存在，依次执行以下3条命令创建策略结构：
    1. `netsh ipsec static add policy name="安全访问控制策略"`
    2. `netsh ipsec static add filteraction name="允许访问" action=permit`
    3. `netsh ipsec static add filteraction name="拒绝访问" action=block`
    - 创建完成后继续 Step 4

### Step 4: 配置端口规则

4.0 判断白名单是否为空？

**分支A：白名单为空（只配置拒绝规则）**

4A.1 执行命令：`netsh ipsec static show filterlist name="拒绝{port}访问"`
4A.2 判断输出：筛选器列表是否存在？（参照"判断输出解析模板"）
  - 否 → 执行：`netsh ipsec static add filterlist name="拒绝{port}访问"`
  - 是 → 跳过

4A.3 执行命令：`netsh ipsec static show filterlist name="拒绝{port}访问" level=verbose`
4A.4 判断输出：是否含 `any` 和 `TCP` 和 `{port}`？
  - 否 → 执行：`netsh ipsec static add filter filterlist="拒绝{port}访问" srcaddr=any dstaddr=me dstport={port} protocol=TCP`
  - 是 → 跳过

4A.5 执行命令：`netsh ipsec static show filterlist name="拒绝{port}访问" level=verbose`
4A.6 判断输出：是否含 `any` 和 `UDP` 和 `{port}`？
  - 否 → 执行：`netsh ipsec static add filter filterlist="拒绝{port}访问" srcaddr=any dstaddr=me dstport={port} protocol=UDP`
  - 是 → 跳过

4A.7 执行命令：`netsh ipsec static show rule name="拒绝{port}访问" policy="安全访问控制策略"`
4A.8 判断输出：规则是否存在？
  - 否 → 执行：`netsh ipsec static add rule name="拒绝{port}访问" policy="安全访问控制策略" filterlist="拒绝{port}访问" filteraction="拒绝访问"`
  - 是 → 跳过
  - 完成后跳到 Step 5

**分支B：白名单非空（配置允许规则 + 拒绝规则）**

4B.1 执行命令：`netsh ipsec static show filterlist name="允许{port}访问"`
4B.2 判断输出：筛选器列表是否存在？
  - 否 → 执行：`netsh ipsec static add filterlist name="允许{port}访问"`
  - 是 → 跳过

4B.3 对每个白名单IP，执行命令：`netsh ipsec static show filterlist name="允许{port}访问" level=verbose`
4B.4 判断输出：是否含 `{ip}` 和 `TCP` 和 `{port}`？
  - 否 → 执行：`netsh ipsec static add filter filterlist="允许{port}访问" srcaddr={ip} dstaddr=me dstport={port} protocol=TCP`
  - 是 → 跳过

4B.5 判断输出：是否含 `{ip}` 和 `UDP` 和 `{port}`？
  - 否 → 执行：`netsh ipsec static add filter filterlist="允许{port}访问" srcaddr={ip} dstaddr=me dstport={port} protocol=UDP`
  - 是 → 跳过
  - 对所有IP重复 4B.3-4B.5

4B.6 执行命令：`netsh ipsec static show filterlist name="拒绝{port}访问"`
4B.7 判断输出：筛选器列表是否存在？
  - 否 → 执行：`netsh ipsec static add filterlist name="拒绝{port}访问"`
  - 是 → 跳过

4B.8 执行命令：`netsh ipsec static show filterlist name="拒绝{port}访问" level=verbose`
4B.9 判断输出：是否含 `any` 和 `TCP` 和 `{port}`？
  - 否 → 执行：`netsh ipsec static add filter filterlist="拒绝{port}访问" srcaddr=any dstaddr=me dstport={port} protocol=TCP`
  - 是 → 跳过

4B.10 判断输出：是否含 `any` 和 `UDP` 和 `{port}`？
  - 否 → 执行：`netsh ipsec static add filter filterlist="拒绝{port}访问" srcaddr=any dstaddr=me dstport={port} protocol=UDP`
  - 是 → 跳过

4B.11 执行命令：`netsh ipsec static show rule name="允许{port}访问" policy="安全访问控制策略"`
4B.12 判断输出：规则是否存在？
  - 否 → 执行：`netsh ipsec static add rule name="允许{port}访问" policy="安全访问控制策略" filterlist="允许{port}访问" filteraction="允许访问"`
  - 是 → 跳过

4B.13 执行命令：`netsh ipsec static show rule name="拒绝{port}访问" policy="安全访问控制策略"`
4B.14 判断输出：规则是否存在？
  - 否 → 执行：`netsh ipsec static add rule name="拒绝{port}访问" policy="安全访问控制策略" filterlist="拒绝{port}访问" filteraction="拒绝访问"`
  - 是 → 跳过

### Step 5: 刷新策略

5.1 执行命令：`netsh ipsec static set policy name="安全访问控制策略" assign=n`
5.2 执行命令：`netsh ipsec static set policy name="安全访问控制策略" assign=y`
5.3 继续 Step 6

> 必须先 assign=n 再 assign=y，不可直接 assign=y。

### Step 6: 验证配置

6.1 执行命令：`netsh ipsec static show policy name="安全访问控制策略"`
6.2 判断输出：是否含 `已分配:是` 或 `Assigned=YES`？
  - 否 → **停止**，返回错误："策略未分配"
  - 是 → 继续

6.3 执行命令：`netsh ipsec static show rule name="拒绝{port}访问" policy="安全访问控制策略"`
6.4 判断输出：拒绝规则是否存在？
  - 否 → **停止**，返回错误："拒绝规则不存在"
  - 是 → 继续

6.5 如果白名单非空，执行命令：`netsh ipsec static show rule name="允许{port}访问" policy="安全访问控制策略"`
6.6 判断输出：允许规则是否存在？
  - 否 → **停止**，返回错误："允许规则不存在"
  - 是 → 继续

6.7 如果白名单非空，执行命令：`netsh ipsec static show filterlist name="允许{port}访问" level=verbose`
6.8 判断输出：筛选器中的IP数量是否与白名单IP数量匹配？
  - 否 → **停止**，返回错误："白名单IP数量不匹配"
  - 是 → 验证通过，返回成功

---

## 取消配置流程（6步，每步必须先判断再执行）

### Step 1-3: 与配置流程相同的环境验证

Step 1: 执行配置流程的 Step 1（检查系统适用性）
Step 2: 执行配置流程的 Step 2（检查并启动依赖服务）
Step 3: 执行配置流程的 Step 3.1-3.2（检查策略是否存在）
  - 策略不存在 → **停止**，返回："策略不存在，无需取消"
  - 策略已存在 → 继续 Step 4

### Step 4: 删除端口规则和筛选器

4.0 判断白名单是否为空？

**分支A：白名单非空（只删除指定IP）**

4A.1 执行命令：`netsh ipsec static show filterlist name="允许{port}访问"`
4A.2 判断输出：筛选器列表是否存在？
  - 否 → **停止**，返回错误："允许筛选器列表不存在，无法删除白名单IP"
  - 是 → 继续

4A.3 执行命令：`netsh ipsec static show filterlist name="允许{port}访问" level=verbose`
4A.4 对每个要删除的IP，判断输出：是否含 `{ip}` 和 `TCP` 和 `{port}`？
  - 是 → 执行：`netsh ipsec static delete filter filterlist="允许{port}访问" srcaddr={ip} dstaddr=me dstport={port} protocol=TCP`
  - 否 → 跳过

4A.5 判断输出：是否含 `{ip}` 和 `UDP` 和 `{port}`？
  - 是 → 执行：`netsh ipsec static delete filter filterlist="允许{port}访问" srcaddr={ip} dstaddr=me dstport={port} protocol=UDP`
  - 否 → 跳过
  - 跳到 Step 5

**分支B：白名单为空（完整删除）**

4B.1 执行命令：`netsh ipsec static show rule name="允许{port}访问" policy="安全访问控制策略"`
4B.2 判断输出：允许规则是否存在？
  - 是 → 执行：`netsh ipsec static delete rule name="允许{port}访问" policy="安全访问控制策略"`
  - 否 → 跳过

4B.3 执行命令：`netsh ipsec static show rule name="拒绝{port}访问" policy="安全访问控制策略"`
4B.4 判断输出：拒绝规则是否存在？
  - 是 → 执行：`netsh ipsec static delete rule name="拒绝{port}访问" policy="安全访问控制策略"`
  - 否 → 跳过

4B.5 执行命令：`netsh ipsec static show filterlist name="允许{port}访问"`
4B.6 判断输出：允许筛选器列表是否存在？
  - 是 → 执行：`netsh ipsec static delete filterlist name="允许{port}访问"`
  - 否 → 跳过

4B.7 执行命令：`netsh ipsec static show filterlist name="拒绝{port}访问"`
4B.8 判断输出：拒绝筛选器列表是否存在？
  - 是 → 执行：`netsh ipsec static delete filterlist name="拒绝{port}访问"`
  - 否 → 跳过

### Step 5: 刷新策略

5.1 执行命令：`netsh ipsec static set policy name="安全访问控制策略" assign=n`
5.2 执行命令：`netsh ipsec static set policy name="安全访问控制策略" assign=y`
5.3 继续 Step 6

> 必须先 assign=n 再 assign=y。策略本身不删除。

### Step 6: 验证取消配置

**白名单非空（只删除指定IP）：**

6A.1 执行命令：`netsh ipsec static show filterlist name="允许{port}访问" level=verbose`
6A.2 判断输出：指定IP是否已从筛选器列表中删除？（不再含 `{ip}` 和 `TCP`/`UDP` 和 `{port}`）
  - 仍在 → **停止**，返回错误："IP删除失败"
  - 已删 → 验证通过

**白名单为空（完整删除）：**

6B.1 执行命令：`netsh ipsec static show rule name="允许{port}访问" policy="安全访问控制策略"`
6B.2 判断输出：允许规则是否已删除？
  - 否 → **停止**，返回错误："允许规则删除失败"
  - 是 → 继续

6B.3 执行命令：`netsh ipsec static show rule name="拒绝{port}访问" policy="安全访问控制策略"`
6B.4 判断输出：拒绝规则是否已删除？
  - 否 → **停止**，返回错误："拒绝规则删除失败"
  - 是 → 继续

6B.5 执行命令：`netsh ipsec static show filterlist name="允许{port}访问"`
6B.6 判断输出：允许筛选器列表是否已删除？
  - 否 → **停止**，返回错误："允许筛选器列表删除失败"
  - 是 → 继续

6B.7 执行命令：`netsh ipsec static show filterlist name="拒绝{port}访问"`
6B.8 判断输出：拒绝筛选器列表是否已删除？
  - 否 → **停止**，返回错误："拒绝筛选器列表删除失败"
  - 是 → 验证通过

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

| 中文格式           | 英文格式                    |
| -------------- | ----------------------- |
| `名称: xxx`      | `Name = xxx`            |
| `已分配: 是`       | `Assigned = YES`        |
| `源 IP 地址: xxx` | `Source Address: xxx`   |
| `协议: TCP`      | `Protocol: TCP`         |
| `目标端口: xxx`    | `Destination Port: xxx` |

协议标准化：

| 原始值                      | 标准化结果 |
| ------------------------ | ----- |
| `6` / `TCP` / `传输控制协议`   | `TCP` |
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
