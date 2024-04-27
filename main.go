package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/pborman/getopt"
)

func main() {
	help := getopt.BoolLong("help", 'h', "display this help")
	cluster_filter := getopt.StringLong("context", 'c', "", "context regex")
	namespace_filter := getopt.StringLong("namespace", 'n', "", "namespace regex")
	pod_filter := getopt.StringLong("pod", 'p', "", "pod regex")
	command := getopt.StringLong("command", 'C', "", "command to run, if empty, just list pods")
	skip_filter := getopt.BoolLong("skip-filter", 's', "skip filtering, does not use regex")

	getopt.Parse()

	if *help {
		getopt.Usage()
		os.Exit(0)
	}
	fmt.Println("cluster:", *cluster_filter)
	fmt.Println("namespace:", *namespace_filter)
	fmt.Println("pod:", *pod_filter)
	fmt.Println("command:", *command)
	cmd := exec.Command("kubectl", "config", "get-contexts", "--no-headers=true", "-o", "name")
	filtered_contexts := []string{*cluster_filter}
	if !*skip_filter {
		fmt.Println("Not skipping ctx filter")
		contexts, err := cmd.Output()
		if err != nil {
			fmt.Println(err)
			log.Fatal(err)
		}
		clusters := strings.Split(string(contexts), "\n")
		filtered_contexts = filterString(clusters, *cluster_filter)
	}

	for _, ctx := range filtered_contexts {
		fmt.Println(">>>>> cluster:", ctx)
		filtered_namespaces := []string{*namespace_filter}
		if !*skip_filter {
			fmt.Println("Not skipping ns filter")
			cmd := exec.Command("kubectl", "get", "namespaces", "--no-headers=true", "--context", ctx, "-o", "name")
			raw_namespaces, err := cmd.Output()
			if err != nil {
				fmt.Println(err)
				log.Fatal(err)
			}
			var namespaces []string
			lines := strings.Split(strings.TrimSpace(string(raw_namespaces)), "\n")
			for _, line := range lines {
				namespaces = append(namespaces, strings.Split(line, "/")[1])
			}
			filtered_namespaces = filterString(namespaces, *namespace_filter)
		}
		for _, ns := range filtered_namespaces {
			fmt.Println(">> namespace:", ns)
			cmd := exec.Command("kubectl", "get", "pods", "--no-headers=true", "--context", ctx, "-o", "name", "-n", ns)
			raw_pods, err := cmd.Output()
			if err != nil {
				fmt.Println(err)
				log.Fatal(err)
			}
			var pods []string
			lines := strings.Split(strings.TrimSpace(string(raw_pods)), "\n")
			for _, line := range lines {
				pods = append(pods, strings.Split(line, "/")[1])
			}
			filtered_pods := filterString(pods, *pod_filter)
			fmt.Println(filtered_pods)
			for _, pod := range filtered_pods {
				cmd := exec.Command("kubectl", "exec", "--context", ctx, "-n", ns, pod, "--", "sh", "-c", *command)
				command_output, err := cmd.Output()
				if err != nil {
					fmt.Printf("%s %s %s: %s\n", ctx, ns, pod, err)

					continue
				}
				// need to cleanout the output
				fmt.Printf("%s %s %s: %s", ctx, ns, pod, string(command_output))
			}

		}
	}
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
