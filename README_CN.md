# Drip - 快速内网穿透工具

自建隧道服务，让本地服务安全地暴露到公网。

[English](README.md)

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-BSD--3--Clause-blue.svg)](LICENSE)
[![TLS](https://img.shields.io/badge/TLS-1.3-green.svg)](https://tools.ietf.org/html/rfc8446)

## 为什么用它？

**数据掌控。** 没有第三方服务器，流量只在你的客户端和服务器之间传输。

**没有限制。** 想开多少隧道就开多少，带宽只受服务器性能限制。

**真正免费。** 用自己的域名，没有付费功能，没有阉割版。

| 特性 | Drip | ngrok 免费版 |
|------|------|-------------|
| 隐私 | 自己的基础设施 | 第三方服务器 |
| 域名 | 你的域名 | 1 个固定子域名 |
| 带宽 | 无限制 | 1 GB/月 |
| 活跃端点 | 无限制 | 1 个端点 |
| 每个代理的隧道数 | 无限制 | 最多 3 个 |
| 请求数 | 无限制 | 20,000 次/月 |
| 警告页面 | 无 | 有（可用请求头移除） |
| 开源 | ✓ | ✗ |

## 一键安装

### 客户端 (macOS/Linux)

```bash
bash <(curl -sL https://raw.githubusercontent.com/Gouryella/drip/main/scripts/install.sh)
```

### 服务端 (Linux)

```bash
bash <(curl -sL https://raw.githubusercontent.com/Gouryella/drip/main/scripts/install-server.sh)
```

## 使用方法

### 基础隧道

```bash
# 暴露本地 HTTP 服务
drip http 3000

# 暴露本地 HTTPS 服务
drip https 443

# 自定义子域名
drip http 3000 --subdomain myapp
# → https://myapp.你的域名.com

# 暴露 TCP 服务（数据库、SSH 等）
drip tcp 5432
```

### 转发到任意地址

不只是 localhost，可以转发到局域网内的任何设备：

```bash
# 转发到局域网其他设备
drip http 8080 --address 192.168.1.100

# 转发到 Docker 容器
drip http 3000 --address 172.17.0.2

# 转发到特定网卡
drip http 3000 --address 10.0.0.5
```

### 后台守护进程

让隧道在后台持续运行：

```bash
# 启动后台隧道
drip daemon start http 3000
drip daemon start https 8443 --subdomain api

# 管理后台隧道
drip daemon list
drip daemon stop http-3000
drip daemon logs http-3000
```

## 服务端部署

### 前置条件

- 域名已解析到服务器（A 记录）
- 泛域名解析：`*.tunnel.example.com -> 服务器IP`
- SSL 证书（推荐通配符证书）

### 方式一：直接部署（推荐）

Drip 服务端直接监听 443 端口处理 TLS：

```bash
# 获取通配符证书
sudo certbot certonly --manual --preferred-challenges dns \
  -d "*.tunnel.example.com" -d "tunnel.example.com"

# 启动服务
drip-server \
  --port 443 \
  --domain tunnel.example.com \
  --tls-cert /etc/letsencrypt/live/tunnel.example.com/fullchain.pem \
  --tls-key /etc/letsencrypt/live/tunnel.example.com/privkey.pem \
  --token 你的密钥
```

### 方式二：Nginx 反向代理

Drip 监听 8443 端口，Nginx 处理 SSL：

```nginx
server {
    listen 443 ssl http2;
    server_name *.tunnel.example.com;

    ssl_certificate /etc/letsencrypt/live/tunnel.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/tunnel.example.com/privkey.pem;

    location / {
        proxy_pass https://127.0.0.1:8443;
        proxy_ssl_verify off;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_buffering off;
    }
}
```

### Systemd 服务

安装脚本会自动创建 `/etc/systemd/system/drip-server.service`，使用以下命令管理：

```bash
sudo systemctl start drip-server    # 启动
sudo systemctl enable drip-server   # 开机启动
sudo journalctl -u drip-server -f   # 查看日志
```

## 核心特性

**安全性**
- 所有连接使用 TLS 1.3 加密
- 基于 Token 的认证
- 不支持旧版不安全协议

**灵活性**
- 支持 HTTP、HTTPS 和 TCP 隧道
- 转发到 localhost 或局域网任意地址
- 自定义子域名或自动生成
- 后台守护进程模式

**性能**
- 二进制协议配合 msgpack 编码
- 连接池复用
- 客户端与服务器之间开销极小

**简单易用**
- 一行命令安装
- 配置一次处处使用
- 实时连接统计

## 架构

```
┌─────────────┐         ┌──────────────┐         ┌─────────────┐
│   外部用户   │ ──────> │     服务器    │ <──────  │   你的电脑   │
│             │  HTTPS  │    (Drip)    │ TLS 1.3 │  localhost  │
└─────────────┘         └──────────────┘         └─────────────┘
```

## 常见使用场景

**开发和测试**
```bash
# 给客户演示本地开发的网站
drip http 3000

# 测试第三方 webhook（如 Stripe）
drip http 8000 --subdomain webhooks
```

**家庭服务器**
```bash
# 远程访问家里的 NAS
drip http 5000 --address 192.168.1.50

# 远程连接家庭网络
drip tcp 22
```

**Docker 和容器**
```bash
# 暴露容器化应用
drip http 8080 --address 172.17.0.3

# 调试数据库
drip tcp 5432 --address db-container
```

## 命令参考

```bash
# HTTP 隧道
drip http <端口> [选项]
  --subdomain, -n    自定义子域名
  --address, -a      目标地址（默认：127.0.0.1）
  --server           服务器地址
  --token            认证令牌

# HTTPS 隧道
drip https <端口> [选项]

# TCP 隧道
drip tcp <端口> [选项]

# 守护进程命令
drip daemon start <类型> <端口> [选项]
drip daemon list
drip daemon stop <名称>
drip daemon logs <名称>

# 配置管理
drip config init
drip config show
```

## 开源协议

BSD 3-Clause License - 详见 [LICENSE](LICENSE)
