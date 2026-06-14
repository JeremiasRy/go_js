#!/bin/bash
set -e

ARCH="$(uname -m)"
release_url="https://github.com/firecracker-microvm/firecracker/releases"
latest_version=$(basename $(curl -fsSLI -o /dev/null -w  %{url_effective} ${release_url}/latest))
CI_VERSION=${latest_version%.*}

latest_ubuntu_key=$(curl "http://spec.ccfc.min.s3.amazonaws.com/?prefix=firecracker-ci/$CI_VERSION/$ARCH/ubuntu-&list-type=2" \
    | grep -oP "(?<=<Key>)(firecracker-ci/$CI_VERSION/$ARCH/ubuntu-[0-9]+\.[0-9]+\.squashfs)(?=</Key>)" \
    | sort -V | tail -1)

if [ -z "$latest_ubuntu_key" ]; then
    echo "Warning: Could not detect artifacts for $CI_VERSION. Falling back to v1.10."
    CI_VERSION="v1.10"
    latest_ubuntu_key=$(curl -s "http://spec.ccfc.min.s3.amazonaws.com/?prefix=firecracker-ci/$CI_VERSION/$ARCH/ubuntu-&list-type=2" \
        | grep -oP "(?<=<Key>)(firecracker-ci/$CI_VERSION/$ARCH/ubuntu-[0-9]+\.[0-9]+\.squashfs)(?=</Key>)" \
        | sort -V | tail -1)
fi

ubuntu_version=$(basename $latest_ubuntu_key .squashfs | grep -oE '[0-9]+\.[0-9]+')

if [ -z "$ubuntu_version" ]; then
    echo "Warning: Could not detect Ubuntu version from S3. Defaulting to 'latest'."
    ubuntu_version="latest"
fi

wget -O ubuntu-$ubuntu_version.squashfs.upstream "https://s3.amazonaws.com/spec.ccfc.min/$latest_ubuntu_key"

rm -rf squashfs-root 
unsquashfs ubuntu-$ubuntu_version.squashfs.upstream
mkdir -p squashfs-root/workspace

cp ./go_js squashfs-root/usr/local/bin/go_js
chmod +x squashfs-root/usr/local/bin/go_js

ssh-keygen -f id_rsa -N ""
cp -v id_rsa.pub squashfs-root/root/.ssh/authorized_keys
mv -v id_rsa ./ubuntu-$ubuntu_version.id_rsa

chown -R root:root squashfs-root
truncate -s 1G ubuntu-$ubuntu_version.ext4
mkfs.ext4 -d squashfs-root -F ubuntu-$ubuntu_version.ext4

KERNEL=vmlinux-6.1
echo
echo "The following files were downloaded and set up:"
[ -f $KERNEL ] && echo "Kernel: $KERNEL" || echo "ERROR: Kernel $KERNEL does not exist"

ROOTFS=$(ls *.ext4 | tail -1)
[ -f "$ROOTFS" ] && echo "Rootfs: $ROOTFS" || echo "ERROR: Rootfs does not exist"

KEY_NAME=$(ls *.id_rsa | tail -1)
[ -f "$KEY_NAME" ] && echo "SSH Key: $KEY_NAME" || echo "ERROR: Key $KEY_NAME does not exist"

ARCH="$(uname -m)"
release_url="https://github.com/firecracker-microvm/firecracker/releases"
latest=$(basename $(curl -fsSLI -o /dev/null -w  %{url_effective} ${release_url}/latest))
curl -L ${release_url}/download/${latest}/firecracker-${latest}-${ARCH}.tgz \
| tar -xz

mv release-${latest}-$(uname -m)/firecracker-${latest}-${ARCH} firecracker
