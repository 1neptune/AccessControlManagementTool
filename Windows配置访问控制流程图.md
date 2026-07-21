```mermaid
flowchart TD
    START([开始]) --> A[输入端口列表 + 白名单IPs]
    
    A --> B1[阶段一: 适用性检查]
    B1 --> B2[检查 secpol.msc 是否存在<br>os.Stat C:\Windows\System32\secpol.msc]
    B2 --> B3{secpol.msc 存在?}
    B3 -->|否| B4[❌ 系统不支持IPsec策略<br>返回错误: 系统为家庭版或组件缺失]
    B3 -->|是| B5[检查 netsh ipsec static 命令]
    B5 --> B6[执行: netsh ipsec static]
    B6 --> B7{命令执行成功?}
    B7 -->|否| B8[❌ 系统不支持 netsh ipsec static]
    B7 -->|是| B9[✅ 适用性检查通过]
    
    B9 --> C1[阶段二: 服务有效性检查]
    C1 --> C2[检查 PolicyAgent 服务]
    C2 --> C3[执行: sc query PolicyAgent]
    C3 --> C4{服务状态 = RUNNING?}
    C4 -->|否| C5[尝试启动 PolicyAgent]
    C5 --> C6[sc config PolicyAgent start= auto<br>net start PolicyAgent]
    C6 --> C7{启动成功?}
    C7 -->|否| C8[❌ 服务无法启动<br>返回错误]
    C7 -->|是| C9[✅ PolicyAgent 已启动]
    C4 -->|是| C9
    
    C9 --> C10[检查 IKEEXT 服务]
    C10 --> C11[执行: sc query IKEEXT]
    C11 --> C12{服务状态 = RUNNING?}
    C12 -->|否| C13[尝试启动 IKEEXT]
    C13 --> C14[sc config IKEEXT start= auto<br>net start IKEEXT]
    C14 --> C15{启动成功?}
    C15 -->|否| C16[❌ 服务无法启动<br>返回错误]
    C15 -->|是| C17[✅ IKEEXT 已启动]
    C12 -->|是| C17
    
    C17 --> D1[阶段三: 策略状态检查]
    D1 --> D2[检查策略是否存在]
    D2 --> D3[执行: netsh ipsec static show policy<br>name=安全访问控制策略]
    D3 --> D4{策略是否存在?}
    
    D4 -->|否| E1[阶段四: 首次配置]
    D4 -->|是| F1[阶段五: 策略验证]
    
    F1 --> F2[验证策略配置完整性]
    F2 --> F3[遍历端口列表, 检查每个端口<br>是否已创建允许/拒绝列表]
    F3 --> F4{配置完整?}
    F4 -->|否| G1[阶段六: 修复模式]
    F4 -->|是| H1[阶段七: 二次配置 / 追加IP]
    
    E1 --> E2[4.1 创建全局策略]
    E2 --> E3[add policy name=安全访问控制策略]
    E3 --> E4[4.2 创建全局筛选器操作]
    E4 --> E5[add filteraction 允许访问 action=permit]
    E5 --> E6[add filteraction 拒绝访问 action=block]
    E6 --> E7[4.3 按端口循环创建列表、筛选器、绑定规则]
    E7 --> E8[遍历每个端口]
    E8 --> E9[创建白名单列表: 允许{port}访问]
    E9 --> E10[创建黑名单列表: 拒绝{port}访问]
    E10 --> E11[添加白名单筛选器<br>srcaddr=白名单IP dstaddr=me dstport={port}]
    E11 --> E12[添加黑名单筛选器<br>srcaddr=any dstaddr=me dstport={port}]
    E12 --> E13[绑定允许规则<br>add rule name=允许{port}访问]
    E13 --> E14[绑定拒绝规则<br>add rule name=拒绝{port}访问]
    E14 --> E15{所有端口处理完?}
    E15 -->|否| E8
    E15 -->|是| E16[4.4 指派策略 刷新]
    E16 --> E17[set policy assign=n]
    E17 --> E18[set policy assign=y]
    E18 --> E19[✅ 首次配置完成]
    
    G1 --> G2[6.1 删除现有策略]
    G2 --> G3[delete policy name=安全访问控制策略]
    G3 --> G4[6.2 重新执行首次配置]
    G4 --> G5[跳转到 4.1]
    G5 --> E19
    
    H1 --> H2[7.1 获取现有白名单IP]
    H2 --> H3[遍历每个端口]
    H3 --> H4[检查现有IP列表]
    H4 --> H5[对比输入IP, 找出需要新增的IP]
    H5 --> H6{有新增IP?}
    H6 -->|否| H7[该端口无变化, 跳过]
    H6 -->|是| H8[7.2 追加新IP筛选器]
    H8 --> H9[add filter srcaddr=新IP]
    H9 --> H10{所有端口处理完?}
    H10 -->|否| H3
    H10 -->|是| H11[7.3 刷新策略]
    H11 --> H12[set policy assign=n]
    H12 --> H13[set policy assign=y]
    H13 --> H14[✅ 二次配置完成]
    
    E19 --> J1[阶段八: 验证配置]
    H14 --> J1
    J1 --> J2[验证策略状态]
    J2 --> J3[执行: netsh ipsec static show policy<br>name=安全访问控制策略 level=verbose]
    J3 --> J4{Assigned = YES?}
    J4 -->|否| J5[⚠️ 策略未生效, 检查日志]
    J4 -->|是| J6[执行验证检查]
    J6 --> J7[Get-NetIPsecRule -PolicyStore ActiveStore]
    J7 --> J8{有规则返回?}
    J8 -->|否| J9[⚠️ 策略可能未生效]
    J8 -->|是| J10[✅ 配置验证通过]
    
    J5 --> END([结束])
    J9 --> END
    J10 --> END
    B4 --> END
    B8 --> END
    C8 --> END
    C16 --> END
    
    style START fill:#e1f5fe
    style B9 fill:#c8e6c9
    style C17 fill:#c8e6c9
    style E19 fill:#c8e6c9
    style H14 fill:#c8e6c9
    style J10 fill:#c8e6c9
    style B4 fill:#ffcdd2
    style B8 fill:#ffcdd2
    style C8 fill:#ffcdd2
    style C16 fill:#ffcdd2
    style J5 fill:#fff9c4
    style J9 fill:#fff9c4
```