#!/usr/bin/env bash
# Builds a FaroOS install ISO: a stock Ubuntu Server ISO repacked with an
# autoinstall config that does an unattended install and leaves FaroOS
# running. See README.md in this directory for what this does and doesn't
# do before running it.
set -euo pipefail

UBUNTU_VERSION="${UBUNTU_VERSION:-24.04.4}"
UBUNTU_SERIES="${UBUNTU_VERSION%.*}" # "24.04.4" -> "24.04"
ISO_NAME="ubuntu-${UBUNTU_VERSION}-live-server-amd64.iso"
ISO_URL="https://releases.ubuntu.com/${UBUNTU_SERIES}/${ISO_NAME}"
SUMS_URL="https://releases.ubuntu.com/${UBUNTU_SERIES}/SHA256SUMS"

# The FaroOS component to install on first boot: "server" (the central
# panel — the sensible default for a dedicated appliance box) or "agent"
# (to make this a plain managed node instead). Passed straight through to
# packaging/install.sh.
FAROOS_COMPONENT="${FAROOS_COMPONENT:-server}"
FAROOS_USERNAME="${FAROOS_USERNAME:-faroos-admin}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CACHE_DIR="${CACHE_DIR:-$SCRIPT_DIR/.cache}"
OUTPUT_DIR="${OUTPUT_DIR:-$SCRIPT_DIR/dist}"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

mkdir -p "$CACHE_DIR" "$OUTPUT_DIR"

for tool in xorriso sed openssl curl sha256sum; do
  command -v "$tool" >/dev/null 2>&1 || {
    echo "error: required tool '$tool' not found (on Debian/Ubuntu: apt-get install xorriso isolinux openssl curl coreutils)" >&2
    exit 1
  }
done

echo "==> FaroOS ISO build: Ubuntu ${UBUNTU_VERSION}, component=${FAROOS_COMPONENT}"

ISO_PATH="$CACHE_DIR/$ISO_NAME"
if [ -f "$ISO_PATH" ]; then
  echo "==> Using cached ISO: $ISO_PATH"
else
  echo "==> Downloading $ISO_NAME (a few GB, this takes a while)..."
  curl -fL --progress-bar -o "$ISO_PATH.partial" "$ISO_URL"
  mv "$ISO_PATH.partial" "$ISO_PATH"
fi

echo "==> Verifying checksum..."
curl -fsSL -o "$WORKDIR/SHA256SUMS" "$SUMS_URL"
expected_sum="$(grep -E " \*?${ISO_NAME}\$" "$WORKDIR/SHA256SUMS" | awk '{print $1}' | head -n1)"
if [ -z "$expected_sum" ]; then
  echo "error: could not find a checksum for $ISO_NAME in $SUMS_URL" >&2
  exit 1
fi
actual_sum="$(sha256sum "$ISO_PATH" | awk '{print $1}')"
if [ "$expected_sum" != "$actual_sum" ]; then
  echo "error: checksum mismatch for $ISO_PATH" >&2
  echo "  expected: $expected_sum" >&2
  echo "  actual:   $actual_sum" >&2
  echo "Deleting the cached file since it's corrupt or was tampered with; re-run to redownload." >&2
  rm -f "$ISO_PATH"
  exit 1
fi
echo "==> Checksum OK ($actual_sum)"

echo "==> Generating one-time admin credentials for this ISO..."
FAROOS_PASSWORD="$(openssl rand -base64 18)"
FAROOS_PASSWORD_HASH="$(openssl passwd -6 "$FAROOS_PASSWORD")"

echo "==> Extracting ISO contents..."
EXTRACT_DIR="$WORKDIR/iso"
mkdir -p "$EXTRACT_DIR"
xorriso -osirrox on -indev "$ISO_PATH" -extract / "$EXTRACT_DIR" >/dev/null 2>&1
chmod -R u+w "$EXTRACT_DIR"
rm -rf "${EXTRACT_DIR:?}/[BOOT]"

echo "==> Injecting autoinstall config..."
mkdir -p "$EXTRACT_DIR/nocloud"
sed \
  -e "s#__FAROOS_USERNAME__#${FAROOS_USERNAME}#g" \
  -e "s#__FAROOS_PASSWORD_HASH__#${FAROOS_PASSWORD_HASH}#g" \
  -e "s#__FAROOS_COMPONENT__#${FAROOS_COMPONENT}#g" \
  "$SCRIPT_DIR/autoinstall/user-data" >"$EXTRACT_DIR/nocloud/user-data"
cp "$SCRIPT_DIR/autoinstall/meta-data" "$EXTRACT_DIR/nocloud/meta-data"

echo "==> Patching boot menus to enable autoinstall..."
if [ -f "$EXTRACT_DIR/isolinux/txt.cfg" ]; then
  sed -i -e 's/---/ autoinstall  ---/g' "$EXTRACT_DIR/isolinux/txt.cfg"
  sed -i -e 's,---, ds=nocloud;s=/cdrom/nocloud/  ---,g' "$EXTRACT_DIR/isolinux/txt.cfg"
fi
if [ -f "$EXTRACT_DIR/boot/grub/grub.cfg" ]; then
  sed -i -e 's/---/ autoinstall  ---/g' "$EXTRACT_DIR/boot/grub/grub.cfg"
  sed -i -e 's,---, ds=nocloud\\;s=/cdrom/nocloud/  ---,g' "$EXTRACT_DIR/boot/grub/grub.cfg"
fi

OUT_ISO="$OUTPUT_DIR/faroos-${UBUNTU_VERSION}-amd64.iso"
echo "==> Repacking hybrid BIOS+UEFI ISO -> $OUT_ISO"
(
  cd "$EXTRACT_DIR"
  xorriso -as mkisofs -r -V "FAROOS_INSTALL" -J \
    -b isolinux/isolinux.bin -c isolinux/boot.cat -no-emul-boot \
    -boot-load-size 4 -isohybrid-mbr /usr/lib/ISOLINUX/isohdpfx.bin \
    -boot-info-table -input-charset utf-8 -eltorito-alt-boot \
    -e boot/grub/efi.img -no-emul-boot -isohybrid-gpt-basdat \
    -o "$OUT_ISO" .
)

CREDS_FILE="$OUTPUT_DIR/faroos-${UBUNTU_VERSION}-amd64.CREDENTIALS.txt"
cat >"$CREDS_FILE" <<EOF
FaroOS install ISO — local account created on first boot
==========================================================
Username: $FAROOS_USERNAME
Password: $FAROOS_PASSWORD

This account is for local console / SSH access to the underlying Ubuntu
Server only. Change the password after first boot (passwd), or set
FAROOS_USERNAME/rerun this script to change it before installing.

FaroOS itself ($FAROOS_COMPONENT) is set up separately — once the machine
has rebooted, open http://<this-machine-ip>:8090 to finish setup.
EOF
chmod 600 "$CREDS_FILE"

echo "==> Done."
echo "    ISO:         $OUT_ISO"
echo "    Credentials: $CREDS_FILE"
