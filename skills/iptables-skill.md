# iptables Skill - Linux 防火墙访问控制

---

## 功能说明

在 Linux 服务器上配置基于 iptables + ipset 的端口访问控制。通过 SSH 远程执行命令，实现：自动检测发行版、安装依赖、适配 Docker 环境、管理 IP 白名单、创建防火墙规则。

---

## 配置端口访问控制

为指定端口设置访问控制，只有白名单中的 IP 可以访问该端口，其他 IP 全部拒绝。

- **白名单为空值时**：自动添加 127.0.0.1 到白名单（确保本机回环可访问），然后只配置拒绝规则，拒绝所有外部访问。
  - 例：配置端口 22，白名单为空 → 服务器本机 127.0.0.1 可以访问 22 端口，其他所有外部 IP 全部被拒绝，SSH 无法从外部连接。
- **白名单为 "all" 时**：允许所有 IP 访问，不配置任何拒绝规则。
  - 例：配置端口 22，白名单为 "all" → 任何 IP 都可以访问 22 端口，不做任何限制，等于不配置防火墙。
- **白名单有具体 IP 时**：为每个白名单 IP 创建允许规则，同时创建拒绝规则拦截非白名单访问。支持逗号分隔多 IP 和 CIDR 格式。
  - 例1：配置端口 22，白名单为 "192.168.1.100,10.0.0.5" → 只有这两个 IP 可以访问 22 端口，其他 IP 全部被拒绝。
  - 例2：配置端口 8080，白名单为 "192.168.1.0/24" → 192.168.1.0~192.168.1.255 网段内的所有 IP 可以访问 8080 端口，其他 IP 全部被拒绝。

## 取消端口访问控制

移除指定端口的访问控制，恢复端口为默认状态。

- **白名单为空值时**：完整删除该端口的所有访问控制规则（允许规则、拒绝规则、日志规则全部删除），并销毁对应的 ipset 集合。
  - 例：取消端口 22 的访问控制，白名单为空 → 删除端口 22 的所有 iptables 规则（ACCEPT、LOG、DROP 全部删除），删除 ipset 集合 22_allow_ip，端口 22 恢复为无限制状态，任何 IP 都可以访问。
- **白名单有具体 IP 时**：只从 ipset 集合中删除指定的 IP，保留其余白名单 IP 和所有规则不变。
  - 例1：取消端口 22 的白名单 IP "10.0.0.5"，原白名单为 "192.168.1.100,10.0.0.5" → 只从 ipset 集合 22_allow_ip 中删除 10.0.0.5，保留 192.168.1.100，规则不变，最终只有 192.168.1.100 可以访问 22 端口。
  - 例2：取消端口 22 的白名单 IP "10.0.0.5"，但该 IP 不在集合中 → 跳过删除，不影响现有配置。

---

## 配置流程（8步，每步必须先判断再执行）

### Step 1: 检测发行版

```
判断: cat /etc/os-release 输出包含哪个关键词？
  ubuntu/kali → apt + ufw + iptables-persistent + netfilter-persistent
  centos/redhat/rocky/其他 → yum + firewalld + iptables-services + iptables
```

### Step 2: 停止系统防火墙

```
判断: 系统防火墙是否正在运行？(systemctl is-active)
  firewalld == active → 执行 systemctl stop/disable firewalld
  ufw == active       → 执行 systemctl stop/disable ufw
  都未运行            → 跳过
```

### Step 3: 检查iptables持久化服务

```
判断1: iptables服务是否已安装？(systemctl list-unit-files | grep)
  未安装 → 安装(apt/yum install) → 判断2: 安装是否成功？
            成功关键词: already installed/installed/success/completed/已安装/is already the newest/0 upgraded/no packages to install
            失败 → 返回错误
          → 启动并启用(systemctl start/enable)
          → 清空INPUT链(iptables -F INPUT)  ← 仅首次安装才清空
  已安装 → 判断2: 服务是否已启动？(systemctl is-active)
            未启动 → 启动并启用(systemctl start/enable)
            已启动 → 跳过
```

### Step 4: 统一检查Docker环境

```
判断1: Docker服务是否在运行？(systemctl is-active docker == "active")
  否 → 非Docker环境，规则加到INPUT链，跳过本步
  是 → 判断2: DOCKER和DOCKER-USER链是否可用？(iptables -L DOCKER -n / iptables -L DOCKER-USER -n)
        输出为空或包含 "No chain/target/match by that name" = 不可用
        可用   → 继续
        不可用 → 执行Docker修复流程（见下方）

Docker修复流程（链不可用时执行）:
  1. 记录当前运行的容器列表
     docker ps --format '{{.Names}}'  → 保存为 restart_before 列表
  2. 重启Docker服务
     systemctl restart docker
  3. 等待Docker恢复
     sleep 3
  4. 判断3: DOCKER和DOCKER-USER链是否重建成功？(iptables -L DOCKER -n / iptables -L DOCKER-USER -n)
     成功 → 继续
     失败 → 执行Docker故障排查流程（见下方）
  5. 判断4: 是否有容器未恢复运行？
     docker ps --format '{{.Names}}'  → 保存为 restart_after 列表
     restart_before 中存在但 restart_after 中不存在的容器 = 缺失的容器
     有缺失 → 逐个重新启动: docker start {容器名}
     无缺失 → 继续

Docker故障排查流程（重启后链仍不可用时执行）:
  1. 查看Docker服务日志定位错误
     journalctl -u docker --no-pager -n 50
  2. 根据日志排查常见问题:
     - 配置文件错误(/etc/docker/daemon.json) → 修正后重试
     - 防火墙冲突 → 检查iptables规则是否阻止docker通信
     - 存储驱动问题 → 检查/var/lib/docker状态
     - 端口占用 → 检查是否有其他进程占用docker所需端口
  3. 修复后再次重启: systemctl restart docker → sleep 3
  4. 再次判断链是否可用 → 重复直到链正常
  5. 链正常后 → 重新检查并启动缺失的容器

判断5: 目标端口是否由docker-proxy监听？(ss -tlnp | grep :{port} 含 docker-prox)
  是 → 规则加到DOCKER-USER链
  否 → 规则加到INPUT链
```

### Step 5: 管理ipset白名单

```
判断0: 白名单是否为空？
  是 → 自动添加 127.0.0.1
  否 → 使用传入的白名单

判断1: ipset集合 {port}_allow_ip 是否存在？(ipset list)
  不存在 → 创建: ipset create {port}_allow_ip hash:net
  已存在 → 跳过

判断2: 逐个IP是否已在集合中？(ipset test)
  不在 → 添加: ipset add -exist {port}_allow_ip {ip}
  已在 → 跳过

执行: ipset save > /etc/ipset.conf
```

### Step 6: 创建ipset自启动服务

```
判断1: ipset-restore服务是否已启用？(systemctl is-enabled ipset-restore 2>/dev/null)
  已启用 → 判断2: 服务状态是否为 failed？(systemctl status ipset-restore.service)
            非 failed（active/inactive/其他）→ 跳过（服务正常，不用重新启动）
            failed → 重新写入服务文件 → daemon-reload → enable → restart
  未启用 → 写入服务文件 → daemon-reload → enable → start

服务文件固定内容（写入 /etc/systemd/system/ipset-restore.service）:
────────────────────────────────────────
[Unit]
Description=Restore ipset rules on boot
Before=iptables.service

[Service]
Type=oneshot
ExecStart=/bin/sh -c '/sbin/ipset restore -exist < /etc/ipset.conf'

[Install]
WantedBy=multi-user.target
────────────────────────────────────────

写入命令: cat > /etc/systemd/system/ipset-restore.service <<'EOF'
         {服务文件内容}
         EOF

后续命令（按顺序执行）:
  systemctl daemon-reload
  systemctl enable ipset-restore
  systemctl start ipset-restore
```

### Step 7: 创建iptables规则

```
判断: 规则加到哪个链？（Step 4 的结果）
  INPUT链（普通端口）:
    判断每条规则是否存在(iptables -C) → 不存在才添加:
    iptables -A INPUT -p tcp --dport {port} -m set --match-set {port}_allow_ip src -j ACCEPT
    iptables -A INPUT -p tcp --dport {port} -j LOG --log-prefix "iptables-drop-{port}: "
    iptables -A INPUT -p tcp --dport {port} -j DROP

  DOCKER-USER链（Docker端口）:
    判断每条规则是否存在(iptables -C) → 不存在才添加:
    iptables -I DOCKER-USER 1 -p tcp --dport {port} -m set --match-set {port}_allow_ip src -j ACCEPT
    iptables -I DOCKER-USER 2 -p tcp --dport {port} -j LOG --log-prefix "iptables-drop-{port}: "
    iptables -I DOCKER-USER 3 -p tcp --dport {port} -j DROP
```

### Step 8: 保存iptables规则

```
执行: apt系 → netfilter-persistent save
      yum系 → service iptables save
```

---

## 取消配置流程（7步，每步必须先判断再执行）

### Step 1-4: 与配置流程相同的环境验证

```
Step 1: 检测发行版（同配置）
Step 2: 检查防火墙状态（同配置，仅检查不禁用）
Step 3: 检查iptables服务
        判断: 未安装 → 返回错误（无法取消配置）
Step 4: 统一检查Docker环境（同配置）
```

### Step 5: 检查ipset集合

```
判断: ipset集合 {port}_allow_ip 是否存在？(ipset list)
  不存在 + 无白名单 → 直接返回（无需取消）
  不存在 + 有白名单 → 返回错误（无法删除不存在的白名单）
  已存在            → 继续
```

### Step 6: 显示Docker环境检测结果

```
判断: Step 4 结果
  Docker端口 → 规则在 DOCKER-USER 链
  普通端口   → 规则在 INPUT 链
```

### Step 7: 检查并删除规则

```
判断1: 白名单是否为空？

  非空（只删除指定IP）:
    逐个IP判断: ipset test {port}_allow_ip {ip}
      在集合中   → ipset del {port}_allow_ip {ip}
      不在集合中 → 跳过
    → ipset save > /etc/ipset.conf

  空（完整删除）:
    判断2: 链规则是否存在？(iptables -C)
      不存在 → 跳过删除
      存在   → 按顺序删除（必须 DROP→LOG→ACCEPT，避免SSH断连）:
        iptables -D {chain} -p tcp --dport {port} -j DROP
        iptables -D {chain} -p tcp --dport {port} -j LOG --log-prefix "iptables-drop-{port}: "
        iptables -D {chain} -p tcp --dport {port} -m set --match-set {port}_allow_ip src -j ACCEPT
    → 判断3: ipset集合是否存在？→ ipset destroy {port}_allow_ip
    → 保存iptables规则
```

---

## 发行版映射表

| 发行版 | 包管理器 | 防火墙 | iptables包 | 服务名 | 保存命令 |
|--------|---------|--------|-----------|--------|---------|
| Ubuntu/Kali | apt | ufw | iptables-persistent | netfilter-persistent | netfilter-persistent save |
| CentOS/RHEL/Rocky/其他 | yum | firewalld | iptables-services | iptables | service iptables save |

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
