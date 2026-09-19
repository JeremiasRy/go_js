#!/bin/bash
set -e
git clone --depth 1 --branch v6.1.176 git://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git linux-6.1.176
cd linux-6.1.176 
wget https://raw.githubusercontent.com/firecracker-microvm/firecracker/refs/heads/main/resources/guest_configs/microvm-kernel-ci-x86_64-6.1.config -O .config
make olddefconfig
make vmlinux -j$(nproc)
