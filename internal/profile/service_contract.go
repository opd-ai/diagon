package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	requiredServiceI2PD    = "i2pd"
	requiredServiceStore   = "store"
	requiredServicePaywall = "paywall"
)

type ServiceContract struct {
	Services []ServiceDefinition `json:"services"`
	APILinks []APILink           `json:"api_links"`
}

type ServiceDefinition struct {
	Name         string   `json:"name"`
	Listen       string   `json:"listen"`
	HealthURL    string   `json:"health_url"`
	DependsOn    []string `json:"depends_on"`
	StartupOrder int      `json:"startup_order"`
}

type APILink struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Endpoint string `json:"endpoint"`
}

func LoadServiceContract(path string) (ServiceContract, error) {
	if strings.TrimSpace(path) == "" {
		return ServiceContract{}, errors.New("service contract path cannot be empty")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return ServiceContract{}, fmt.Errorf("read service contract file %s: %w", path, err)
	}

	var contract ServiceContract
	if err := json.Unmarshal(raw, &contract); err != nil {
		return ServiceContract{}, fmt.Errorf("parse service contract file %s: %w", path, err)
	}

	if len(contract.Services) == 0 {
		return ServiceContract{}, fmt.Errorf("service contract file %s must include at least one service", path)
	}

	return contract, nil
}

func ValidateServiceContract(path string) (Result, error) {
	contract, err := LoadServiceContract(path)
	if err != nil {
		return Result{}, err
	}
	return ValidateServiceContractDefinition(contract), nil
}

func ValidateServiceContractDefinition(contract ServiceContract) Result {
	result := Result{}

	if len(contract.Services) == 0 {
		result.Errors = append(result.Errors, "service contract must define at least one service")
		result.Sort()
		return result
	}

	serviceByName := make(map[string]ServiceDefinition, len(contract.Services))
	startupByOrder := make(map[int]string, len(contract.Services))

	for _, service := range contract.Services {
		name := strings.TrimSpace(service.Name)
		if name == "" {
			result.Errors = append(result.Errors, "service contract contains service with empty name")
			continue
		}

		if _, exists := serviceByName[name]; exists {
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate service %q in service contract", name))
			continue
		}

		host, port, listenErr := splitHostPort(service.Listen)
		if listenErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("service %q has invalid listen address %q: %v", name, service.Listen, listenErr))
		}
		if host != "" && !isLoopbackHost(host) {
			result.Errors = append(result.Errors, fmt.Sprintf("service %q listen host %q must be local-only (localhost/127.0.0.1/::1)", name, host))
		}

		healthHost, healthPort, healthErr := validateServiceURL(service.HealthURL)
		if healthErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("service %q has invalid health_url %q: %v", name, service.HealthURL, healthErr))
		} else {
			if !isLoopbackHost(healthHost) {
				result.Errors = append(result.Errors, fmt.Sprintf("service %q health_url host %q must be local-only (localhost/127.0.0.1/::1)", name, healthHost))
			}
			if port != 0 && healthPort != 0 && port != healthPort {
				result.Errors = append(result.Errors, fmt.Sprintf("service %q listen port %d must match health_url port %d", name, port, healthPort))
			}
		}

		if service.StartupOrder <= 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("service %q startup_order must be greater than zero", name))
		} else if existingName, exists := startupByOrder[service.StartupOrder]; exists {
			result.Errors = append(result.Errors, fmt.Sprintf("services %q and %q share startup_order %d", existingName, name, service.StartupOrder))
		} else {
			startupByOrder[service.StartupOrder] = name
		}

		serviceByName[name] = service
	}

	for _, required := range []string{requiredServiceI2PD, requiredServiceStore, requiredServicePaywall} {
		if _, ok := serviceByName[required]; !ok {
			result.Errors = append(result.Errors, fmt.Sprintf("service contract missing required service %q", required))
		}
	}

	if i2pd, ok := serviceByName[requiredServiceI2PD]; ok {
		for _, dependent := range []string{requiredServiceStore, requiredServicePaywall} {
			if svc, exists := serviceByName[dependent]; exists && i2pd.StartupOrder >= svc.StartupOrder {
				result.Errors = append(result.Errors, fmt.Sprintf("service %q must start before %q", requiredServiceI2PD, dependent))
			}
		}
	}

	graph := make(map[string][]string, len(serviceByName))
	for name, service := range serviceByName {
		seenDep := make(map[string]struct{}, len(service.DependsOn))
		for _, dep := range service.DependsOn {
			depName := strings.TrimSpace(dep)
			if depName == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("service %q contains empty dependency", name))
				continue
			}
			if depName == name {
				result.Errors = append(result.Errors, fmt.Sprintf("service %q cannot depend on itself", name))
				continue
			}
			if _, exists := serviceByName[depName]; !exists {
				result.Errors = append(result.Errors, fmt.Sprintf("service %q depends on undefined service %q", name, depName))
				continue
			}
			if _, exists := seenDep[depName]; exists {
				result.Warnings = append(result.Warnings, fmt.Sprintf("service %q lists duplicate dependency %q", name, depName))
				continue
			}
			seenDep[depName] = struct{}{}
			graph[name] = append(graph[name], depName)
		}
	}

	for _, cycle := range findCycles(graph) {
		result.Errors = append(result.Errors, fmt.Sprintf("dependency cycle detected: %s", cycle))
	}

	for _, link := range contract.APILinks {
		from := strings.TrimSpace(link.From)
		to := strings.TrimSpace(link.To)
		if from == "" || to == "" {
			result.Errors = append(result.Errors, "api link entries must include non-empty from and to")
			continue
		}
		fromSvc, fromOK := serviceByName[from]
		if !fromOK {
			result.Errors = append(result.Errors, fmt.Sprintf("api link references undefined source service %q", from))
			continue
		}
		toSvc, toOK := serviceByName[to]
		if !toOK {
			result.Errors = append(result.Errors, fmt.Sprintf("api link references undefined target service %q", to))
			continue
		}

		host, port, endpointErr := validateServiceURL(link.Endpoint)
		if endpointErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("api link %q -> %q has invalid endpoint %q: %v", from, to, link.Endpoint, endpointErr))
			continue
		}
		if !isLoopbackHost(host) {
			result.Errors = append(result.Errors, fmt.Sprintf("api link %q -> %q endpoint host %q must be local-only", from, to, host))
		}

		toHost, toPort, listenErr := splitHostPort(toSvc.Listen)
		if listenErr == nil {
			if !isLoopbackHost(toHost) {
				result.Errors = append(result.Errors, fmt.Sprintf("target service %q listen host %q must be local-only", to, toHost))
			}
			if toPort != 0 && port != 0 && port != toPort {
				result.Errors = append(result.Errors, fmt.Sprintf("api link %q -> %q endpoint port %d must match target listen port %d", from, to, port, toPort))
			}
		}

		if !containsString(fromSvc.DependsOn, to) {
			result.Errors = append(result.Errors, fmt.Sprintf("api link %q -> %q requires %q to list %q in depends_on", from, to, from, to))
		}
	}

	result.Sort()
	return result
}

func splitHostPort(addr string) (string, int, error) {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return "", 0, errors.New("address cannot be empty")
	}

	host, portRaw, err := net.SplitHostPort(trimmed)
	if err != nil {
		return "", 0, err
	}

	if strings.TrimSpace(host) == "" {
		return "", 0, errors.New("host cannot be empty")
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port %q", portRaw)
	}
	if port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("port %d out of range", port)
	}

	return host, port, nil
}

func validateServiceURL(rawURL string) (string, int, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", 0, errors.New("url cannot be empty")
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return "", 0, err
	}
	if !u.IsAbs() {
		return "", 0, errors.New("url must be absolute")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", 0, fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if strings.TrimSpace(u.Host) == "" {
		return "", 0, errors.New("url host cannot be empty")
	}

	host := u.Hostname()
	if strings.TrimSpace(host) == "" {
		return "", 0, errors.New("url host cannot be empty")
	}

	portRaw := u.Port()
	if portRaw == "" {
		return "", 0, errors.New("url port is required")
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port %q", portRaw)
	}
	if port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("port %d out of range", port)
	}

	return host, port, nil
}

func isLoopbackHost(host string) bool {
	normalized := strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	if normalized == "localhost" {
		return true
	}

	ip := net.ParseIP(normalized)
	return ip != nil && ip.IsLoopback()
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == needle {
			return true
		}
	}
	return false
}

func findCycles(graph map[string][]string) []string {
	const (
		stateUnvisited = iota
		stateVisiting
		stateDone
	)

	nodes := make([]string, 0, len(graph))
	for node := range graph {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)

	state := make(map[string]int, len(graph))
	stack := make([]string, 0, len(graph))
	seenCycles := make(map[string]struct{})
	cycles := make([]string, 0)

	var dfs func(string)
	dfs = func(node string) {
		state[node] = stateVisiting
		stack = append(stack, node)

		for _, dep := range graph[node] {
			switch state[dep] {
			case stateUnvisited:
				dfs(dep)
			case stateVisiting:
				start := indexOf(stack, dep)
				if start >= 0 {
					cycle := append([]string{}, stack[start:]...)
					cycle = append(cycle, dep)
					cycleKey := strings.Join(cycle, " -> ")
					if _, exists := seenCycles[cycleKey]; !exists {
						seenCycles[cycleKey] = struct{}{}
						cycles = append(cycles, cycleKey)
					}
				}
			}
		}

		stack = stack[:len(stack)-1]
		state[node] = stateDone
	}

	for _, node := range nodes {
		if state[node] == stateUnvisited {
			dfs(node)
		}
	}

	sort.Strings(cycles)
	return cycles
}

func indexOf(items []string, needle string) int {
	for idx, item := range items {
		if item == needle {
			return idx
		}
	}
	return -1
}
