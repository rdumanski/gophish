#!/usr/bin/env bash
###############################################################################
# Heavy lifting for the gophish EC2 box. Safe to re-run by hand:
#   sudo bash /opt/gophish-src/deploy/aws/provision.sh
# Idempotent — installs Docker, mounts the data volume (format-once), adds swap,
# then builds and (re)starts the container.
###############################################################################
set -euxo pipefail
exec > /var/log/gophish-provision.log 2>&1

DATA_DIR=/opt/gophish-data
SRC_DIR=/opt/gophish-src
COMPOSE_FILE="$SRC_DIR/deploy/aws/docker-compose.yml"
APP_UID=1000 # the 'app' user created in the Dockerfile

###############################################################################
# 1. Docker Engine + compose plugin
###############################################################################
if ! command -v docker >/dev/null 2>&1; then
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  . /etc/os-release
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu ${VERSION_CODENAME} stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -y
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  systemctl enable --now docker
fi

###############################################################################
# 2. Persistent data volume — find the spare disk, format ONCE, mount
#    EBS device names are not stable on Nitro (sdf -> some nvmeXn1), so detect
#    the whole, unpartitioned, unmounted disk instead of trusting a path.
###############################################################################
mkdir -p "$DATA_DIR"

DATA_DEVICE="${DATA_DEVICE:-}"
if [ -z "$DATA_DEVICE" ]; then
  for i in $(seq 1 30); do
    DATA_DEVICE="$(lsblk -dpno NAME,TYPE | awk '$2=="disk"{print $1}' | while read -r d; do
      mp="$(lsblk -no MOUNTPOINT "$d" | tr -d ' \n')"
      kids="$(lsblk -no NAME "$d" | tail -n +2)"
      if [ -z "$mp" ] && [ -z "$kids" ]; then echo "$d"; fi
    done | head -1)"
    [ -n "$DATA_DEVICE" ] && break
    sleep 2
  done
fi

if [ -n "$DATA_DEVICE" ] && [ -b "$DATA_DEVICE" ]; then
  # blkid succeeds only if a filesystem already exists -> never reformat live data.
  if ! blkid "$DATA_DEVICE"; then
    mkfs.ext4 -L gophishdata "$DATA_DEVICE"
  fi
  grep -q 'LABEL=gophishdata' /etc/fstab || \
    echo "LABEL=gophishdata $DATA_DIR ext4 defaults,nofail 0 2" >> /etc/fstab
  mount -a
else
  echo "WARNING: no spare data volume found; SQLite DB will live on the root disk." >&2
fi

# The container runs as uid 1000 and writes the DB into this dir (mounted at /data).
chown -R "$APP_UID:$APP_UID" "$DATA_DIR"

###############################################################################
# 3. Swap — insurance for the in-container build on smaller instances
###############################################################################
if [ ! -f /swapfile ]; then
  fallocate -l 2G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile
  grep -q '/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

###############################################################################
# 4. Build and run
###############################################################################
cd "$SRC_DIR"
docker compose -f "$COMPOSE_FILE" up -d --build

echo "Provision complete. Initial admin password:"
docker compose -f "$COMPOSE_FILE" logs gophish 2>&1 | grep -i 'password' || \
  echo "(password line not in logs yet; re-check with: docker compose -f $COMPOSE_FILE logs gophish | grep -i password)"
