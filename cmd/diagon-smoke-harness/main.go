// Command diagon-smoke-harness executes the CI end-to-end transaction smoke test
// from an emitted release-candidate smoke plan. It stands up stub origin services
// and i2pd tunnel proxies, drives the marketplace transaction path through a
// graceful restart cycle, writes the smoke result, and validates success criteria.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/opd-ai/diagon/internal/citools"
)

func main() {
	var (
		planPath           string
		outputPath         string
		expectedWalletMode string
	)

	flag.StringVar(&planPath, "plan", "artifacts/stage-6-smoke-plan.json", "path to the emitted release-candidate smoke plan JSON")
	flag.StringVar(&outputPath, "output", "artifacts/stage-6-smoke.json", "path to write the smoke result JSON")
	flag.StringVar(&expectedWalletMode, "expected-wallet-mode", "", "if set, assert the plan wallet_mode matches this value")
	flag.Parse()

	if err := citools.RunSmokeHarness(citools.SmokeHarnessOptions{
		PlanPath:           planPath,
		OutputPath:         outputPath,
		ExpectedWalletMode: expectedWalletMode,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "smoke harness error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("smoke harness passed; result written to %s\n", outputPath)
}
