FROM ubuntu:24.04

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends openssh-server \
    && rm -rf /var/lib/apt/lists/* /etc/ssh/ssh_host_* \
    && useradd --create-home --shell /bin/bash demo \
    && passwd -d demo \
    && install -d -m 700 -o demo -g demo /home/demo/.ssh \
    && printf 'Bastion host: bastion\n' > /host-info.txt \
    && chmod 644 /host-info.txt

COPY --chown=demo:demo --chmod=600 keys/demo_ed25519.pub /home/demo/.ssh/authorized_keys
COPY ssh/sshd_config /etc/ssh/sshd_config
COPY --chmod=755 ssh/entrypoint.sh /usr/local/bin/start-sshd

EXPOSE 22
ENTRYPOINT ["/usr/local/bin/start-sshd"]
