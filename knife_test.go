package main

import (
	"fmt"
	"testing"
	"sync"
	"reflect"
)

func TestFormatOutput(t *testing.T) {
	testCases := []struct {
		ctx, ns, pod string
		output       string
		expected     string
	}{
		{"ctx1", "ns1", "pod1", "line1\nline2", "ctx1 ns1 pod1: line1\nctx1 ns1 pod1: line2\n"},
		{"ctx2", "ns2", "pod2", "", "ctx2 ns2 pod2: \n"},
		{"ctx3", "ns3", "pod3", "singleLine", "ctx3 ns3 pod3: singleLine\n"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s/%s/%s", tc.ctx, tc.ns, tc.pod), func(t *testing.T) {
			result := formatOutput(tc.ctx, tc.ns, tc.pod, tc.output)
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

type MockCommander struct {
	Output []byte
	Err    error
}

func (m *MockCommander) Exec(command string, args ...string) ([]byte, error) {
	return m.Output, m.Err
}

func TestRunCommand(t *testing.T) {
    ctx, ns, pod, command, shell := "ctx1", "ns1", "somepod", "echo 'hello'", "/bin/bash"
	var wg sync.WaitGroup
    semaphore := make(chan struct{}, 1) // Limit concurrency to simulate behavior

    testCases := []struct {
        name    string
        output  string
        err     error
        expect  string
    }{
        {"Success", "hello\nworld", nil, "ctx1 ns1 somepod: hello\ntestCtx testNs testPod: world\n"},
        {"Fail", "", fmt.Errorf("error"), "ctx1 ns1 somepod: error\n"},
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
			wg.Add(1)
            commander := &MockCommander{Output: []byte(tc.output), Err: tc.err}
            runCommand(ctx, ns, pod, command, shell, &wg, semaphore, commander)
            wg.Wait() // Ensure all go routines complete
        })
    }
}

func TestFilterString(t *testing.T) {
	testCases := []struct {
		items       []string
		pattern       string
		expected     []string
	}{
		{[]string{"a", "b", "c"}, "a", []string{"a"}},
		{[]string{"aa", "abc", "c"}, "^a", []string{"aa", "abc"}},
		{[]string{"aa", "abc", "c"}, "c$", []string{"abc", "c"}},
		{[]string{"baa", "abc", "cb"}, "b", []string{"baa", "abc", "cb"}},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v, %s", tc.items, tc.pattern), func(t *testing.T) {
			result := filterString(tc.items, tc.pattern)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}