


## Chaos mesh

- https://rancher.com/docs/k3s/latest/en/quick-start/
- https://chaos-mesh.org/docs/quick-start/


Install `k3s`:
```sh
curl -sfL https://get.k3s.io | sh -
```

Install Chaos Mesh:
```sh
curl -sSL https://mirrors.chaos-mesh.org/v2.1.0/install.sh | sudo bash -s -- --k3s
```

Open up a port to the UI dashboard:
```sh
sudo kubectl patch service chaos-dashboard -n chaos-testing --type='json' --patch='[{"op": "replace", "path": "/spec/ports/0/nodePort", "value":31111}]'
```


If the `chaos-testing` namespace gets stuck in the "Terminating" state, reinstall k3s:
```sh
sudo /usr/local/bin/k3s-killall.sh
sudo /usr/local/bin/k3s-uninstall.sh
```

...or maybe try following this guide (untested): https://craignewtondev.medium.com/how-to-fix-kubernetes-namespace-deleting-stuck-in-terminating-state-5ed75792647e