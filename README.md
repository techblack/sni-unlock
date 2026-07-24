# sni-unlock

使用 Go 实现的 TCP/UDP DNS 与 SNI/HTTP 透明代理，工作方式参考 `dnsmasq + sniproxy`：

- 代理名单内的域名，DNS 返回本机代理 IP；
- 不在代理名单内的域名，DNS 请求原样转发给上游并返回原解析结果；
- HTTP 根据 `Host`、TLS 根据 ClientHello 中的 SNI 连接源站；
- DNS、HTTP、TLS 监听端口均可自定义；
- TCP DNS 始终启用，UDP DNS 可通过配置按需启用；
- 可通过客户端 IP/CIDR 白名单限制谁能使用全部服务；
- 代理入口再次校验域名名单，避免成为开放代理。

## 构建

```bash
go build -o sni-unlock ./cmd/sniproxy-dns
cp config.example.yaml config.yaml
```

编辑 `config.yaml`，至少把 `dns.proxy_ipv4` 改成客户端能够访问到的代理服务器地址，然后启动：

```bash
./sni-unlock -config config.yaml
```

默认值为 TCP DNS `:53`、HTTP `:80`、TLS `:443`，UDP DNS 默认关闭。示例配置为了便于非 root 测试，将 DNS 改为 `5353`。

## 一键安装

支持 Alpine、Ubuntu、Debian、CentOS，以及 `x86_64/amd64`、`x86/386`、`aarch64/arm64`、`armv7`。程序和配置统一安装到 `/opt/sni-unlock`，并自动注册 systemd 或 OpenRC 服务：

```bash
curl -fsSL https://raw.githubusercontent.com/techblack/sni-unlock/main/install.sh | sudo sh
```

也可以使用 wget：

```bash
wget -qO- https://raw.githubusercontent.com/techblack/sni-unlock/main/install.sh | sudo sh
```

安装脚本自动下载最新 Release、验证 SHA-256 校验和，并尝试获取本机公网 IPv4。NAT、多出口或自动识别不准确时，请明确指定：

```bash
curl -fsSL https://raw.githubusercontent.com/techblack/sni-unlock/main/install.sh | sudo PROXY_IPV4=203.0.113.10 sh
```

安装指定版本：

```bash
curl -fsSL https://raw.githubusercontent.com/techblack/sni-unlock/main/install.sh | sudo VERSION=v0.2.0 sh
```

重复运行脚本会升级程序和 `/opt/sni-unlock/proxy-domains.example.txt`，但保留已有 `config.yaml` 和 `proxy-domains.txt`。常用服务命令：

```bash
# Ubuntu / Debian / CentOS
systemctl status sni-unlock
systemctl restart sni-unlock

# Alpine
rc-service sni-unlock status
rc-service sni-unlock restart
```

## 配置说明

完整配置见 `config.example.yaml`。

- `dns.listen`：对外 DNS 监听地址和端口；TCP/UDP 共用此地址。
- `dns.udp_enabled`：是否同时启用 UDP DNS，默认 `false`。
- `dns.upstream`：未命中名单时使用的上游 DNS。
- `dns.network`：访问上游 DNS 的协议，可设为 `tcp` 或 `udp`。
- `dns.proxy_ipv4/proxy_ipv6`：名单域名返回的代理服务器地址。
- `proxy.http_listen/proxy.tls_listen`：HTTP 与 TLS 代理监听端口。
- `proxy_domains`：配置内的代理根域名列表，自动匹配所有子域名。
- `proxy_domains_file`：外部域名文本文件；每行一个域名，忽略空行和 `#` 注释，相对路径以配置文件目录为基准。
- `allow_clients`：服务白名单，支持 IP/CIDR；空数组 `[]` 表示不限制。

`proxy_domains` 与 `proxy_domains_file` 可以同时设置，服务会合并两份名单；也可以仅设置其中一种。外部文件示例见 `proxy-domains.example.txt`：

```text
# 流媒体域名
netflix.com
netflix.net
disneyplus.com # Disney+
```

Docker 部署时，需要将配置引用的名单文件一并挂载到容器内，例如：

```bash
docker run --rm --network host \
  -v "$PWD/config.yaml:/config.yaml:ro" \
  -v "$PWD/proxy-domains.txt:/proxy-domains.txt:ro" \
  sni-unlock
```

DNS 查询示例：

```bash
dig +tcp @127.0.0.1 -p 5353 netflix.com A
dig +tcp @127.0.0.1 -p 5353 example.com A
dig @127.0.0.1 -p 5353 netflix.com A
```

若使用非标准 HTTP/TLS 端口，DNS 只负责返回 IP，客户端还必须显式访问对应端口。浏览器默认访问 `80/443`，因此实际透明代理部署通常仍监听这两个端口。

## Docker

```bash
docker build -t sni-unlock .
docker run --rm --network host \
  -v "$PWD/config.yaml:/config.yaml:ro" \
  sni-unlock
```

## 注意事项

- UDP DNS 默认关闭；启用公网 UDP DNS 前，强烈建议设置 `allow_clients` 并配置防火墙，避免成为开放递归解析器或被用于 DNS 放大攻击。
- TLS 代理依赖明文 SNI；使用 ECH 且不暴露 SNI 的连接无法代理。
- `allow_clients` 留空会对公网开放服务，生产环境建议设置白名单并同时配置防火墙。
- 代理服务器会直接连接名单域名的源站 IP，不解密或签发 TLS 证书。
