package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opd-ai/diagon/internal/profile"
)

func main() {
	var (
		profileDir    string
		profileName   string
		policyPath    string
		contractPath  string
		bootstrapPath string
		probeLive     bool
		probeTimeout  time.Duration
		probeEvery    time.Duration
		strict        bool
		outputFmt     string
	)

	flag.StringVar(&profileDir, "profile-dir", "profiles", "directory containing profile files")
	flag.StringVar(&profileName, "profile-name", "myprofile", "profile filename prefix")
	flag.StringVar(&policyPath, "policy-file", "", "optional JSON policy file for required packages and preseed keys")
	flag.StringVar(&contractPath, "service-contract-file", "", "optional JSON service integration contract file for Store/Paywall/i2pd checks")
	flag.StringVar(&bootstrapPath, "bootstrap-profile-file", "", "optional JSON local bootstrap profile for single-host startup defaults and secrets")
	flag.BoolVar(&probeLive, "probe-live", false, "actively probe service health/listen endpoints from service contract")
	flag.DurationVar(&probeTimeout, "probe-timeout", 30*time.Second, "max time to wait for live service probes")
	flag.DurationVar(&probeEvery, "probe-interval", 500*time.Millisecond, "retry interval for live service probes")
	flag.BoolVar(&strict, "strict", false, "treat warnings as validation errors")
	flag.StringVar(&outputFmt, "format", "text", "output format: text or json")
	flag.Parse()

	result := profile.Result{}
	var err error

	if strings.EqualFold(strings.TrimSpace(policyPath), "") {
		result, err = profile.Validate(profileDir, profileName)
	} else {
		policy, policyErr := profile.LoadPolicy(policyPath)
		if policyErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", policyErr), nil)
			os.Exit(2)
		}
		result, err = profile.ValidateWithPolicy(profileDir, profileName, policy)
	}
	if err != nil {
		emitFailure(outputFmt, fmt.Errorf("validation error: %w", err), nil)
		os.Exit(2)
	}

	var (
		bootstrapProfile     *profile.BootstrapProfile
		resolvedContractPath string
		loadedContract       *profile.ServiceContract
	)

	if !strings.EqualFold(strings.TrimSpace(bootstrapPath), "") {
		bootstrap, loadErr := profile.LoadBootstrapProfile(bootstrapPath)
		if loadErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", loadErr), nil)
			os.Exit(2)
		}
		bootstrapProfile = &bootstrap
		resolvedContractPath = strings.TrimSpace(contractPath)
		if resolvedContractPath == "" {
			resolvedContractPath = resolveBootstrapContractPath(bootstrapPath, bootstrap.ServiceContractFile)
		}
	} else {
		resolvedContractPath = strings.TrimSpace(contractPath)
	}

	if probeLive && resolvedContractPath == "" {
		emitFailure(outputFmt, fmt.Errorf("validation error: --probe-live requires --service-contract-file or a bootstrap profile with service_contract_file"), nil)
		os.Exit(2)
	}

	if resolvedContractPath != "" {
		contract, loadErr := profile.LoadServiceContract(resolvedContractPath)
		if loadErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", loadErr), nil)
			os.Exit(2)
		}
		loadedContract = &contract

		serviceResult := profile.ValidateServiceContractDefinition(contract)
		if probeLive {
			serviceResult = profile.ProbeServiceContractDefinition(contract, profile.RuntimeProbeOptions{
				Timeout:  probeTimeout,
				Interval: probeEvery,
			})
		}

		if serviceResult.HasErrors() {
			result.Errors = append(result.Errors, serviceResult.Errors...)
		}
		if len(serviceResult.Warnings) > 0 {
			result.Warnings = append(result.Warnings, serviceResult.Warnings...)
		}
		result.Sort()
	}

	if bootstrapProfile != nil {
		bootstrapResult := profile.ValidateBootstrapProfileDefinition(*bootstrapProfile, loadedContract)
		if bootstrapResult.HasErrors() {
			result.Errors = append(result.Errors, bootstrapResult.Errors...)
		}
		if len(bootstrapResult.Warnings) > 0 {
			result.Warnings = append(result.Warnings, bootstrapResult.Warnings...)
		}
		result.Sort()
	}

	if strings.EqualFold(outputFmt, "json") {
		status := "ok"
		if result.HasErrors() || (strict && len(result.Warnings) > 0) {
			status = "failed"
		}

		payload := struct {
			Status               string   `json:"status"`
			ProfileDir           string   `json:"profile_dir"`
			ProfileName          string   `json:"profile_name"`
			BootstrapProfileFile string   `json:"bootstrap_profile_file,omitempty"`
			ServiceContractFile  string   `json:"service_contract_file,omitempty"`
			ProbeLive            bool     `json:"probe_live"`
			ProbeTimeout         string   `json:"probe_timeout,omitempty"`
			ProbeInterval        string   `json:"probe_interval,omitempty"`
			Strict               bool     `json:"strict"`
			Errors               []string `json:"errors"`
			Warnings             []string `json:"warnings"`
		}{
			Status:               status,
			ProfileDir:           profileDir,
			ProfileName:          profileName,
			BootstrapProfileFile: strings.TrimSpace(bootstrapPath),
			ServiceContractFile:  resolvedContractPath,
			ProbeLive:            probeLive,
			Strict:               strict,
			Errors:               result.Errors,
			Warnings:             result.Warnings,
		}
		if probeLive {
			payload.ProbeTimeout = probeTimeout.String()
			payload.ProbeInterval = probeEvery.String()
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if encodeErr := enc.Encode(payload); encodeErr != nil {
			fmt.Fprintf(os.Stderr, "validation error: encode json: %v\n", encodeErr)
			os.Exit(2)
		}

		if status == "failed" {
			os.Exit(1)
		}
		return
	}

	if !strings.EqualFold(outputFmt, "text") {
		fmt.Fprintf(os.Stderr, "validation error: unsupported format %q (use text or json)\n", outputFmt)
		os.Exit(2)
	}

	for _, warning := range result.Warnings {
		fmt.Printf("WARN: %s\n", warning)
	}

	if strict && len(result.Warnings) > 0 {
		for _, warning := range result.Warnings {
			fmt.Fprintf(os.Stderr, "ERROR: strict mode enabled; warning escalated: %s\n", warning)
		}
		os.Exit(1)
	}

	if result.HasErrors() {
		for _, validationError := range result.Errors {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", validationError)
		}
		os.Exit(1)
	}

	fmt.Printf("Profile %q in %q validated successfully\n", profileName, profileDir)
}

func emitFailure(outputFmt string, err error, result *profile.Result) {
	if strings.EqualFold(outputFmt, "json") {
		payload := struct {
			Status   string   `json:"status"`
			Errors   []string `json:"errors"`
			Warnings []string `json:"warnings,omitempty"`
		}{
			Status: "failed",
			Errors: []string{err.Error()},
		}
		if result != nil {
			payload.Warnings = result.Warnings
			payload.Errors = append(payload.Errors, result.Errors...)
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if encodeErr := enc.Encode(payload); encodeErr != nil {
			fmt.Fprintf(os.Stderr, "validation error: %v\n", err)
			fmt.Fprintf(os.Stderr, "validation error: encode json: %v\n", encodeErr)
		}
		return
	}

	fmt.Fprintf(os.Stderr, "%v\n", err)
}

func resolveBootstrapContractPath(bootstrapPath, contractPath string) string {
	trimmedContractPath := strings.TrimSpace(contractPath)
	if trimmedContractPath == "" {
		return ""
	}
	if filepath.IsAbs(trimmedContractPath) {
		return trimmedContractPath
	}
	return filepath.Clean(filepath.Join(filepath.Dir(bootstrapPath), trimmedContractPath))
}
