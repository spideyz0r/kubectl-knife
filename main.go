package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"github.com/pborman/getopt"
)

type Knife struct {
	pods    []Pod
	command string
	user    string
}

type Pod struct {
	pod_name  string
	namespace string
	context   string
}

func (k Knife) Print() {
	// maybe also list uptime, or get pod like output
	fmt.Printf("%s\t%s\t%s\n", "context", "namespace", "pod_name")
	for _, p := range k.pods {
		fmt.Printf("%s\t%s\t%s\n", p.context, p.namespace, p.pod_name)
	}
}

func main() {
	help := getopt.BoolLong("help", 'h', "display this help")
	cluster_filter := getopt.StringLong("context", 'c', "", "context regex")
	namespace_filter := getopt.StringLong("namespace", 'n', "", "namespace regex")
	pod_filter := getopt.StringLong("pod", 'p', "", "pod regex")
	command := getopt.StringLong("command", 'C', "", "command to run, if empty, just list pods")
	shell := getopt.StringLong("shell", 'S', "sh", "default: sh")
	skip_filter := getopt.BoolLong("skip-filter", 's', "skip filtering, does not use regex")

	getopt.Parse()

	if *help {
		getopt.Usage()
		os.Exit(0)
	}
	pods, err := discoveryPods(*cluster_filter, *namespace_filter, *pod_filter, *skip_filter)
	if err != nil {
		log.Fatal(err)
	}

	var k Knife
	k = Knife{
		pods:    pods,
		command: *command,
	}
	if *command == "" {
		k.Print()
		os.Exit(0)
	}
	var wg sync.WaitGroup
	for _, p := range k.pods {
		wg.Add(1)
		go runCommand(p.context, p.namespace, p.pod_name, *command, *shell, &wg)
	}
	wg.Wait()
}

func discoveryPods(cluster_filter, namespace_filter, pod_filter string, skip_filter bool) ([]Pod, error) {
	var pods []Pod
	filtered_contexts := []string{cluster_filter}
	if !skip_filter {
		cmd := exec.Command("kubectl", "config", "get-contexts", "--no-headers=true", "-o", "name")
		contexts, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		clusters := strings.Split(string(contexts), "\n")
		filtered_contexts = filterString(clusters, cluster_filter)
	}

	for _, ctx := range filtered_contexts {
		filtered_namespaces := []string{namespace_filter}
		if !skip_filter {
			cmd := exec.Command("kubectl", "get", "namespaces", "--no-headers=true", "--context", ctx, "-o", "name")
			raw_namespaces, err := cmd.Output()
			if err != nil {
				return nil, err
			}
			filtered_namespaces = filterString(getName(string(raw_namespaces)), namespace_filter)
		}
		for _, ns := range filtered_namespaces {
			cmd := exec.Command("kubectl", "get", "pods", "--no-headers=true", "--context", ctx, "-o", "name", "-n", ns)
			raw_pods, err := cmd.Output()
			if err != nil {
				return nil, err
			}
			filtered_pods := filterString(getName(string(raw_pods)), pod_filter)
			for _, pod := range filtered_pods {
				pods = append(pods, Pod{
					pod_name:  pod,
					namespace: ns,
					context:   ctx,
				})
			}
		}
	}
	return pods, nil
}

func filterString(items []string, patter string) []string {
	var result []string
	re, err := regexp.Compile(patter)
	if err != nil {
		log.Fatal(err)
	}
	for _, item := range items {
		if re.MatchString(item) {
			result = append(result, item)
		}
	}
	return result
}

func getName(kubectl_cmd string) []string {
	var result []string
	lines := strings.Split(strings.TrimSpace(string(kubectl_cmd)), "\n")
	for _, line := range lines {
		result = append(result, strings.Split(line, "/")[1])
	}
	return result
}

func runCommand(ctx, ns, pod, command, shell string, wg *sync.WaitGroup) {
	defer wg.Done()
	cmd := exec.Command("kubectl", "exec", "--context", ctx, "-n", ns, pod, "--", shell, "-c", command)
	command_output, err := cmd.Output()
	if err != nil {
		fmt.Printf("%s %s %s: %s\n", ctx, ns, pod, err)
	} else {
		// need to cleanout the output, or think on a way to repeat the pod name in each line of the output
		fmt.Printf("%s %s %s: %s", ctx, ns, pod, string(command_output))
	}
}
