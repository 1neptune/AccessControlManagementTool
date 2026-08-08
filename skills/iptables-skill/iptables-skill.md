---

name: iptables-access-control
description: Linux服务器端口IP白名单访问控制技能。通过iptables+ipset实现端口级别访问控制：仅允许白名单IP访问指定端口，拒绝其他所有IP。支持配置访问控制（设置端口白名单）和取消访问控制（移除端口规则）两种操作。自动适配Docker环境、检测发行版、规则持久化。当用户需要在Linux服务器上限制或取消端口访问时调用。
allowed-tools: Bash, iptables, ipset, systemctl, docker, ss, grep, awk, sed
version: 2.0.0
author: security-team
tags: [linux, firewall, iptables, ipset, docker, security, access-control]

---

# iptables Skill - Linux 防火墙访问控制

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

    ▶ [Step 2.1] 命令: systemctl is-active firewalld
    ◀ [Step 2.1] 输出: active
    ? [Step 2.1] 判断: 匹配"是"模式(active) → 决定: 执行 systemctl stop firewalld

    ▶ [Step 2.2] 命令: systemctl stop firewalld
    ◀ [Step 2.2] 输出: (无输出)
    ? [Step 2.2] 判断: 命令执行完成 → 决定: 继续 systemctl disable firewalld

    ▶ [Step 2.3] 命令: systemctl disable firewalld
    ◀ [Step 2.3] 输出: Removed symlink /etc/systemd/...
    ? [Step 2.3] 判断: 命令执行完成 → 决定: 继续 Step 2.3 检查 ufw

### 错误执行示例（以下行为严格禁止）

    ❌ 直接执行 systemctl stop firewalld（未先执行 is-active 判断）
    ❌ 合并执行 systemctl stop firewalld && systemctl disable firewalld（批量执行）
    ❌ 假设 firewalld 是 active 的直接 stop（未获取实际输出就假设）
    ❌ 跳过 Step 2 直接执行 Step 3（跳步）
    ❌ 执行了判断命令但不输出判断结果直接跳到下一步（忽略判断）

### 禁止行为清单

1. **禁止跳过判断**：每个操作命令前必须有对应的判断命令及其真实输出
2. **禁止合并执行**：判断命令和操作命令不可合并为一条，必须分开执行
3. **禁止假设输出**：必须实际执行命令获取真实输出，不可猜测/假设输出内容
4. **禁止跳步**：必须按 Step 1→2→3→4→5→6→7→8 顺序，不可跳过任何 Step
5. **禁止批量执行**：每个命令单独执行，获取输出后再执行下一个命令
6. **禁止忽略错误**：命令输出含错误信息时必须停止并报告，不可继续执行
7. **禁止省略输出**：每个命令的输出必须向用户展示，不可隐藏或省略

### 执行前检查清单（开始前必须向用户确认以下信息）

- 端口号已确认: {port}
- 白名单已确认: {空 / all / 具体IP列表}
- 操作类型已确认: {配置 / 取消配置}
- 当前位置: 准备从 Step 1 开始

> 确认以上信息后，从 Step 1 开始逐条执行。每个命令执行后必须展示输出和判断结果，再进入下一步。

---

## 功能说明

在 Linux 服务器上通过 iptables + ipset 实现端口级别 IP 白名单访问控制，仅允许白名单 IP 访问指定端口，拒绝其他所有 IP。

**输入参数**：端口号（如22/80/443/3306）、白名单IP列表（空/all/具体IP，支持CIDR）、操作类型（配置/取消配置）

**白名单模式**：
- 空 → 自动添加127.0.0.1，仅本机可访问，拒绝所有外部IP
- all → 允许所有IP访问，不配置拒绝规则
- 具体IP → 为每个IP创建允许规则，拒绝其他所有IP

**自动化能力**：检测发行版(apt/yum) → 停止冲突防火墙(firewalld/ufw) → 安装持久化服务 → 适配Docker环境(INPUT/DOCKER-USER链自动选择) → 管理ipset集合(创建/添加/删除/销毁) → 创建iptables规则(ACCEPT/LOG/DROP) → 规则持久化保存

---

## 执行规则（必须严格遵守）

1. **按序执行**：必须按 Step 1→2→3→4→5→6→7→8 顺序执行，不可跳步、不可逆序
2. **先判断后执行**：每个 Step 必须先执行判断命令，获取输出后再决定是否执行操作命令
3. **判断不通过则停止**：判断条件不满足时，立即停止并返回错误信息，不继续后续步骤
4. **逐条执行**：每个命令单独执行，获取输出后再决定下一步操作
5. **不猜测**：所有判断必须基于命令输出的实际内容，不可猜测或假设

## 判断输出解析模板

| 判断对象 | 执行命令 | "是"匹配模式 | "否"匹配模式 |
|---------|---------|------------|------------|
| 服务是否运行 | `systemctl is-active {service}` | 输出为 `active` | 输出为 `inactive`/`unknown`/其他 |
| 包是否已安装 | `apt list --installed 2>/dev/null \| grep {pkg}` 或 `rpm -q {pkg}` | 输出含包名 | 输出为空或 `not installed`/`未安装` |
| Docker是否运行 | `systemctl is-active docker` | 输出为 `active` | 输出非 `active` |
| 链是否可用 | `iptables -L {chain} -n 2>&1` | 输出含 `Chain {chain}` | 输出含 `No chain`/`不存在的链` |
| 端口是否Docker代理 | `ss -tlnp \| grep :{port}` | 输出含 `docker-proxy` | 输出不含 `docker-proxy` |
| ipset集合是否存在 | `ipset list {name} 2>&1` | 输出含 `Name: {name}` | 输出含 `does not exist`/`不存在` |
| IP是否在集合中 | `ipset test {name} {ip} 2>&1` | 输出为 `YES`/`is in set` | 输出为 `NO`/`is NOT in set` |
| iptables规则是否存在 | `iptables -C {chain} {rule} 2>&1` | exit code=0 无输出 | exit code≠0 含 `Bad rule`/`No chain` |
| 安装是否成功 | 安装命令输出 | 含 `installed`/`success`/`completed`/`已安装`/`newest` | 含 `error`/`failed`/`Unable`/`无法` |

---

## 配置端口访问控制

为指定端口设置访问控制，只有白名单中的 IP 可以访问该端口，其他 IP 全部拒绝。

- **白名单为空值时**：自动添加 127.0.0.1 到白名单（确保本机回环可访问），然后只配置拒绝规则，拒绝所有外部访问。
  - 例：配置端口 22，白名单为空 → 服务器本机 127.0.0.1 可以访问 22 端口，其他所有外部 IP 全部被拒绝，SSH 无法从外部连接。
- **白名单为 "all" 时**：允许所有 IP 访问，不配置任何拒绝规则。
  - 例：配置端口 22，白名单为 "all" → 任何 IP 都可以访问 22 端口，不做任何限制，等于不配置防火墙。
- **白名单有具体 IP 时**：为每个白名单 IP 创建允许规则，同时创建拒绝规则拦截非白名单访问。支持逗号分隔多 IP 和 CIDR 格式。
  - 例1：配置端口 22，白名单为 "192.168.1.100,10.0.0.5" → 只有这两个 IP 可以访问 22 端口，其他 IP 全部被拒绝。
  - 例2：配置端口 80，白名单为 "192.168.1.0/24" → 只有 192.168.1.0/24 网段的 IP 可以访问 80 端口。

## 取消端口访问控制

移除指定端口的访问控制，恢复端口为默认状态。

- **白名单为空值时**：完整删除该端口的所有访问控制规则（允许规则、拒绝规则、日志规则全部删除），并销毁对应的 ipset 集合。
  - 例：取消端口 22 的访问控制，白名单为空 → 删除所有 iptables 规则和 ipset 集合。
- **白名单有具体 IP 时**：只从 ipset 集合中删除指定的 IP，保留其余白名单 IP 和所有规则不变。
  - 例：取消端口 80 白名单 IP "10.0.0.5" → 从 ipset 中删除指定 IP。

---

## 配置流程（8步，每步必须先判断再执行）

### Step 1: 检测发行版

1.1 执行命令：`cat /etc/os-release`
1.2 判断输出：输出是否含 `ubuntu` 或 `kali`？
  - 是 → 记录：包管理器=apt，防火墙=ufw，iptables包=iptables-persistent，保存命令=netfilter-persistent save
  - 否 → 判断输出：是否含 `centos` 或 `redhat` 或 `rocky`？
    - 是 → 记录：包管理器=yum，防火墙=firewalld，iptables包=iptables-services，保存命令=service iptables save
    - 否 → 默认使用 yum 系配置
  - 继续 Step 2

### Step 2: 停止系统防火墙

2.1 执行命令：`systemctl is-active firewalld`
2.2 判断输出：输出是否为 `active`？（参照"判断输出解析模板"）
  - 是 → 依次执行：
    - `systemctl stop firewalld`
    - `systemctl disable firewalld`
  - 否 → 跳过

2.3 执行命令：`systemctl is-active ufw`
2.4 判断输出：输出是否为 `active`？
  - 是 → 依次执行：
    - `systemctl stop ufw`
    - `systemctl disable ufw`
  - 否 → 跳过
  - 继续 Step 3

### Step 3: 检查iptables持久化服务

3.1 执行命令：`systemctl list-unit-files | grep -E "iptables|netfilter-persistent"`
3.2 判断输出：iptables服务是否已安装？（输出是否含 `iptables` 或 `netfilter-persistent`）
  - 是 → 跳到 3.6
  - 否 → 继续安装流程 3.3

3.3 执行安装命令（根据Step 1的包管理器）：
  - apt系：`apt update && apt install -y iptables-persistent netfilter-persistent`
  - yum系：`yum install -y iptables-services iptables`

3.4 判断输出：安装是否成功？（输出是否含 `installed`/`success`/`completed`/`已安装`/`newest`？参照"判断输出解析模板"）
  - 否 → **停止**，返回错误："iptables 安装失败"
  - 是 → 继续 3.5

3.5 执行以下命令（仅首次安装）：
  - `systemctl start netfilter-persistent` 或 `systemctl start iptables`
  - `systemctl enable netfilter-persistent` 或 `systemctl enable iptables`
  - `iptables -F INPUT`
  - 跳到 Step 4

3.6 执行命令：`systemctl is-active netfilter-persistent` 或 `systemctl is-active iptables`
3.7 判断输出：服务是否已启动？（输出是否为 `active`？）
  - 是 → 跳过，继续 Step 4
  - 否 → 执行：
    - `systemctl start netfilter-persistent` 或 `systemctl start iptables`
    - `systemctl enable netfilter-persistent` 或 `systemctl enable iptables`
    - 继续 Step 4

### Step 4: 统一检查Docker环境

4.1 执行命令：`systemctl is-active docker`
4.2 判断输出：输出是否为 `active`？（参照"判断输出解析模板"）
  - 否 → 非Docker环境，记录：目标链=INPUT，跳到 Step 5
  - 是 → 继续 4.3

4.3 执行命令：`iptables -L DOCKER -n 2>&1`
4.4 判断输出：输出是否含 `Chain DOCKER`？
  - 是 → 继续 4.5
  - 否 → Docker链不可用，执行Docker修复流程：
    1. 执行 `docker ps --format '{{.Names}}'` → 记录为 restart_before 列表
    2. 执行 `systemctl restart docker`
    3. 执行 `sleep 3`
    4. 执行 `iptables -L DOCKER -n 2>&1` → 判断是否含 `Chain DOCKER`
       - 是 → 继续
       - 否 → 执行 `journalctl -u docker --no-pager -n 50` 查看日志 → **停止**，返回错误："Docker链不可用"
    5. 执行 `docker ps --format '{{.Names}}'` → 记录为 restart_after 列表
    6. 判断 restart_before 中有但 restart_after 中没有的容器 → 逐个执行 `docker start {容器名}`

4.5 执行命令：`ss -tlnp | grep :{port}`
4.6 判断输出：输出是否含 `docker-proxy`？
  - 是 → 记录：目标链=DOCKER-USER
  - 否 → 记录：目标链=INPUT
  - 继续 Step 5

### Step 5: 管理ipset白名单

5.0 判断白名单是否为空？
  - 空 → 自动添加 `127.0.0.1` 到白名单列表
  - 非空 → 使用传入的白名单
  - 白名单为 `all` → 跳过 Step 5-7，直接到 Step 8（允许所有IP，不配置拒绝规则）

5.1 执行命令：`ipset list {port}_allow_ip 2>&1`
5.2 判断输出：ipset集合是否存在？（输出是否含 `Name: {port}_allow_ip`？参照"判断输出解析模板"）
  - 否 → 执行：`ipset create {port}_allow_ip hash:net`
  - 是 → 跳过

5.3 对每个白名单IP，执行命令：`ipset test {port}_allow_ip {ip} 2>&1`
5.4 判断输出：IP是否已在集合中？（输出是否为 `YES`/`is in set`？）
  - 否 → 执行：`ipset add -exist {port}_allow_ip {ip}`
  - 是 → 跳过
  - 对所有IP重复 5.3-5.4

5.5 执行命令：`ipset save > /etc/ipset.conf`
5.6 继续 Step 6

### Step 6: 创建ipset自启动服务

6.1 执行命令：`systemctl is-enabled ipset-restore 2>/dev/null`
6.2 判断输出：服务是否已启用？（exit code=0）
  - 是 → 执行 `systemctl status ipset-restore.service` → 判断输出是否含 `failed`
    - 含 `failed` → 重新写入服务文件（见6.3-6.6）
    - 不含 `failed` → 跳过本步，继续 Step 7
  - 否 → 继续写入服务文件 6.3

6.3 执行命令写入服务文件：
```
cat > /etc/systemd/system/ipset-restore.service <<'EOF'
[Unit]
Description=Restore ipset rules on boot
Before=iptables.service

[Service]
Type=oneshot
ExecStart=/bin/sh -c '/sbin/ipset restore -exist < /etc/ipset.conf'

[Install]
WantedBy=multi-user.target
EOF
```

6.4 执行命令：`systemctl daemon-reload`
6.5 执行命令：`systemctl enable ipset-restore`
6.6 执行命令：`systemctl start ipset-restore`
6.7 继续 Step 7

### Step 7: 创建iptables规则

7.0 判断Step 4记录的目标链：
  - 如果白名单为 `all` → 跳到 Step 8

**目标链=INPUT：**

7.1 执行命令：`iptables -C INPUT -p tcp --dport {port} -m set --match-set {port}_allow_ip src -j ACCEPT 2>&1`
7.2 判断输出：规则是否存在？（exit code=0？参照"判断输出解析模板"）
  - 否 → 执行：`iptables -A INPUT -p tcp --dport {port} -m set --match-set {port}_allow_ip src -j ACCEPT`
  - 是 → 跳过

7.3 执行命令：`iptables -C INPUT -p tcp --dport {port} -j LOG --log-prefix "iptables-drop-{port}: " 2>&1`
7.4 判断输出：规则是否存在？
  - 否 → 执行：`iptables -A INPUT -p tcp --dport {port} -j LOG --log-prefix "iptables-drop-{port}: "`
  - 是 → 跳过

7.5 执行命令：`iptables -C INPUT -p tcp --dport {port} -j DROP 2>&1`
7.6 判断输出：规则是否存在？
  - 否 → 执行：`iptables -A INPUT -p tcp --dport {port} -j DROP`
  - 是 → 跳过
  - 跳到 Step 8

**目标链=DOCKER-USER：**

7.7 执行命令：`iptables -C DOCKER-USER -p tcp --dport {port} -m set --match-set {port}_allow_ip src -j ACCEPT 2>&1`
7.8 判断输出：规则是否存在？
  - 否 → 执行：`iptables -I DOCKER-USER 1 -p tcp --dport {port} -m set --match-set {port}_allow_ip src -j ACCEPT`
  - 是 → 跳过

7.9 执行命令：`iptables -C DOCKER-USER -p tcp --dport {port} -j LOG --log-prefix "iptables-drop-{port}: " 2>&1`
7.10 判断输出：规则是否存在？
  - 否 → 执行：`iptables -I DOCKER-USER 2 -p tcp --dport {port} -j LOG --log-prefix "iptables-drop-{port}: "`
  - 是 → 跳过

7.11 执行命令：`iptables -C DOCKER-USER -p tcp --dport {port} -j DROP 2>&1`
7.12 判断输出：规则是否存在？
  - 否 → 执行：`iptables -I DOCKER-USER 3 -p tcp --dport {port} -j DROP`
  - 是 → 跳过

### Step 8: 保存iptables规则

8.1 判断Step 1记录的包管理器：
  - apt系 → 执行命令：`netfilter-persistent save`
  - yum系 → 执行命令：`service iptables save`
8.2 判断输出：保存是否成功？（无错误信息）
  - 否 → **停止**，返回错误："iptables 规则保存失败"
  - 是 → 配置完成，返回成功

---

## 取消配置流程（7步，每步必须先判断再执行）

### Step 1-4: 与配置流程相同的环境验证

Step 1: 执行配置流程的 Step 1（检测发行版）
Step 2: 执行配置流程的 Step 2（检查防火墙状态，仅检查不禁用）
Step 3: 执行配置流程的 Step 3.1-3.2（检查iptables服务是否已安装）
  - 未安装 → **停止**，返回错误："iptables 未安装，无法取消配置"
  - 已安装 → 继续
Step 4: 执行配置流程的 Step 4（检查Docker环境，确定目标链）

### Step 5: 检查ipset集合

5.1 执行命令：`ipset list {port}_allow_ip 2>&1`
5.2 判断输出：ipset集合是否存在？（输出是否含 `Name: {port}_allow_ip`？参照"判断输出解析模板"）
  - 不存在 + 白名单为空 → **停止**，返回："ipset集合不存在，无需取消"
  - 不存在 + 白名单非空 → **停止**，返回错误："ipset集合不存在，无法删除白名单IP"
  - 已存在 → 继续 Step 6

### Step 6: 检查并删除规则

6.0 判断白名单是否为空？

**分支A：白名单非空（只删除指定IP）**

6A.1 对每个要删除的IP，执行命令：`ipset test {port}_allow_ip {ip} 2>&1`
6A.2 判断输出：IP是否在集合中？（输出是否为 `YES`/`is in set`？参照"判断输出解析模板"）
  - 是 → 执行：`ipset del {port}_allow_ip {ip}`
  - 否 → 跳过
  - 对所有IP重复 6A.1-6A.2

6A.3 执行命令：`ipset save > /etc/ipset.conf`
6A.4 跳到 Step 7

**分支B：白名单为空（完整删除）**

6B.1 执行命令：`iptables -C {chain} -p tcp --dport {port} -j DROP 2>&1`
6B.2 判断输出：DROP规则是否存在？（exit code=0？）
  - 是 → 执行：`iptables -D {chain} -p tcp --dport {port} -j DROP`
  - 否 → 跳过

6B.3 执行命令：`iptables -C {chain} -p tcp --dport {port} -j LOG --log-prefix "iptables-drop-{port}: " 2>&1`
6B.4 判断输出：LOG规则是否存在？
  - 是 → 执行：`iptables -D {chain} -p tcp --dport {port} -j LOG --log-prefix "iptables-drop-{port}: "`
  - 否 → 跳过

6B.5 执行命令：`iptables -C {chain} -p tcp --dport {port} -m set --match-set {port}_allow_ip src -j ACCEPT 2>&1`
6B.6 判断输出：ACCEPT规则是否存在？
  - 是 → 执行：`iptables -D {chain} -p tcp --dport {port} -m set --match-set {port}_allow_ip src -j ACCEPT`
  - 否 → 跳过

> 删除顺序必须 DROP→LOG→ACCEPT，先删ACCEPT会断开SSH连接。

6B.7 执行命令：`ipset list {port}_allow_ip 2>&1`
6B.8 判断输出：ipset集合是否存在？
  - 是 → 执行：`ipset destroy {port}_allow_ip`
  - 否 → 跳过

6B.9 判断Step 1记录的包管理器：
  - apt系 → 执行命令：`netfilter-persistent save`
  - yum系 → 执行命令：`service iptables save`

### Step 7: 输出取消结果

输出取消摘要：
  - 端口号: {port}
  - 操作: 已移除访问控制

---

## 发行版映射表

| 发行版                  | 包管理器 | 防火墙       | iptables包           | 保存命令                      |
| -------------------- | ---- | --------- | ------------------- | ------------------------- |
| Ubuntu/Kali          | apt  | ufw       | iptables-persistent | netfilter-persistent save |
| CentOS/RHEL/Rocky/其他 | yum  | firewalld | iptables-services   | service iptables save     |

---

## 铁律（不可违反）

1. **删除规则顺序**: 必须 DROP → LOG → ACCEPT，先删 ACCEPT 会断开 SSH
2. **首次安装才清空**: `iptables -F INPUT` 仅在首次安装 iptables 后执行，已安装场景不清空
3. **Docker环境必检链**: 只要 `systemctl is-active docker == active`，无论什么端口都必须检查 DOCKER/DOCKER-USER 链
4. **链不可用必重启**: Docker 链不可用时必须 `systemctl restart docker` → `sleep 3` → 再检查
5. **规则去重**: 添加前必须用 `iptables -C` 检查，ipset 添加前必须用 `ipset test` 检查
6. **ipset集合命名**: 固定格式 `{port}_allow_ip`，类型 `hash:net`
7. **白名单为空**: 自动添加 `127.0.0.1`，不能留空
8. **安装成功判断**: 必须用白名单关键词匹配输出，不能只看 exit code
