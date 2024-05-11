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
	debug := getopt.BoolLong("debug", 'd', "debug mode")
	cluster_filter := getopt.StringLong("context", 'c', "", "context regex")
	namespace_filter := getopt.StringLong("namespace", 'n', "", "namespace regex")
	pod_filter := getopt.StringLong("pod", 'p', "", "pod regex")
	command := getopt.StringLong("command", 'C', "", "command to run, if empty, just list pods")
	shell := getopt.StringLong("shell", 'S', "sh", "default: sh")
	skip_filter := getopt.BoolLong("skip-filter", 's', "skip filtering, does not use regex")
	max_concurrency := getopt.IntLong("max-concurrency", 'm', 10, "max concurrency, default: 10")


	getopt.Parse()

	if *help {
		getopt.Usage()
		os.Exit(0)
	}

	if *debug {
		fmt.Println("DEBUG: starting kube-knife. Concurrency:", *max_concurrency)
	}

	pods, err := discoveryPods(*cluster_filter, *namespace_filter, *pod_filter, *skip_filter, *debug, *max_concurrency)
	if err != nil {
		log.Fatal(err)
	}

	var k Knife
	k = Knife{
		pods:    pods,
		command: *command,
	}
	if *command == "" {
		if *debug {
			fmt.Println("DEBUG: listing pods")
		}
		k.Print()
		os.Exit(0)
	}
	if *debug {
		fmt.Println("DEBUG: executing command on pods")
	}
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, *max_concurrency)
	for _, p := range k.pods {
		wg.Add(1)
		go runCommand(p.context, p.namespace, p.pod_name, *command, *shell, &wg, semaphore)
	}
	wg.Wait()
}

func getContexts(filter string, skip_filter bool) ([]string, error) {
	if skip_filter {
		return []string{filter}, nil
	}
	cmd := exec.Command("kubectl", "config", "get-contexts", "--no-headers=true", "-o", "name")
	contexts, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	clusters := strings.Split(string(contexts), "\n")
	if len(clusters) == 0 {
		return nil, fmt.Errorf("no clusters found")
	}
	return filterString(clusters, filter), nil
}

func getNamespaces(ctx, filter string, skip_filter bool) ([]string, error) {
	if skip_filter{
		return []string{filter}, nil
	}
	cmd := exec.Command("kubectl", "get", "namespaces", "--no-headers=true", "--context", ctx, "-o", "name")
	raw_namespaces, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if len(raw_namespaces) == 0 {
		return nil, fmt.Errorf("no namespaces found")
	}
	return filterString(getName(string(raw_namespaces)), filter), nil
}

func getPods(ctx, ns, filter string, skip_filter bool) ([]string, error) {
	if skip_filter {
		return []string{filter}, nil
	}
	cmd := exec.Command("kubectl", "get", "pods", "--no-headers=true", "--context", ctx, "-n", ns, "-o", "name")
	raw_pods, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if len(raw_pods) == 0 {
		return nil, fmt.Errorf("no pods found")
	}
	return filterString(getName(string(raw_pods)), filter), nil
}

func discoveryPods(cluster_filter, namespace_filter, pod_filter string, skip_filter, debug bool, max_concurrency int) ([]Pod, error) {
	var pods []Pod
	clusters, err := getContexts(cluster_filter, skip_filter)
	if err != nil {
		return nil, err
	}

	podChan := make(chan []Pod)
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, max_concurrency)

	for _, ctx := range clusters {
		wg.Add(1)
		go func(ctx string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			var clusterPods []Pod

			namespaces, err := getNamespaces(ctx, namespace_filter, skip_filter)
			if err != nil || len(namespaces) == 0 {
				if debug {
					fmt.Println("DEBUG: no namespaces found for context:", ctx)
				}
				podChan <- clusterPods
				return
			}

			namespacePodChan := make(chan []Pod)
			var nsWg sync.WaitGroup

			for _, ns := range namespaces {
				nsWg.Add(1)
				go func(ctx, ns string) {
					defer nsWg.Done()
					ns_pods, err := getPods(ctx, ns, pod_filter, skip_filter)
					if err != nil || len(ns_pods) == 0 {
						namespacePodChan <- nil
						return
					}
					if debug {
						fmt.Printf("DEBUG: found %d pods in context %s namespace %s\n", len(ns_pods), ctx, ns)
					}
					var pods []Pod
					for _, pod := range ns_pods {
						if debug {
							fmt.Printf("DEBUG: found pod %s in context %s namespace %s\n", pod, ctx, ns)
						}
						clusterPods = append(clusterPods, Pod{
							pod_name:  pod,
							namespace: ns,
							context:   ctx,
						})
					}
					namespacePodChan <- pods
				}(ctx, ns)
			}
			go func() {
				nsWg.Wait()
				close(namespacePodChan)
			}()
			for range namespaces {
				namespacePods := <-namespacePodChan
				if namespacePods != nil {
					clusterPods = append(clusterPods, namespacePods...)
				}
			}
			podChan <- clusterPods
		}(ctx)
	}
	go func() {
		wg.Wait()
		close(podChan)
	}()

	for range clusters {
		clusterPods := <-podChan
		pods = append(pods, clusterPods...)
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

func runCommand(ctx, ns, pod, command, shell string, wg *sync.WaitGroup, semaphore chan struct{}) {
	defer wg.Done()
	semaphore <- struct{}{}
	defer func() { <-semaphore }()

	cmd := exec.Command("kubectl", "exec", "--context", ctx, "-n", ns, pod, "--", shell, "-c", command)
	command_output, err := cmd.Output()
	if err != nil {
		fmt.Printf("%s %s %s: %s\n", ctx, ns, pod, err)
	} else {
		command_output := strings.TrimSuffix(string(command_output), "\n")
		lines := strings.Split(command_output, "\n")
		for _, line := range lines {
			fmt.Printf("%s %s %s: %s\n", ctx, ns, pod, line)
		}
	}
}
