### 1. 创建主策略

```
netsh ipsec static add policy name="安全访问控制策略" description="安全访问控制策略"
```

### 2. 创建筛选器操作：允许 和 阻止

```
netsh ipsec static add filteraction name="允许访问" action=permit
netsh ipsec static add filteraction name="拒绝访问" action=block
```

### 3. 创建筛选器列表：用于白名单IP和拒绝所有（具体的端口号）

```
netsh ipsec static add filterlist name="允许3389访问"
netsh ipsec static add filterlist name="拒绝3389访问"
netsh ipsec static add filterlist name="允许3306访问"
netsh ipsec static add filterlist name="拒绝3306访问"
```

### 4. 向白名单列表添加规则（只允许 192.168.21.52具体的白名单）

```
netsh ipsec static add filter filterlist="允许3389访问" srcaddr=192.168.21.52 dstaddr=me dstport=3389 protocol=TCP
netsh ipsec static add filter filterlist="允许3389访问" srcaddr=192.168.21.52 dstaddr=me dstport=3389 protocol=UDP
netsh ipsec static add filter filterlist="允许3306访问" srcaddr=192.168.21.52 dstaddr=me dstport=3306 protocol=TCP
netsh ipsec static add filter filterlist="允许3306访问" srcaddr=192.168.21.52 dstaddr=me dstport=3306 protocol=UDP
```

### 5. 向拒绝列表添加规则（拒绝所有其他 IP）

```
netsh ipsec static add filter filterlist="拒绝3389访问" srcaddr=any dstaddr=me dstport=3389 protocol=TCP
netsh ipsec static add filter filterlist="拒绝3389访问" srcaddr=any dstaddr=me dstport=3389 protocol=UDP
netsh ipsec static add filter filterlist="拒绝3306访问" srcaddr=any dstaddr=me dstport=3306 protocol=TCP
netsh ipsec static add filter filterlist="拒绝3306访问" srcaddr=any dstaddr=me dstport=3306 protocol=UDP
```

### 6. 将规则添加到策略

```
netsh ipsec static add rule name="允许3389访问" policy="安全访问控制策略" filterlist="允许3389访问" filteraction="允许访问"
netsh ipsec static add rule name="拒绝3389访问" policy="安全访问控制策略" filterlist="拒绝3389访问" filteraction="拒绝访问"
netsh ipsec static add rule name="允许3306访问" policy="安全访问控制策略" filterlist="允许3306访问" filteraction="允许访问"
netsh ipsec static add rule name="拒绝3306访问" policy="安全访问控制策略" filterlist="拒绝3306访问" filteraction="拒绝访问"
```

### 7. 指派（激活）策略

```
netsh ipsec static set policy name="安全访问控制策略" assign=n
netsh ipsec static set policy name="安全访问控制策略" assign=y
```

### 8. 添加新IP到白名单

```
netsh ipsec static add filter filterlist="允许3306访问" srcaddr=192.168.21.50 dstaddr=me dstport=1433 protocol=TCP
netsh ipsec static add filter filterlist="允许3306访问" srcaddr=192.168.21.50 dstaddr=me dstport=1433 protocol=UDP
netsh ipsec static set policy name="安全访问控制策略" assign=n
netsh ipsec static set policy name="安全访问控制策略" assign=y
```

### 9. 主策略重命名

```
netsh ipsec static show policy all
netsh ipsec static set policy 综合远程访问安全策略  安全访问控制策略
```



### 10. 筛选器列表重命名