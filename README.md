# kubectl ssh plugin

## based
create a Privileged in specified node and ensure host node(PID:1) all namespaces.

## how to use

```sh
$ git clone https://github.com/zxh326/kubectl-ssh
$ cd kubectl-ssh
$ GO111MODULE="on" go build cmd/kubectl-ssh.go 
$ chmod +x kubectl-ssh 
$ cp kubectl-ssh /usr/local/bin/kubectl-ssh
$ kubectl ssh nodeName
$ kubectl ssh -l nodeLable
```

## Cleanup

You can "uninstall" this plugin from kubectl by simply removing it from your PATH:

```sh
$ rm /usr/local/bin/kubectl-ssh
```

## other
this project just want to test & study kubectl plugin go's version,
you can use shell version achieve this.

## todo
- [ ] ssh to pod
- [ ] support real ssh(get node address and ssh it)