package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/opd-ai/diagon/internal/profile"
)

func main() {
	var (
		profileDir  string
		profileName string
	)

	flag.StringVar(&profileDir, "profile-dir", "profiles", "directory containing profile files")
	flag.StringVar(&profileName, "profile-name", "myprofile", "profile filename prefix")
	flag.Parse()

	result, err := profile.Validate(profileDir, profileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validation error: %v\n", err)
		os.Exit(2)
	}

	for _, warning := range result.Warnings {
		fmt.Printf("WARN: %s\n", warning)
	}

	if result.HasErrors() {
		for _, validationError := range result.Errors {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", validationError)
		}
		os.Exit(1)
	}

	fmt.Printf("Profile %q in %q validated successfully\n", profileName, profileDir)
}
