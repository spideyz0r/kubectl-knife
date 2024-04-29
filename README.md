# kube-knife
[![goreleaser](https://github.com/spideyz0r/kube-knife/actions/workflows/release.yml/badge.svg)](https://github.com/spideyz0r/kube-knife/actions/workflows/release.yml)

kube-knife is a tool to run commands on multiple pods concurrently using kubectl commands

## Usage
```
Usage: k8s-knife [-hs] [-C value] [-c value] [-n value] [-p value] [-S value] [parameters ...]
 -C, --command=value
                    command to run, if empty, just list pods
 -c, --context=value
                    context regex
 -h, --help         display this help
 -n, --namespace=value
                    namespace regex
 -p, --pod=value    pod regex
 -S, --shell=value  default: sh
 -s, --skip-filter  specific; skip filtering, does not use regex

```

## TODO
- Improve pod listing output, and make it more useful
- Add pod prefix when displaying multi-line output
- Dynamically tries /bin/sh, /bin/dash, /bin/bash, /bin/ash ===> sh -c "ls -d /tmp" || bash -c "ls -d /tmp" || ash -c "ls -d /tmp"  || dash -c "ls -d /tmp"
- Put together some tests


