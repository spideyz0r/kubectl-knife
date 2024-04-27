# k8s-knife
k8s-knife is a tool to run commands on multiple pods concurrently using kubectl commands

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
Put together rpm and goreleaser pipeline
Make the discovery of the pods concurrently
Improve pod listing output, make it more useful
Maybe don't fail when context or ns fail, just inform and skip it
Add pod prefix when displaying multi-line output
Dinamycally tries /bin/sh, /bin/dash, /bin/bash, /bin/ash ===> sh -c "ls -d /tmp" || bash -c "ls -d /tmp" || ash -c "ls -d /tmp"  || dash -c "ls -d /tmp"
Put together some tests


