#!/bin/bash
set -e

apt-get update && apt-get install -y \
    iproute2 \
    iptables

BRIDGE_DEV="br0"
BRIDGE_IP="172.16.0.1/24"

ip link add name "$BRIDGE_DEV" type bridge
ip addr add "$BRIDGE_IP" dev "$BRIDGE_DEV"
ip link set dev "$BRIDGE_DEV" up

iptables -t nat -A POSTROUTING -s 172.16.0.0/24 -o eth0 -j MASQUERADE
iptables -A FORWARD -i "$BRIDGE_DEV" -j ACCEPT
iptables -A FORWARD -o "$BRIDGE_DEV" -m state --state RELATED,ESTABLISHED -j ACCEPT

echo "Network bridge created. Starting Go server..."
./web