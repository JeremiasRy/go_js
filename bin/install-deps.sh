#!/bin/bash
set -e
apt-get update && apt-get install -y \
    build-essential \
    bc \
    bison \
    flex \
    libssl-dev \
    libelf-dev \
    wget \
    tar \
    curl \
    squashfs-tools \
    openssh-client 