#!/bin/sh
set -eu

mkdir -p /run/sshd
umask 077
mkdir -p /var/lib/ssh-host-keys
chmod 700 /var/lib/ssh-host-keys
for type in rsa ecdsa ed25519; do
    key="/var/lib/ssh-host-keys/ssh_host_${type}_key"
    if [ ! -e "$key" ]; then
        ssh-keygen -q -t "$type" -N '' -f "$key"
    fi
done
exec /usr/sbin/sshd -D -e
