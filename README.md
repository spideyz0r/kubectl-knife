# kube-knife
[![goreleaser](https://github.com/spideyz0r/kube-knife/actions/workflows/release.yml/badge.svg)](https://github.com/spideyz0r/kube-knife/actions/workflows/release.yml) ![CI](https://github.com/spideyz0r/kube-knife/workflows/gotester/badge.svg)

kube-knife is a tool to run commands on multiple pods concurrently using kubectl commands

## Install

### Binary: MacOS (amd64/arm64), Windows, Linux
```
https://github.com/spideyz0r/kube-knife/releases
```
### From source
```
git checkout https://github.com/spideyz0r/kube-knife
cd kube-knife; go build -v -o kube-knife
```

## Usage
```
Usage: kube-knife [-dhs] [-C value] [-c value] [-m value] [-n value] [-p value] [-S value] [parameters ...]
 -C, --command=value
                    command to run, if empty, just list pods
 -c, --context=value
                    context regex
 -d, --debug        debug mode
 -h, --help         display this help
 -m, --max-concurrency=value
                    max concurrency, default: 10
 -n, --namespace=value
                    namespace regex
 -p, --pod=value    pod regex
 -S, --shell=value  default: sh
 -s, --skip-filter  skip filtering, does not use regex
```

## TODO
- Improve pod listing output, and make it more useful
- Dynamically tries /bin/sh, /bin/dash, /bin/bash, /bin/ash ===> sh -c "ls -d /tmp" || bash -c "ls -d /tmp" || ash -c "ls -d /tmp"  || dash -c "ls -d /tmp"
- Put together some tests


