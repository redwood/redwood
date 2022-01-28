
# Enabling IPv6 connections when running Redwood in Docker

### 1. Configure the host

**/etc/sysctl.conf**

```
net.ipv6.conf.all.disable_ipv6 = 0
net.ipv6.conf.default.disable_ipv6 = 0
```

```sh
sysctl -p
ip6tables -t nat -A POSTROUTING -s fd00::/80 ! -o docker0 -j MASQUERADE
```

### 2. Configure the Docker daemon

**/etc/docker/daemon.json**

```json
{
    "ipv6": true,
    "fixed-cidr-v6": "fd00::/80"
}
```

```sh
systemctl restart docker
```



