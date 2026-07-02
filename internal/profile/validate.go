package profile

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Result struct {
	Errors   []string
	Warnings []string
}

type Policy struct {
	RequiredPackages map[string]struct{}
	RequiredPreseed  map[string]string
}

func (r Result) HasErrors() bool {
	return len(r.Errors) > 0
}

func (r *Result) Sort() {
	sort.Strings(r.Errors)
	sort.Strings(r.Warnings)
}

func DefaultPolicy() Policy {
	return Policy{
		RequiredPackages: map[string]struct{}{
			"curl":           {},
			"i2pd":           {},
			"openssh-server": {},
		},
		RequiredPreseed: map[string]string{
			"passwd/root-login": "false",
			"time/zone":         "*non-empty*",
		},
	}
}

func LoadPolicy(path string) (Policy, error) {
	if strings.TrimSpace(path) == "" {
		return Policy{}, errors.New("policy path cannot be empty")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, fmt.Errorf("read policy file %s: %w", path, err)
	}

	type rawPolicy struct {
		RequiredPackages []string          `json:"required_packages"`
		RequiredPreseed  map[string]string `json:"required_preseed"`
	}

	parsed := rawPolicy{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Policy{}, fmt.Errorf("parse policy file %s: %w", path, err)
	}

	policy := Policy{
		RequiredPackages: make(map[string]struct{}),
		RequiredPreseed:  make(map[string]string),
	}

	for _, pkg := range parsed.RequiredPackages {
		trimmed := strings.TrimSpace(pkg)
		if trimmed == "" {
			return Policy{}, fmt.Errorf("policy file %s contains empty package name", path)
		}
		policy.RequiredPackages[trimmed] = struct{}{}
	}

	for key, value := range parsed.RequiredPreseed {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			return Policy{}, fmt.Errorf("policy file %s contains empty preseed key", path)
		}
		policy.RequiredPreseed[trimmedKey] = strings.TrimSpace(value)
	}

	if len(policy.RequiredPackages) == 0 {
		return Policy{}, fmt.Errorf("policy file %s must include at least one required package", path)
	}
	if len(policy.RequiredPreseed) == 0 {
		return Policy{}, fmt.Errorf("policy file %s must include at least one required preseed key", path)
	}

	return policy, nil
}

func Validate(profileDir, profileName string) (Result, error) {
	return ValidateWithPolicy(profileDir, profileName, DefaultPolicy())
}

func ValidateWithPolicy(profileDir, profileName string, policy Policy) (Result, error) {
	if strings.TrimSpace(profileDir) == "" {
		return Result{}, errors.New("profile directory cannot be empty")
	}
	if strings.TrimSpace(profileName) == "" {
		return Result{}, errors.New("profile name cannot be empty")
	}
	if len(policy.RequiredPackages) == 0 {
		return Result{}, errors.New("policy must include at least one required package")
	}
	if len(policy.RequiredPreseed) == 0 {
		return Result{}, errors.New("policy must include at least one required preseed key")
	}

	packagesPath := filepath.Join(profileDir, profileName+".packages")
	preseedPath := filepath.Join(profileDir, profileName+".preseed")

	packages, packageWarnings, err := parsePackages(packagesPath)
	if err != nil {
		return Result{}, err
	}
	preseed, preseedWarnings, err := parsePreseed(preseedPath)
	if err != nil {
		return Result{}, err
	}

	result := Result{}
	result.Warnings = append(result.Warnings, packageWarnings...)
	result.Warnings = append(result.Warnings, preseedWarnings...)

	for pkg := range policy.RequiredPackages {
		if _, ok := packages[pkg]; !ok {
			result.Errors = append(result.Errors, fmt.Sprintf("missing required package %q in %s", pkg, packagesPath))
		}
	}

	for key, expected := range policy.RequiredPreseed {
		value, ok := preseed[key]
		if !ok {
			result.Errors = append(result.Errors, fmt.Sprintf("missing required preseed key %q in %s", key, preseedPath))
			continue
		}

		if expected == "*non-empty*" {
			if strings.TrimSpace(value) == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("preseed key %q must not be empty", key))
			}
			continue
		}

		if value != expected {
			result.Errors = append(result.Errors, fmt.Sprintf("preseed key %q must be %q, got %q", key, expected, value))
		}
	}

	for pkg := range packages {
		if strings.HasPrefix(pkg, "*") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("wildcard package %q detected in %s; prefer explicit package pinning", pkg, packagesPath))
		}
	}

	result.Sort()

	return result, nil
}

func parsePackages(path string) (map[string]struct{}, []string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open packages file %s: %w", path, err)
	}
	defer file.Close()

	packages := make(map[string]struct{})
	warnings := make([]string, 0)

	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, token := range strings.Fields(line) {
			if _, exists := packages[token]; exists {
				warnings = append(warnings, fmt.Sprintf("duplicate package %q in %s at line %d", token, path, lineNo))
			}
			packages[token] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan packages file %s at line %d: %w", path, lineNo, err)
	}

	if len(packages) == 0 {
		return nil, nil, fmt.Errorf("packages file %s contains no package entries", path)
	}

	return packages, warnings, nil
}

func parsePreseed(path string) (map[string]string, []string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open preseed file %s: %w", path, err)
	}
	defer file.Close()

	values := make(map[string]string)
	warnings := make([]string, 0)

	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			return nil, nil, fmt.Errorf("invalid preseed format in %s at line %d: expected at least 4 fields", path, lineNo)
		}
		if fields[0] != "d-i" {
			return nil, nil, fmt.Errorf("invalid preseed format in %s at line %d: expected prefix %q", path, lineNo, "d-i")
		}

		key := fields[1]
		value := strings.Join(fields[3:], " ")
		if _, exists := values[key]; exists {
			warnings = append(warnings, fmt.Sprintf("duplicate preseed key %q in %s at line %d; last value wins", key, path, lineNo))
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan preseed file %s at line %d: %w", path, lineNo, err)
	}

	if len(values) == 0 {
		return nil, nil, fmt.Errorf("preseed file %s contains no settings", path)
	}

	return values, warnings, nil
}
