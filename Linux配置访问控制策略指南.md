# Linux 防火墙安全配置指南

## iptables & firewalld 双方案实践

## 核心说明

- 宿主机端口（SSH/Java/Nginx）→ 使用 `INPUT` 链
- Docker `-p` 映射端口 → 必须使用 `DOCKER-USER` 链
- firewalld 与 iptables **二选一**，不可同时启用

---

# 1. firewalld 方案（测试/单机推荐）

**适合：开发环境、单机、简单业务**  
**不推荐：K8s、Docker 集群**

## 1.1 基础初始化

```bash
# 关闭 iptables
systemctl stop iptables
systemctl disable iptables

# 启动 firewalld
systemctl start firewalld
systemctl enable firewalld

# 基础端口放行
firewall-cmd --zone=public --add-port=46422/tcp --permanent
firewall-cmd --zone=public --add-port=8080/tcp --permanent

# 开启拒绝日志（审计）
firewall-cmd --set-log-denied=all
firewall-cmd --runtime-to-permanent
firewall-cmd --reload
```

## 1.2 IP 白名单管理

```bash
# 创建 IP 地址集
firewall-cmd --permanent --new-ipset=ssh_allow_ip --type=hash:ip --option=family=inet

# 添加白名单
firewall-cmd --permanent --ipset=ssh_allow_ip --add-entry=127.0.0.1
firewall-cmd --permanent --ipset=ssh_allow_ip --add-entry=192.168.6.10
firewall-cmd --permanent --ipset=ssh_allow_ip --add-entry=192.168.30.254
firewall-cmd --permanent --ipset=ssh_allow_ip --add-entry=192.168.21.0/24

# 删除白名单
firewall-cmd --permanent --ipset=ssh_allow_ip --remove-entry=192.168.21.239

# 查看 IP 集
firewall-cmd --permanent --get-ipsets
cat /etc/firewalld/ipsets/ssh_allow_ip.xml
```

## 1.3 端口访问控制

```bash
# 白名单访问 46422
firewall-cmd --permanent --add-rich-rule="rule family=\"ipv4\" source ipset=\"ssh_allow_ip\" port protocol=\"tcp\" port=\"46422\" accept"

# 清理默认开放规则（安全加固）
firewall-cmd --permanent --zone=public --remove-service=ssh
firewall-cmd --permanent --zone=public --remove-port=46422/tcp

# 允许 Ping
firewall-cmd --permanent --add-icmp-block-inversion
firewall-cmd --permanent --remove-icmp-block=echo-request

# 重载生效
firewall-cmd --reload
```

## 1.4 Docker 兼容配置

```bash
# 关闭 单机 Docker iptables 接管
# 注意：K8s / Docker 集群 禁止关闭 
echo '{
  "iptables": false
}' > /etc/docker/daemon.json

systemctl daemon-reload
systemctl restart docker
```

## 1.5 查看与日志

```bash
firewall-cmd --permanent --list-rich-rules
firewall-cmd --permanent --list-all

# 实时查看拦截日志
ssh root@172.19.10.152 -p 46422
journalctl -f -k | grep -E 'DPT=(46422|80|8080|1521|3306|6379)'
```

## 1.6 日常维护命令

```bash
# 重新加载规则
firewall-cmd --reload

# 查看当前运行配置
firewall-cmd --list-all
firewall-cmd --list-ports
firewall-cmd --list-rich-rules

# 追加白名单（永久）
firewall-cmd --permanent --ipset=ssh_allow_ip --add-entry=192.168.21.242
firewall-cmd --reload

# 删除白名单（永久）
firewall-cmd --permanent --ipset=ssh_allow_ip --remove-entry=192.168.21.242
firewall-cmd --reload

# 新增端口白名单（富规则）
firewall-cmd --permanent --add-rich-rule="rule family=\"ipv4\" source ipset=\"oracle_allow_ip\" port protocol=\"tcp\" port=\"1521\" accept"
firewall-cmd --reload


# 删除端口白名单（富规则）
firewall-cmd --permanent --remove-rich-rule="rule family=\"ipv4\" source ipset=\"oracle_allow_ip\" port protocol=\"tcp\" port=\"1521\" accept"
firewall-cmd --reload

# 删除端口放行
firewall-cmd --permanent --remove-port=1521/tcp
firewall-cmd --reload

# 查看防火墙状态
firewall-cmd --state
systemctl status firewalld
```

---

# 2. iptables 方案（生产推荐，Docker/K8s 友好）

✅ 支持：单机 / Docker / K8s / 集群 / 分布式
✅ 官方推荐：K8s 网络底层基于 iptables
✅ 不会与 kube-proxy、CNI、Service 冲突

## 2.1 环境初始化

```bash
# 关闭 firewalld
systemctl stop firewalld
systemctl disable firewalld

# 安装 iptables 服务
yum install -y iptables-services

# 启动 iptables
systemctl start iptables
systemctl enable iptables

# 初始化iptables
systemctl staus docker
iptables -F INPUT
iptables -F FORWARD
systemctl restart docker
iptables -F DOCKER-USER
iptables -L INPUT -n
iptables -L FORWARD -n
iptables -L DOCKER -n
iptables -L DOCKER-USER-n
```

## 2.2 宿主机端口控制（INPUT 链）

```bash
ipset create clickhouse_allow_ip hash:net
ipset add -exist clickhouse_allow_ip 192.168.21.0/24
ipset add -exist clickhouse_allow_ip 172.19.9.0/24# 清空规则
iptables -F INPUT

Chain INPUT (policy ACCEPT)


# 创建 SSH 白名单 运维
ipset create -exist ssh_allow_ip hash:ip

ipset create 46422_allow_ip hash:net
ipset list 46422_allow_ip
ipset add -exist ssh_allow_ip 127.0.0.1
ipset add -exist ssh_allow_ip 172.19.10.152
ipset add -exist ssh_allow_ip 192.168.6.10
ipset add -exist ssh_allow_ip 192.168.30.254
ipset add -exist ssh_allow_ip 192.168.21.52
ipset add -exist ssh_allow_ip 172.19.9.13


# 限制 SSH 46422
# 允许白名单
iptables -A INPUT -p tcp --dport 46422 -m set --match-set ssh_allow_ip src -j ACCEPT
# 记录拦截日志
iptables -A INPUT -p tcp --dport 46422 -j LOG --log-prefix "iptables-drop-22: "
# 拦截其他IP
iptables -A INPUT -p tcp --dport 46422 -j DROP

# 创建 nginx 白名单 业务
ipset create nginx_allow_ip hash:net
ipset add -exist nginx_allow_ip 192.168.21.0/24
# 放行白名单 IP（来自 nginx_allow_ip 集合的源地址）
iptables -A INPUT -p tcp --dport 80 -m set --match-set nginx_allow_ip src -j ACCEPT
# 可选：记录被拒绝的访问（便于调试或安全分析）
iptables -A INPUT -p tcp --dport 80 -j LOG --log-prefix "iptables-drop-80: "
# 拒绝其他所有 IP 访问 80 端口
iptables -A INPUT -p tcp --dport 80 -j DROP

# 创建 web 白名单 业务
ipset create web_allow_ip hash:net
ipset add -exist web_allow_ip 192.168.21.0/24
ipset add -exist web_allow_ip 172.19.9.0/24
# 放行白名单 IP（来自 web_allow_ip 集合的源地址）
iptables -A INPUT -p tcp --dport 8100 -m set --match-set web_allow_ip src -j ACCEPT
# 可选：记录被拒绝的访问（便于调试或安全分析）
iptables -A INPUT -p tcp --dport 8100 -j LOG --log-prefix "iptables-drop-80: "
# 拒绝其他所有 IP 访问 80 端口
iptables -A INPUT -p tcp --dport 8100 -j DROP


iptables -D INPUT -p tcp --dport 8100 -m set --match-set web_allow_ip src -j ACCEPT
iptables -D INPUT -p tcp --dport 8100 -j LOG --log-prefix "iptables-drop-8100: "
iptables -D INPUT -p tcp --dport 8100 -j DROP

service iptables save
systemctl restart iptables

核心原则：只要目标 IP 地址不是本机（或 127.0.0.1）的 IP，数据包就必须经过 FORWARD 链

iptables -L DOCKER
iptables: No chain/target/match by that name.

外网IP → INPUT → FORWARD → DOCKER链 → 容器


# 放行白名单 IP 访问 8100（Docker 必须用 FORWARD）
iptables -I FORWARD 1 -p tcp --dport 8100 -m set --match-set web_allow_ip src -j ACCEPT

# 记录被拒绝的访问
iptables -I FORWARD 2 -p tcp --dport 8100 -j LOG --log-prefix "iptables-drop-8100: "

# 拒绝其他所有 IP 访问 8100 端口
iptables -I FORWARD 3 -p tcp --dport 8100 -j DROP

service iptables save
systemctl restart iptables

iptables -L FORWARD -n
dmesg -wT | grep -E "iptables-drop"



[root@clickhouse clickhouse]# iptables -L DOCKER -n
Chain DOCKER (2 references)
target     prot opt source               destination         
ACCEPT     tcp  --  0.0.0.0/0            192.168.2.2          tcp dpt:9000
ACCEPT     tcp  --  0.0.0.0/0            192.168.2.2          tcp dpt:8123


ipset create clickhouse_allow_ip hash:net
ipset add -exist clickhouse_allow_ip 192.168.21.0/24
ipset add -exist clickhouse_allow_ip 172.19.9.0/24
ipset add clickhouse_allow_ip 192.168.21.52
ipset add clickhouse_allow_ip 192.168.3.45
ipset add clickhouse_allow_ip 192.168.30.254



ipset list clickhouse_allow_ip

ipset del clickhouse_allow_ip 192.168.21.52


iptables -I DOCKER-USER 1 -p tcp --dport 8123 -m set --match-set clickhouse_allow_ip src -j ACCEPT
iptables -I DOCKER-USER 2 -p tcp --dport 9000 -m set --match-set clickhouse_allow_ip src -j ACCEPT
iptables -I DOCKER-USER 3 -p tcp --dport 8123 -j LOG --log-prefix "iptables-drop-8123: " 
iptables -I DOCKER-USER 4 -p tcp --dport 9000 -j LOG --log-prefix "iptables-drop-9000: " 
iptables -I DOCKER-USER 5 -p tcp --dport 8123 -j DROP
iptables -I DOCKER-USER 6 -p tcp --dport 9000 -j DROP
iptables -L DOCKER-USER -n --line-numbers

ipset save > /etc/ipset.conf

service iptables save

systemctl restart iptables
```

## 2.3 Docker 映射端口控制（DOCKER-USER 链）

```bash
# 清空并默认放行
iptables -F DOCKER-USER
iptables -A DOCKER-USER -j ACCEPT

# Oracle 白名单
ipset create -exist oracle_allow_ip hash:ip
ipset add -exist oracle_allow_ip 127.0.0.1
ipset add -exist oracle_allow_ip 192.168.30.254
ipset add -exist oracle_allow_ip 192.168.21.52
ipset add -exist oracle_allow_ip 172.19.10.152
ipset add -exist oracle_allow_ip 172.19.10.153
ipset add -exist oracle_allow_ip 172.19.10.154
ipset add -exist oracle_allow_ip 172.19.10.155
ipset add -exist oracle_allow_ip 172.19.10.156

# 限制 1521 端口（必须插入到最前）
# 允许白名单
iptables -I DOCKER-USER 1 -p tcp --dport 1521 -m set --match-set oracle_allow_ip src -j ACCEPT
# 记录拦截日志
iptables -I DOCKER-USER 2 -p tcp --dport 1521 -j LOG --log-prefix "iptables-drop-oracle: "
# 拦截其他IP
iptables -I DOCKER-USER 3 -p tcp --dport 1521 -j DROP
```

## 2.4 永久保存配置

```bash
ipset restore < /etc/ipset.conf

systemctl restart iptables

iptables -L INPUT -n

ipset list ssh_allow_ip

ipset del ssh_allow_ip 192.168.30.254

# 保存 ipset
ipset save > /etc/ipset.conf

# 开机自动恢复 IP 集
sed -i '/ipset restore/d' /etc/rc.local


cat > /etc/systemd/system/ipset-restore.service <<EOF
[Unit]
Description=Restore ipset rules on boot
Before=iptables.service

[Service]
Type=oneshot
ExecStart=/bin/sh -c '/sbin/ipset restore -exist < /etc/ipset.conf'

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable ipset-restore
systemctl start ipset-restore

# 保存 iptables 规则
service iptables save
systemctl restart iptables
```

# 2.5 查看与拦截日志

```bash
# 查看规则
iptables -L INPUT -n
iptables -L DOCKER-USER -n --line-numbers

# 查看白名单
ipset list ssh_allow_ip
ipset list oracle_allow_ip

# 实时查看 iptables 拦截日志
dmesg -wT | grep -E "iptables-drop"

# 过滤端口拦截日志
journalctl -f -k | grep -i "iptables-drop"
```

## 2.6 日常维护命令

```bash
# 新增白名单 IP（立即生效 + 永久保存）
ipset add ssh_allow_ip 192.168.21.242

# 删除单个IP
ipset del ssh_allow_ip 192.168.30.254

# 删除集合内所有 IP，白名单变空
ipset flush ssh_allow_ip

# 查看集合内容
ipset list ssh_allow_ip

# 保存配置
ipset add ssh_allow_ip 172.19.10.70
ipset save > /etc/ipset.conf



# 根据行号删除
# 本机端口 → INPUT
# Docker 端口 → DOCKER-USER
# 删除规则 → iptables -D 链名 行号
# INPUT 链
iptables -D INPUT 2
#  DOCKER-USER 链
iptables -D DOCKER-USER 2 

# 保存规则
service iptables save

# 重启服务
systemctl restart iptables

# 清空规则（调试用）
iptables -F INPUT
iptables -F DOCKER-USER
```

# 3. 关键规则区别

## INPUT（宿主机端口）

- 默认策略：ACCEPT（全部放行）
- 只需限制敏感端口
- 命令：iptables -A INPUT ...

## DOCKER-USER（Docker 映射端口）

- 最后一行必须为 ACCEPT
- 限制规则必须用 -I 插入顶部
- 命令：iptables -I DOCKER-USER 1 ...

# 4. 适用场景（重要）

- 测试 / 单机 / 简单业务 → firewalld
- 生产 / Docker/K8s / 集群 / 分布式 → iptables（推荐）

# 5. K8s 集群说明（必读）

- iptables 是 K8s 网络底层核心依赖，完全兼容
- DOCKER-USER 链不会影响 kube-proxy/Service/CNI
- K8s 集群必须关闭 firewalld，使用 iptables
- 禁止关闭 Docker iptables 接管