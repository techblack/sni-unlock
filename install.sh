#!/bin/sh

set -eu

REPOSITORY="techblack/sni-unlock"
INSTALL_DIR="${INSTALL_DIR:-/opt/sni-unlock}"
RELEASE_VERSION="${VERSION:-latest}"
PROXY_IPV4="${PROXY_IPV4:-}"
INSTALL_ROOT="${INSTALL_ROOT:-}"
SKIP_SERVICE="${SKIP_SERVICE:-0}"
RELEASE_BASE_URL="${RELEASE_BASE_URL:-https://github.com/$REPOSITORY/releases/download}"
OS_RELEASE_FILE="${OS_RELEASE_FILE:-/etc/os-release}"
MACHINE="${MACHINE:-$(uname -m)}"

log() {
	printf '[sni-unlock] %s\n' "$*"
}

fail() {
	printf '[sni-unlock] 错误: %s\n' "$*" >&2
	exit 1
}

require_root() {
	[ -n "$INSTALL_ROOT" ] || [ "$(id -u)" -eq 0 ] || fail "请使用 root 用户运行"
}

detect_system() {
	[ -r "$OS_RELEASE_FILE" ] || fail "无法识别操作系统"
	# shellcheck disable=SC1090
	. "$OS_RELEASE_FILE"
	case "${ID:-}" in
		alpine)
			SYSTEM="alpine"
			SERVICE_MANAGER="openrc"
			;;
		ubuntu | debian)
			SYSTEM="$ID"
			SERVICE_MANAGER="systemd"
			;;
		centos)
			SYSTEM="centos"
			SERVICE_MANAGER="systemd"
			;;
		*) fail "仅支持 Alpine、Ubuntu、Debian 和 CentOS，当前为 ${ID:-unknown}" ;;
	esac
}

detect_architecture() {
	case "$MACHINE" in
		x86_64 | amd64) ARCH="amd64" ;;
		aarch64 | arm64) ARCH="arm64" ;;
		i386 | i486 | i586 | i686 | 386) ARCH="386" ;;
		armv7 | armv7l) ARCH="armv7" ;;
		*) fail "仅支持 x86_64/amd64、x86/386、aarch64/arm64 和 armv7" ;;
	esac
}

install_downloader() {
	if command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1; then
		return
	fi
	log "安装下载工具"
	case "$SYSTEM" in
		alpine) apk add --no-cache curl ca-certificates ;;
		ubuntu | debian)
			apt-get update
			DEBIAN_FRONTEND=noninteractive apt-get install -y curl ca-certificates
			;;
		centos) yum install -y curl ca-certificates ;;
	esac
}

download() {
	url="$1"
	destination="$2"
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL --retry 3 --connect-timeout 15 -o "$destination" "$url"
	else
		wget -O "$destination" "$url"
	fi
}

resolve_version() {
	if [ "$RELEASE_VERSION" != "latest" ]; then
		case "$RELEASE_VERSION" in v*) ;; *) RELEASE_VERSION="v$RELEASE_VERSION" ;; esac
		return
	fi
	latest_url="https://api.github.com/repos/$REPOSITORY/releases/latest"
	if command -v curl >/dev/null 2>&1; then
		metadata=$(curl -fsSL --retry 3 --connect-timeout 15 "$latest_url")
	else
		metadata=$(wget -qO- "$latest_url")
	fi
	RELEASE_VERSION=$(printf '%s\n' "$metadata" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
	case "$RELEASE_VERSION" in v*) ;; *) fail "无法获取最新版本" ;; esac
}

detect_proxy_ipv4() {
	if [ -n "$PROXY_IPV4" ]; then
		return 0
	fi
	if command -v curl >/dev/null 2>&1; then
		PROXY_IPV4=$(curl -4fsS --connect-timeout 5 https://api.ipify.org || true)
	else
		PROXY_IPV4=$(wget -qO- -T 5 https://api.ipify.org || true)
	fi
	if [ -z "$PROXY_IPV4" ] && command -v ip >/dev/null 2>&1; then
		PROXY_IPV4=$(ip -4 route get 1.1.1.1 2>/dev/null | sed -n 's/.* src \([^ ]*\).*/\1/p' | head -1)
	fi
	[ -n "$PROXY_IPV4" ] || fail "无法自动获取 IPv4，请通过 PROXY_IPV4=1.2.3.4 指定"
}

install_files() {
	archive_name="sni-unlock_${RELEASE_VERSION}_linux_${ARCH}.tar.gz"
	archive="$TEMP_DIR/$archive_name"
	url="$RELEASE_BASE_URL/$RELEASE_VERSION/$archive_name"
	log "下载 $RELEASE_VERSION ($SYSTEM/$ARCH)"
	download "$url" "$archive"
	checksums="$TEMP_DIR/checksums.txt"
	download "$RELEASE_BASE_URL/$RELEASE_VERSION/checksums.txt" "$checksums"
	expected_checksum=$(sed -n "s#^\\([0-9a-fA-F][0-9a-fA-F]*\\)[[:space:]][[:space:]]*\\./$archive_name\$#\\1#p" "$checksums")
	[ -n "$expected_checksum" ] || fail "checksums.txt 中缺少 $archive_name"
	actual_checksum=$(sha256sum "$archive" | awk '{print $1}')
	[ "$actual_checksum" = "$expected_checksum" ] || fail "$archive_name 校验失败"
	tar -xzf "$archive" -C "$TEMP_DIR"
	package_dir="$TEMP_DIR/sni-unlock_${RELEASE_VERSION}_linux_${ARCH}"
	[ -x "$package_dir/sni-unlock" ] || fail "发布包缺少 sni-unlock"

	target_dir="$INSTALL_ROOT$INSTALL_DIR"
	mkdir -p "$target_dir"
	cp "$package_dir/sni-unlock" "$target_dir/sni-unlock"
	chmod 0755 "$target_dir/sni-unlock"
	cp "$package_dir/proxy-domains.example.txt" "$target_dir/proxy-domains.example.txt"
	if [ ! -f "$target_dir/proxy-domains.txt" ]; then
		cp "$package_dir/proxy-domains.example.txt" "$target_dir/proxy-domains.txt"
	else
		log "保留已有名单 $INSTALL_DIR/proxy-domains.txt"
	fi
	cp "$package_dir/README.md" "$target_dir/README.md"

	if [ ! -f "$target_dir/config.yaml" ]; then
		cat >"$target_dir/config.yaml" <<EOF
dns:
  listen: ":53"
  udp_enabled: false
  upstream: "1.1.1.1:53"
  network: "tcp"
  proxy_ipv4: "$PROXY_IPV4"
  proxy_ipv6: ""
  ttl: 60

proxy:
  http_listen: ":80"
  tls_listen: ":443"
  http_target_port: 80
  tls_target_port: 443
  dial_timeout: "10s"

proxy_domains_file: "proxy-domains.txt"
allow_clients: []
EOF
		chmod 0644 "$target_dir/config.yaml"
	else
		log "保留已有配置 $INSTALL_DIR/config.yaml"
	fi
}

install_service() {
	if [ "$SKIP_SERVICE" != "0" ]; then
		return 0
	fi
	case "$SERVICE_MANAGER" in
		systemd)
		service_path="$INSTALL_ROOT/etc/systemd/system/sni-unlock.service"
		mkdir -p "$(dirname "$service_path")"
		cp "$PACKAGE_DIR/sni-unlock.service" "$service_path"
		systemctl daemon-reload
		systemctl enable sni-unlock
		systemctl restart sni-unlock
		sleep 1
		systemctl is-active --quiet sni-unlock || fail "服务启动失败，请运行 journalctl -u sni-unlock 查看日志"
		;;
		openrc)
		service_path="$INSTALL_ROOT/etc/init.d/sni-unlock"
		mkdir -p "$(dirname "$service_path")"
		cp "$PACKAGE_DIR/sni-unlock.openrc" "$service_path"
		chmod 0755 "$service_path"
		rc-update add sni-unlock default
		if rc-service sni-unlock status >/dev/null 2>&1; then
			rc-service sni-unlock restart
		else
			rc-service sni-unlock start
		fi
		;;
	esac
}

main() {
	require_root
	detect_system
	detect_architecture
	install_downloader
	resolve_version
	detect_proxy_ipv4
	TEMP_DIR=$(mktemp -d)
	trap 'rm -rf "$TEMP_DIR"' EXIT INT TERM
	install_files
	PACKAGE_DIR="$TEMP_DIR/sni-unlock_${RELEASE_VERSION}_linux_${ARCH}"
	install_service
	log "安装完成: $INSTALL_DIR"
	log "配置文件: $INSTALL_DIR/config.yaml"
	log "代理名单: $INSTALL_DIR/proxy-domains.txt"
	log "服务白名单默认为空，请按需设置 allow_clients"
}

main "$@"
