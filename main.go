package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"github.com/pborman/getopt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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

type Commander interface {
	Exec(command string, args ...string) ([]byte, error)
}

type RealCommander struct{}

func (c *RealCommander) Exec(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).Output()
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
		fmt.Println("DEBUG: starting kubectl-knife. Concurrency:", *max_concurrency)
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
	commander := &RealCommander{}
	for _, p := range k.pods {
		wg.Add(1)
		go runCommand(p.context, p.namespace, p.pod_name, *command, *shell, &wg, semaphore, commander)
	}
	wg.Wait()
}

func getContexts(filter string, skip_filter bool) ([]string, error) {
	if skip_filter {
		return []string{filter}, nil
	}

	config_loading_rules := clientcmd.NewDefaultClientConfigLoadingRules()
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(config_loading_rules, &clientcmd.ConfigOverrides{}).RawConfig()
	if err != nil {
		return nil, err
	}

	var contexts []string
	for name := range config.Contexts {
		contexts = append(contexts, name)
	}

	return filterString(contexts, filter), nil
}

func setContext(ctx string) (*kubernetes.Clientset, error) {
	config_loading_rules := clientcmd.NewDefaultClientConfigLoadingRules()
	config_overrides := &clientcmd.ConfigOverrides{
		CurrentContext: ctx,
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(config_loading_rules, config_overrides).ClientConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func getNamespaces(ctx, filter string, skip_filter bool) ([]string, error) {
	if skip_filter {
		return []string{filter}, nil
	}

	clientset, err := setContext(ctx)
	if err != nil {
		return nil, err
	}

	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var names []string
	for _, ns := range namespaces.Items {
		names = append(names, ns.Name)
	}

	return filterString(names, filter), nil
}

func getPods(ctx, ns, filter string, skip_filter bool) ([]string, error) {
	if skip_filter {
		return []string{filter}, nil
	}

	clientset, err := setContext(ctx)
	if err != nil {
		return nil, err
	}

	pods, err := clientset.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found")
	}
	var pod_names []string
	for _, pod := range pods.Items {
		pod_names = append(pod_names, pod.Name)
	}

	return filterString(pod_names, filter), nil
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

func filterString(items []string, pattern string) []string {
	var result []string
	re, err := regexp.Compile(pattern)
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

func runCommand(ctx, ns, pod, command, shell string, wg *sync.WaitGroup, semaphore chan struct{}, commander Commander) {
	defer wg.Done()
	semaphore <- struct{}{}
	defer func() { <-semaphore }()

	command_output, err := commander.Exec("kubectl", "exec", "--context", ctx, "-n", ns, pod, "--", shell, "-c", command)
	if err != nil {
		fmt.Printf("%s %s %s: %s\n", ctx, ns, pod, err)
	} else {
		fmt.Printf(formatOutput(ctx, ns, pod, string(command_output)))
	}
}

func formatOutput(ctx, ns, pod, output string) string {
	var result string
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	for _, line := range lines {
		result += fmt.Sprintf("%s %s %s: %s\n", ctx, ns, pod, line)
	}
	return result
}
