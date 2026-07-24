# sni-unlock

使用 Go 实现的 TCP DNS 与 SNI/HTTP 透明代理，工作方式参考 `dnsmasq + sniproxy`：

- 代理名单内的域名，DNS 返回本机代理 IP；
- 不在代理名单内的域名，DNS 请求原样转发给上游并返回原解析结果；
- HTTP 根据 `Host`、TLS 根据 ClientHello 中的 SNI 连接源站；
- DNS、HTTP、TLS 监听端口均可自定义；
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

默认值为 TCP DNS `:53`、HTTP `:80`、TLS `:443`。示例配置为了便于非 root 测试，使用 `5353/8080/8443`。

## 配置说明

完整配置见 `config.example.yaml`。

- `dns.listen`：对外 TCP DNS 监听地址和端口。
- `dns.upstream`：未命中名单时使用的上游 DNS。
- `dns.network`：访问上游 DNS 的协议，可设为 `tcp` 或 `udp`。
- `dns.proxy_ipv4/proxy_ipv6`：名单域名返回的代理服务器地址。
- `proxy.http_listen/proxy.tls_listen`：HTTP 与 TLS 代理监听端口。
- `proxy_domains`：代理根域名列表，自动匹配所有子域名。
- `allow_clients`：服务白名单，支持 IP/CIDR；空数组 `[]` 表示不限制。

TCP DNS 查询示例：

```bash
dig +tcp @127.0.0.1 -p 5353 netflix.com A
dig +tcp @127.0.0.1 -p 5353 example.com A
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

- 本服务只监听 TCP DNS，不监听 UDP DNS；客户端必须支持或显式启用 TCP 查询。
- TLS 代理依赖明文 SNI；使用 ECH 且不暴露 SNI 的连接无法代理。
- `allow_clients` 留空会对公网开放服务，生产环境建议设置白名单并同时配置防火墙。
- 代理服务器会直接连接名单域名的源站 IP，不解密或签发 TLS 证书。
