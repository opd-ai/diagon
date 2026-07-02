package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DefinitionOfDoneStatus string

const (
	DefinitionOfDoneStatusPassed  DefinitionOfDoneStatus = "passed"
	DefinitionOfDoneStatusPending DefinitionOfDoneStatus = "pending"
	DefinitionOfDoneStatusFailed  DefinitionOfDoneStatus = "failed"
)

type DefinitionOfDoneCriterion struct {
	ID          string                 `json:"id"`
	Requirement string                 `json:"requirement"`
	Status      DefinitionOfDoneStatus `json:"status"`
	Evidence    []string               `json:"evidence,omitempty"`
	Errors      []string               `json:"errors,omitempty"`
}

type DefinitionOfDoneReport struct {
	GeneratedAt       string                      `json:"generated_at"`
	Environment       string                      `json:"environment"`
	ReadyForNextPhase bool                        `json:"ready_for_next_phase"`
	PassedCriteria    int                         `json:"passed_criteria"`
	PendingCriteria   int                         `json:"pending_criteria"`
	FailedCriteria    int                         `json:"failed_criteria"`
	Criteria          []DefinitionOfDoneCriterion `json:"criteria"`
}

type DefinitionOfDoneOptions struct {
	ProbeLive        bool
	ProbeOptions     RuntimeProbeOptions
	ReleaseBundleDir string
}

func BuildDefinitionOfDoneReport(bootstrap BootstrapProfile, contract ServiceContract, matrix IntegrationMatrix, environmentName string, options DefinitionOfDoneOptions) (DefinitionOfDoneReport, error) {
	environment, err := matrix.EnvironmentByName(environmentName)
	if err != nil {
		return DefinitionOfDoneReport{}, err
	}

	criteria := make([]DefinitionOfDoneCriterion, 0, 5)

	criteria = append(criteria, evaluateDefinitionOfDoneOnboarding(bootstrap, contract, environment))
	criteria = append(criteria, evaluateDefinitionOfDoneServicesAndTunnels(contract, options))
	criteria = append(criteria, evaluateDefinitionOfDoneConfigAndHealth(bootstrap, contract, options))
	criteria = append(criteria, evaluateDefinitionOfDoneCI(matrix, environmentName))
	criteria = append(criteria, evaluateDefinitionOfDoneReleaseArtifacts(options.ReleaseBundleDir))

	report := DefinitionOfDoneReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Environment: strings.TrimSpace(environment.Environment),
		Criteria:    criteria,
	}

	for _, criterion := range report.Criteria {
		switch criterion.Status {
		case DefinitionOfDoneStatusPassed:
			report.PassedCriteria++
		case DefinitionOfDoneStatusPending:
			report.PendingCriteria++
		case DefinitionOfDoneStatusFailed:
			report.FailedCriteria++
		}
	}

	report.ReadyForNextPhase = report.PassedCriteria == len(report.Criteria)
	return report, nil
}

func WriteDefinitionOfDoneReport(path string, report DefinitionOfDoneReport) error {
	return writeJSONFile(path, report, "definition of done report")
}

func evaluateDefinitionOfDoneOnboarding(bootstrap BootstrapProfile, contract ServiceContract, environment IntegrationEnvironment) DefinitionOfDoneCriterion {
	criterion := DefinitionOfDoneCriterion{
		ID:          "dod-1-onboarding",
		Requirement: "A new engineer can provision a supported Debian host and start the full stack using one documented profile and documented secrets input.",
		Evidence: []string{
			"Bootstrap quickstart can be generated from canonical profile/contract inputs.",
			"Required bootstrap secrets are declared with explicit source/ref wiring.",
		},
	}

	guide, err := BuildBootstrapQuickstartGuide(bootstrap, contract, &environment)
	if err != nil {
		criterion.Status = DefinitionOfDoneStatusFailed
		criterion.Errors = []string{err.Error()}
		return criterion
	}

	if strings.TrimSpace(guide) == "" {
		criterion.Status = DefinitionOfDoneStatusFailed
		criterion.Errors = []string{"bootstrap quickstart guide is empty"}
		return criterion
	}

	requiredSecrets := 0
	for _, secret := range bootstrap.Secrets {
		if secret.Required {
			requiredSecrets++
		}
	}

	if requiredSecrets == 0 {
		criterion.Status = DefinitionOfDoneStatusFailed
		criterion.Errors = []string{"bootstrap profile must declare at least one required secret for onboarding handoff"}
		return criterion
	}

	criterion.Status = DefinitionOfDoneStatusPassed
	return criterion
}

func evaluateDefinitionOfDoneServicesAndTunnels(contract ServiceContract, options DefinitionOfDoneOptions) DefinitionOfDoneCriterion {
	criterion := DefinitionOfDoneCriterion{
		ID:          "dod-2-runtime-readiness",
		Requirement: "i2pd runs as a local service, required tunnels are up, and dependent services report ready status within defined timeout.",
		Evidence: []string{
			"Service contract includes i2pd/store/paywall topology and i2pd tunnel mappings.",
			"Live probe verifies service readiness sequencing and tunnel listener reachability.",
		},
	}

	if !options.ProbeLive {
		criterion.Status = DefinitionOfDoneStatusPending
		criterion.Errors = []string{"run with live probes enabled to produce runtime readiness evidence"}
		return criterion
	}

	probeResult := ProbeServiceContractDefinition(contract, options.ProbeOptions)
	if probeResult.HasErrors() {
		criterion.Status = DefinitionOfDoneStatusFailed
		criterion.Errors = append([]string{}, probeResult.Errors...)
		return criterion
	}

	criterion.Status = DefinitionOfDoneStatusPassed
	return criterion
}

func evaluateDefinitionOfDoneConfigAndHealth(bootstrap BootstrapProfile, contract ServiceContract, options DefinitionOfDoneOptions) DefinitionOfDoneCriterion {
	criterion := DefinitionOfDoneCriterion{
		ID:          "dod-3-config-and-health",
		Requirement: "Diagon successfully wires Store and Paywall configs and reports aggregated health accurately.",
		Evidence: []string{
			"Config injection bundle is generated for store/paywall/i2pd without validation errors.",
			"Aggregated health model reports all components ready when live probes succeed.",
		},
	}

	if _, err := BuildInjectedConfigBundle(bootstrap, contract); err != nil {
		criterion.Status = DefinitionOfDoneStatusFailed
		criterion.Errors = []string{err.Error()}
		return criterion
	}

	if !options.ProbeLive {
		criterion.Status = DefinitionOfDoneStatusPending
		criterion.Errors = []string{"run with live probes enabled to verify aggregated health readiness"}
		return criterion
	}

	aggregated := AggregateServiceHealth(contract.Services, options.ProbeOptions)
	if !aggregated.Ready {
		criterion.Status = DefinitionOfDoneStatusFailed
		criterion.Errors = append([]string{}, aggregated.Errors...)
		return criterion
	}

	criterion.Status = DefinitionOfDoneStatusPassed
	return criterion
}

func evaluateDefinitionOfDoneCI(matrix IntegrationMatrix, environmentName string) DefinitionOfDoneCriterion {
	criterion := DefinitionOfDoneCriterion{
		ID:          "dod-4-ci-stages",
		Requirement: "CI demonstrates passing static checks, builds, unit tests, integration tests, end-to-end smoke tests, and Debian packaging verification.",
		Evidence: []string{
			"Release candidate baseline includes merge and release quality gate expectations.",
			"Integration matrix environment exists and yields frozen CI baseline metadata.",
		},
	}

	baseline, err := BuildReleaseCandidateBaseline(matrix, environmentName)
	if err != nil {
		criterion.Status = DefinitionOfDoneStatusFailed
		criterion.Errors = []string{err.Error()}
		return criterion
	}

	if !containsStringValue(baseline.QualityGates, "merge blocked unless Stages 1 through 7 pass") {
		criterion.Status = DefinitionOfDoneStatusFailed
		criterion.Errors = []string{"missing merge quality gate for Stages 1 through 7"}
		return criterion
	}
	if !containsStringValue(baseline.QualityGates, "release blocked unless Stage 8 passes") {
		criterion.Status = DefinitionOfDoneStatusFailed
		criterion.Errors = []string{"missing release quality gate for Stage 8"}
		return criterion
	}

	criterion.Status = DefinitionOfDoneStatusPassed
	return criterion
}

func evaluateDefinitionOfDoneReleaseArtifacts(releaseBundleDir string) DefinitionOfDoneCriterion {
	criterion := DefinitionOfDoneCriterion{
		ID:          "dod-5-release-artifacts",
		Requirement: "Release artifact set includes version manifest, checksums, and operator runbook sufficient for reproducible deployment.",
		Evidence: []string{
			"release bundle includes SHA256SUMS.",
			"release bundle includes version-manifest.json.",
			"release bundle includes operator-runbook.md.",
		},
	}

	trimmedDir := strings.TrimSpace(releaseBundleDir)
	if trimmedDir == "" {
		criterion.Status = DefinitionOfDoneStatusPending
		criterion.Errors = []string{"set a release bundle directory to verify artifact files on disk"}
		return criterion
	}

	requiredFiles := []string{
		"SHA256SUMS",
		"version-manifest.json",
		"operator-runbook.md",
	}

	missing := make([]string, 0)
	for _, name := range requiredFiles {
		if !fileExists(filepath.Join(trimmedDir, name)) && !fileExists(filepath.Join(trimmedDir, "manifest", name)) {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		criterion.Status = DefinitionOfDoneStatusFailed
		criterion.Errors = []string{fmt.Sprintf("release bundle missing required artifact(s): %s", strings.Join(missing, ", "))}
		return criterion
	}

	criterion.Status = DefinitionOfDoneStatusPassed
	return criterion
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func containsStringValue(values []string, target string) bool {
	trimmedTarget := strings.TrimSpace(target)
	for _, value := range values {
		if strings.TrimSpace(value) == trimmedTarget {
			return true
		}
	}
	return false
}
