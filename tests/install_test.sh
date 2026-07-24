#!/bin/sh

set -eu

ROOT=$(mktemp -d)
RELEASES=$(mktemp -d)
BUILD=$(mktemp -d)
trap 'rm -rf "$ROOT" "$RELEASES" "$BUILD"' EXIT INT TERM

VERSION="v0.0.0-test"
mkdir -p "$RELEASES/$VERSION"

for ARCHIVE_ARCH in amd64 arm64 386 armv7; do
	PACKAGE="sni-unlock_${VERSION}_linux_${ARCHIVE_ARCH}"
	PACKAGE_DIR="$BUILD/$PACKAGE"
	mkdir -p "$PACKAGE_DIR"
	cat >"$PACKAGE_DIR/sni-unlock" <<'EOF'
#!/bin/sh
echo test
EOF
	chmod +x "$PACKAGE_DIR/sni-unlock"
	printf 'example.com\n' >"$PACKAGE_DIR/proxy-domains.example.txt"
	printf '# test\n' >"$PACKAGE_DIR/README.md"
	printf '[Unit]\n' >"$PACKAGE_DIR/sni-unlock.service"
	printf '#!/sbin/openrc-run\n' >"$PACKAGE_DIR/sni-unlock.openrc"
	tar -C "$BUILD" -czf "$RELEASES/$VERSION/$PACKAGE.tar.gz" "$PACKAGE"
done
(cd "$RELEASES/$VERSION" && sha256sum ./*.tar.gz >checksums.txt)

for SYSTEM in alpine ubuntu debian centos; do
	OS_RELEASE="$BUILD/os-release-$SYSTEM"
	printf 'ID=%s\n' "$SYSTEM" >"$OS_RELEASE"
	for ARCHIVE_ARCH in amd64 arm64 386 armv7; do
		MACHINE="$ARCHIVE_ARCH"
		[ "$ARCHIVE_ARCH" = "amd64" ] && MACHINE="x86_64"
		[ "$ARCHIVE_ARCH" = "386" ] && MACHINE="i686"
		[ "$ARCHIVE_ARCH" = "armv7" ] && MACHINE="armv7l"
		INSTALL_ROOT="$ROOT/$SYSTEM-$ARCHIVE_ARCH" \
		SKIP_SERVICE=1 \
		VERSION="$VERSION" \
		PROXY_IPV4="192.0.2.10" \
		RELEASE_BASE_URL="file://$RELEASES" \
		OS_RELEASE_FILE="$OS_RELEASE" \
		MACHINE="$MACHINE" \
		sh ./install.sh >/dev/null
		test -x "$ROOT/$SYSTEM-$ARCHIVE_ARCH/opt/sni-unlock/sni-unlock"
	done
done

INSTALL_DIR="$ROOT/ubuntu-amd64/opt/sni-unlock"
test -x "$INSTALL_DIR/sni-unlock"
test -f "$INSTALL_DIR/proxy-domains.txt"
grep -q 'proxy_ipv4: "192.0.2.10"' "$INSTALL_DIR/config.yaml"
grep -q 'listen: ":53"' "$INSTALL_DIR/config.yaml"
grep -q 'udp_enabled: false' "$INSTALL_DIR/config.yaml"

printf '\n# preserved\n' >>"$INSTALL_DIR/config.yaml"
printf 'custom.example\n' >"$INSTALL_DIR/proxy-domains.txt"
INSTALL_ROOT="$ROOT/ubuntu-amd64" \
SKIP_SERVICE=1 \
VERSION="$VERSION" \
PROXY_IPV4="198.51.100.20" \
RELEASE_BASE_URL="file://$RELEASES" \
OS_RELEASE_FILE="$BUILD/os-release-ubuntu" \
MACHINE="x86_64" \
sh ./install.sh
grep -q '# preserved' "$INSTALL_DIR/config.yaml"
grep -q 'proxy_ipv4: "192.0.2.10"' "$INSTALL_DIR/config.yaml"
grep -q 'custom.example' "$INSTALL_DIR/proxy-domains.txt"

printf 'installer smoke test passed\n'
