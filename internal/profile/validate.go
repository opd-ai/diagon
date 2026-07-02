package profile

import (
	"bufio"
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

func (r Result) HasErrors() bool {
	return len(r.Errors) > 0
}

func Validate(profileDir, profileName string) (Result, error) {
	if strings.TrimSpace(profileDir) == "" {
		return Result{}, errors.New("profile directory cannot be empty")
	}
	if strings.TrimSpace(profileName) == "" {
		return Result{}, errors.New("profile name cannot be empty")
	}

	packagesPath := filepath.Join(profileDir, profileName+".packages")
	preseedPath := filepath.Join(profileDir, profileName+".preseed")

	packages, err := parsePackages(packagesPath)
	if err != nil {
		return Result{}, err
	}
	preseed, err := parsePreseed(preseedPath)
	if err != nil {
		return Result{}, err
	}

	result := Result{}

	requiredPackages := []string{"i2pd", "curl", "openssh-server"}
	for _, pkg := range requiredPackages {
		if _, ok := packages[pkg]; !ok {
			result.Errors = append(result.Errors, fmt.Sprintf("missing required package %q in %s", pkg, packagesPath))
		}
	}

	if value, ok := preseed["passwd/root-login"]; !ok {
		result.Errors = append(result.Errors, fmt.Sprintf("missing required preseed key %q in %s", "passwd/root-login", preseedPath))
	} else if value != "false" {
		result.Errors = append(result.Errors, fmt.Sprintf("preseed key %q must be %q, got %q", "passwd/root-login", "false", value))
	}

	if value, ok := preseed["time/zone"]; !ok {
		result.Errors = append(result.Errors, fmt.Sprintf("missing required preseed key %q in %s", "time/zone", preseedPath))
	} else if strings.TrimSpace(value) == "" {
		result.Errors = append(result.Errors, fmt.Sprintf("preseed key %q must not be empty", "time/zone"))
	}

	for pkg := range packages {
		if strings.HasPrefix(pkg, "*") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("wildcard package %q detected in %s; prefer explicit package pinning", pkg, packagesPath))
		}
	}
	sort.Strings(result.Warnings)

	return result, nil
}

func parsePackages(path string) (map[string]struct{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open packages file %s: %w", path, err)
	}
	defer file.Close()

	packages := make(map[string]struct{})

	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, token := range strings.Fields(line) {
			packages[token] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan packages file %s at line %d: %w", path, lineNo, err)
	}

	if len(packages) == 0 {
		return nil, fmt.Errorf("packages file %s contains no package entries", path)
	}

	return packages, nil
}

func parsePreseed(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open preseed file %s: %w", path, err)
	}
	defer file.Close()

	values := make(map[string]string)

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
			return nil, fmt.Errorf("invalid preseed format in %s at line %d: expected at least 4 fields", path, lineNo)
		}
		if fields[0] != "d-i" {
			return nil, fmt.Errorf("invalid preseed format in %s at line %d: expected prefix %q", path, lineNo, "d-i")
		}

		key := fields[1]
		value := strings.Join(fields[3:], " ")
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan preseed file %s at line %d: %w", path, lineNo, err)
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("preseed file %s contains no settings", path)
	}

	return values, nil
}
