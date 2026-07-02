package profile

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type RuntimeProbeOptions struct {
	Timeout        time.Duration
	Interval       time.Duration
	ConnectTimeout time.Duration
	HTTPTimeout    time.Duration
	SequenceJitter time.Duration
}

func DefaultRuntimeProbeOptions() RuntimeProbeOptions {
	return RuntimeProbeOptions{
		Timeout:        30 * time.Second,
		Interval:       500 * time.Millisecond,
		ConnectTimeout: 1 * time.Second,
		HTTPTimeout:    2 * time.Second,
		SequenceJitter: 50 * time.Millisecond,
	}
}

func (o RuntimeProbeOptions) normalize() RuntimeProbeOptions {
	defaults := DefaultRuntimeProbeOptions()
	if o.Timeout <= 0 {
		o.Timeout = defaults.Timeout
	}
	if o.Interval <= 0 {
		o.Interval = defaults.Interval
	}
	if o.ConnectTimeout <= 0 {
		o.ConnectTimeout = defaults.ConnectTimeout
	}
	if o.HTTPTimeout <= 0 {
		o.HTTPTimeout = defaults.HTTPTimeout
	}
	if o.SequenceJitter <= 0 {
		o.SequenceJitter = maxDuration(defaults.SequenceJitter, 2*o.Interval)
	}
	return o
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func ProbeServiceContract(path string, options RuntimeProbeOptions) (Result, error) {
	contract, err := LoadServiceContract(path)
	if err != nil {
		return Result{}, err
	}

	return ProbeServiceContractDefinition(contract, options), nil
}

func ProbeServiceContractDefinition(contract ServiceContract, options RuntimeProbeOptions) Result {
	result := ValidateServiceContractDefinition(contract)
	if result.HasErrors() {
		return result
	}

	options = options.normalize()
	probeResult := probeServiceReadiness(contract.Services, options)
	result.Errors = append(result.Errors, probeResult.Errors...)
	result.Warnings = append(result.Warnings, probeResult.Warnings...)
	result.Sort()
	return result
}

type serviceProbeOutcome struct {
	readyAt time.Time
	err     error
}

func probeServiceReadiness(services []ServiceDefinition, options RuntimeProbeOptions) Result {
	result := Result{}
	if len(services) == 0 {
		result.Errors = append(result.Errors, "service contract must define at least one service")
		result.Sort()
		return result
	}

	start := time.Now()
	deadline := start.Add(options.Timeout)

	outcomes := make(map[string]serviceProbeOutcome, len(services))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, service := range services {
		svc := service
		wg.Add(1)
		go func() {
			defer wg.Done()
			readyAt, err := waitForServiceReady(svc, options, deadline)
			mu.Lock()
			outcomes[svc.Name] = serviceProbeOutcome{readyAt: readyAt, err: err}
			mu.Unlock()
		}()
	}

	wg.Wait()

	for _, service := range services {
		outcome := outcomes[service.Name]
		if outcome.err != nil {
			result.Errors = append(result.Errors, outcome.err.Error())
		}
	}

	if result.HasErrors() {
		result.Sort()
		return result
	}

	for _, service := range services {
		serviceOutcome := outcomes[service.Name]
		for _, dep := range service.DependsOn {
			depName := strings.TrimSpace(dep)
			if depName == "" {
				continue
			}
			depOutcome, ok := outcomes[depName]
			if !ok || depOutcome.readyAt.IsZero() || serviceOutcome.readyAt.IsZero() {
				continue
			}
			if depOutcome.readyAt.After(serviceOutcome.readyAt.Add(options.SequenceJitter)) {
				result.Errors = append(result.Errors, fmt.Sprintf("service %q reported readiness before dependency %q (service=%s dependency=%s)", service.Name, depName, serviceOutcome.readyAt.Format(time.RFC3339Nano), depOutcome.readyAt.Format(time.RFC3339Nano)))
			}
		}
	}

	sorted := append([]ServiceDefinition{}, services...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].StartupOrder == sorted[j].StartupOrder {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].StartupOrder < sorted[j].StartupOrder
	})

	for idx := 1; idx < len(sorted); idx++ {
		prev := sorted[idx-1]
		curr := sorted[idx]
		prevReady := outcomes[prev.Name].readyAt
		currReady := outcomes[curr.Name].readyAt
		if !prevReady.IsZero() && !currReady.IsZero() && currReady.Add(options.SequenceJitter).Before(prevReady) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("service %q became ready before lower startup_order service %q (orders %d -> %d)", curr.Name, prev.Name, prev.StartupOrder, curr.StartupOrder))
		}
	}

	result.Sort()
	return result
}

func waitForServiceReady(service ServiceDefinition, options RuntimeProbeOptions, deadline time.Time) (time.Time, error) {
	var lastErr error

	for {
		now := time.Now()
		if now.After(deadline) {
			if lastErr == nil {
				lastErr = fmt.Errorf("service %q did not become ready before timeout %s", service.Name, options.Timeout)
			}
			return time.Time{}, fmt.Errorf("service %q failed runtime readiness probe within %s: %v", service.Name, options.Timeout, lastErr)
		}

		if err := probeServiceListen(service.Listen, options.ConnectTimeout); err != nil {
			lastErr = err
		} else if err := probeServiceHealth(service.HealthURL, options.HTTPTimeout); err != nil {
			lastErr = err
		} else {
			return time.Now(), nil
		}

		time.Sleep(options.Interval)
	}
}

func probeServiceListen(addr string, timeout time.Duration) error {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return fmt.Errorf("listen probe %q failed: %w", addr, err)
	}
	_ = conn.Close()
	return nil
}

func probeServiceHealth(rawURL string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("build health request %q: %w", rawURL, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("health probe %q failed: %w", rawURL, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("health probe %q returned non-ready status %d", rawURL, resp.StatusCode)
	}

	return nil
}
