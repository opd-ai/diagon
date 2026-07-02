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
		profileDir      string
		profileName     string
		policyPath      string
		contractPath    string
		bootstrapPath   string
		matrixPath      string
		matrixEnv       string
		configOutPath   string
		debianOutPath   string
		depsOutPath     string
		smokeOutPath    string
		runbookOutPath  string
		quickstartPath  string
		walletOutPath   string
		releaseOutPath  string
		fallbackOutPath string
		probeLive       bool
		probeTimeout    time.Duration
		probeEvery      time.Duration
		strict          bool
		outputFmt       string
	)

	flag.StringVar(&profileDir, "profile-dir", "profiles", "directory containing profile files")
	flag.StringVar(&profileName, "profile-name", "myprofile", "profile filename prefix")
	flag.StringVar(&policyPath, "policy-file", "", "optional JSON policy file for required packages and preseed keys")
	flag.StringVar(&contractPath, "service-contract-file", "", "optional JSON service integration contract file for Store/Paywall/i2pd checks")
	flag.StringVar(&bootstrapPath, "bootstrap-profile-file", "", "optional JSON local bootstrap profile for single-host startup defaults and secrets")
	flag.StringVar(&matrixPath, "integration-matrix-file", "", "optional JSON integration matrix used for release candidate version freezing")
	flag.StringVar(&matrixEnv, "integration-environment", "", "integration matrix environment name for release candidate version freezing")
	flag.StringVar(&configOutPath, "emit-config-injection-file", "", "optional output path for generated Store/Paywall/i2pd injected config bundle (use '-' for stdout)")
	flag.StringVar(&debianOutPath, "emit-debian-package-file", "", "optional output path for generated Debian package baseline bundle (use '-' for stdout)")
	flag.StringVar(&depsOutPath, "emit-debian-dependency-manifest-file", "", "optional output path for generated Debian dependency manifest bundle (use '-' for stdout)")
	flag.StringVar(&smokeOutPath, "emit-release-smoke-file", "", "optional output path for generated Phase 4 release-candidate smoke plan (use '-' for stdout)")
	flag.StringVar(&runbookOutPath, "emit-operator-runbook-file", "", "optional output path for generated operator runbook markdown (use '-' for stdout)")
	flag.StringVar(&quickstartPath, "emit-bootstrap-quickstart-file", "", "optional output path for generated single-host bootstrap quickstart markdown (use '-' for stdout)")
	flag.StringVar(&walletOutPath, "emit-wallet-validation-checklist-file", "", "optional output path for generated production wallet validation checklist markdown (use '-' for stdout)")
	flag.StringVar(&releaseOutPath, "emit-release-baseline-file", "", "optional output path for generated release candidate baseline manifest (use '-' for stdout)")
	flag.StringVar(&fallbackOutPath, "emit-debian-compose-bundle-file", "", "optional output path for generated Debian fallback compose/service bundle (use '-' for stdout)")
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
		integrationMatrix    *profile.IntegrationMatrix
		integrationEnv       *profile.IntegrationEnvironment
		resolvedContractPath string
		loadedContract       *profile.ServiceContract
		aggregatedHealth     *profile.ServiceHealthAggregation
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

	trimmedMatrixPath := strings.TrimSpace(matrixPath)
	trimmedMatrixEnv := strings.TrimSpace(matrixEnv)
	if trimmedMatrixPath != "" {
		matrix, loadErr := profile.LoadIntegrationMatrix(trimmedMatrixPath)
		if loadErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", loadErr), nil)
			os.Exit(2)
		}
		integrationMatrix = &matrix
		if trimmedMatrixEnv != "" {
			env, envErr := matrix.EnvironmentByName(trimmedMatrixEnv)
			if envErr != nil {
				emitFailure(outputFmt, fmt.Errorf("validation error: %w", envErr), nil)
				os.Exit(2)
			}
			integrationEnv = &env
		}
	} else if trimmedMatrixEnv != "" {
		emitFailure(outputFmt, fmt.Errorf("validation error: --integration-environment requires --integration-matrix-file"), nil)
		os.Exit(2)
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
			health := profile.AggregateServiceHealth(contract.Services, profile.RuntimeProbeOptions{
				Timeout:  probeTimeout,
				Interval: probeEvery,
			})
			aggregatedHealth = &health
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

	trimmedConfigOut := strings.TrimSpace(configOutPath)
	if trimmedConfigOut != "" {
		if bootstrapProfile == nil || loadedContract == nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: --emit-config-injection-file requires both bootstrap and service contract inputs"), &result)
			os.Exit(2)
		}

		bundle, buildErr := profile.BuildInjectedConfigBundle(*bootstrapProfile, *loadedContract)
		if buildErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", buildErr), &result)
			os.Exit(2)
		}
		if writeErr := profile.WriteInjectedConfigBundle(trimmedConfigOut, bundle); writeErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", writeErr), &result)
			os.Exit(2)
		}
	}

	trimmedDebianOut := strings.TrimSpace(debianOutPath)
	if trimmedDebianOut != "" {
		if bootstrapProfile == nil || loadedContract == nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: --emit-debian-package-file requires both bootstrap and service contract inputs"), &result)
			os.Exit(2)
		}

		plan, buildErr := profile.BuildDebianPackagePlan(*bootstrapProfile, *loadedContract)
		if buildErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", buildErr), &result)
			os.Exit(2)
		}
		if writeErr := profile.WriteDebianPackagePlan(trimmedDebianOut, plan); writeErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", writeErr), &result)
			os.Exit(2)
		}
	}

	trimmedSmokeOut := strings.TrimSpace(smokeOutPath)
	if trimmedSmokeOut != "" {
		if bootstrapProfile == nil || loadedContract == nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: --emit-release-smoke-file requires both bootstrap and service contract inputs"), &result)
			os.Exit(2)
		}

		plan, buildErr := profile.BuildReleaseCandidateSmokePlan(*bootstrapProfile, *loadedContract)
		if buildErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", buildErr), &result)
			os.Exit(2)
		}
		if writeErr := profile.WriteReleaseCandidateSmokePlan(trimmedSmokeOut, plan); writeErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", writeErr), &result)
			os.Exit(2)
		}
	}

	trimmedRunbookOut := strings.TrimSpace(runbookOutPath)
	if trimmedRunbookOut != "" {
		if bootstrapProfile == nil || loadedContract == nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: --emit-operator-runbook-file requires both bootstrap and service contract inputs"), &result)
			os.Exit(2)
		}

		runbook, buildErr := profile.BuildOperatorRunbook(*bootstrapProfile, *loadedContract, integrationEnv)
		if buildErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", buildErr), &result)
			os.Exit(2)
		}
		if writeErr := profile.WriteOperatorRunbook(trimmedRunbookOut, runbook); writeErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", writeErr), &result)
			os.Exit(2)
		}
	}

	trimmedWalletOut := strings.TrimSpace(walletOutPath)
	if trimmedWalletOut != "" {
		if bootstrapProfile == nil || loadedContract == nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: --emit-wallet-validation-checklist-file requires both bootstrap and service contract inputs"), &result)
			os.Exit(2)
		}

		checklist, buildErr := profile.BuildWalletValidationChecklist(*bootstrapProfile, *loadedContract, integrationEnv)
		if buildErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", buildErr), &result)
			os.Exit(2)
		}
		if writeErr := profile.WriteWalletValidationChecklist(trimmedWalletOut, checklist); writeErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", writeErr), &result)
			os.Exit(2)
		}
	}

	trimmedQuickstartOut := strings.TrimSpace(quickstartPath)
	if trimmedQuickstartOut != "" {
		if bootstrapProfile == nil || loadedContract == nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: --emit-bootstrap-quickstart-file requires both bootstrap and service contract inputs"), &result)
			os.Exit(2)
		}

		guide, buildErr := profile.BuildBootstrapQuickstartGuide(*bootstrapProfile, *loadedContract, integrationEnv)
		if buildErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", buildErr), &result)
			os.Exit(2)
		}
		if writeErr := profile.WriteBootstrapQuickstartGuide(trimmedQuickstartOut, guide); writeErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", writeErr), &result)
			os.Exit(2)
		}
	}

	trimmedReleaseOut := strings.TrimSpace(releaseOutPath)
	if trimmedReleaseOut != "" {
		if integrationMatrix == nil || trimmedMatrixEnv == "" {
			emitFailure(outputFmt, fmt.Errorf("validation error: --emit-release-baseline-file requires both --integration-matrix-file and --integration-environment"), &result)
			os.Exit(2)
		}

		baseline, buildErr := profile.BuildReleaseCandidateBaseline(*integrationMatrix, trimmedMatrixEnv)
		if buildErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", buildErr), &result)
			os.Exit(2)
		}
		if writeErr := profile.WriteReleaseCandidateBaseline(trimmedReleaseOut, baseline); writeErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", writeErr), &result)
			os.Exit(2)
		}
	}

	trimmedDepsOut := strings.TrimSpace(depsOutPath)
	if trimmedDepsOut != "" {
		if integrationMatrix == nil || trimmedMatrixEnv == "" {
			emitFailure(outputFmt, fmt.Errorf("validation error: --emit-debian-dependency-manifest-file requires both --integration-matrix-file and --integration-environment"), &result)
			os.Exit(2)
		}

		manifest, buildErr := profile.BuildDebianDependencyManifest(*integrationMatrix, trimmedMatrixEnv)
		if buildErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", buildErr), &result)
			os.Exit(2)
		}
		if writeErr := profile.WriteDebianDependencyManifest(trimmedDepsOut, manifest); writeErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", writeErr), &result)
			os.Exit(2)
		}
	}

	trimmedFallbackOut := strings.TrimSpace(fallbackOutPath)
	if trimmedFallbackOut != "" {
		if bootstrapProfile == nil || loadedContract == nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: --emit-debian-compose-bundle-file requires both bootstrap and service contract inputs"), &result)
			os.Exit(2)
		}

		bundle, buildErr := profile.BuildDebianComposeServiceBundle(*bootstrapProfile, *loadedContract, integrationEnv)
		if buildErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", buildErr), &result)
			os.Exit(2)
		}
		if writeErr := profile.WriteDebianComposeServiceBundle(trimmedFallbackOut, bundle); writeErr != nil {
			emitFailure(outputFmt, fmt.Errorf("validation error: %w", writeErr), &result)
			os.Exit(2)
		}
	}

	if strings.EqualFold(outputFmt, "json") {
		status := "ok"
		if result.HasErrors() || (strict && len(result.Warnings) > 0) {
			status = "failed"
		}

		payload := struct {
			Status                  string                            `json:"status"`
			ProfileDir              string                            `json:"profile_dir"`
			ProfileName             string                            `json:"profile_name"`
			BootstrapProfileFile    string                            `json:"bootstrap_profile_file,omitempty"`
			ServiceContractFile     string                            `json:"service_contract_file,omitempty"`
			IntegrationMatrixFile   string                            `json:"integration_matrix_file,omitempty"`
			IntegrationEnvironment  string                            `json:"integration_environment,omitempty"`
			ConfigInjectionFile     string                            `json:"config_injection_file,omitempty"`
			DebianPackageFile       string                            `json:"debian_package_file,omitempty"`
			DebianDependencyFile    string                            `json:"debian_dependency_manifest_file,omitempty"`
			ReleaseSmokeFile        string                            `json:"release_smoke_file,omitempty"`
			OperatorRunbookFile     string                            `json:"operator_runbook_file,omitempty"`
			BootstrapQuickstartFile string                            `json:"bootstrap_quickstart_file,omitempty"`
			WalletChecklistFile     string                            `json:"wallet_validation_checklist_file,omitempty"`
			ReleaseBaselineFile     string                            `json:"release_baseline_file,omitempty"`
			DebianComposeBundleFile string                            `json:"debian_compose_bundle_file,omitempty"`
			ProbeLive               bool                              `json:"probe_live"`
			ProbeTimeout            string                            `json:"probe_timeout,omitempty"`
			ProbeInterval           string                            `json:"probe_interval,omitempty"`
			AggregatedHealth        *profile.ServiceHealthAggregation `json:"aggregated_health,omitempty"`
			Strict                  bool                              `json:"strict"`
			Errors                  []string                          `json:"errors"`
			Warnings                []string                          `json:"warnings"`
		}{
			Status:                  status,
			ProfileDir:              profileDir,
			ProfileName:             profileName,
			BootstrapProfileFile:    strings.TrimSpace(bootstrapPath),
			ServiceContractFile:     resolvedContractPath,
			IntegrationMatrixFile:   trimmedMatrixPath,
			IntegrationEnvironment:  trimmedMatrixEnv,
			ConfigInjectionFile:     trimmedConfigOut,
			DebianPackageFile:       trimmedDebianOut,
			DebianDependencyFile:    trimmedDepsOut,
			ReleaseSmokeFile:        trimmedSmokeOut,
			OperatorRunbookFile:     trimmedRunbookOut,
			BootstrapQuickstartFile: trimmedQuickstartOut,
			WalletChecklistFile:     trimmedWalletOut,
			ReleaseBaselineFile:     trimmedReleaseOut,
			DebianComposeBundleFile: trimmedFallbackOut,
			ProbeLive:               probeLive,
			AggregatedHealth:        aggregatedHealth,
			Strict:                  strict,
			Errors:                  result.Errors,
			Warnings:                result.Warnings,
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
