#!/bin/bash
set -e
wget https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-6.1.tar.xz  
tar -xvf linux-6.1.tar.xz
cd linux-6.1 
wget https://raw.githubusercontent.com/firecracker-microvm/firecracker/refs/heads/main/resources/guest_configs/microvm-kernel-ci-x86_64-6.1.config -O .config
make olddefconfig
make vmlinux -j$(echo $(nproc --all) / 2 | bc)
