package main

import (
	"time"

	"github.com/tigrisdata-community/s3-api-compat-tests/tests"
)

// Report is the top-level JSON output.
type Report struct {
	GeneratedAt time.Time        `json:"generated_at"`
	Providers   []ProviderReport `json:"providers"`
}

// ProviderReport holds all results for a single provider.
type ProviderReport struct {
	Name     string             `json:"name"`
	Endpoint string             `json:"endpoint"`
	Region   string             `json:"region"`
	Total    int                `json:"total"`
	Passed   int                `json:"passed"`
	Failed   int                `json:"failed"`
	Skipped  int                `json:"skipped"`
	Errors   int                `json:"errors"`
	Results  []tests.TestResult `json:"results"`
}

// Tally computes the summary counts from Results.
func (pr *ProviderReport) Tally() {
	pr.Total = len(pr.Results)
	pr.Passed = 0
	pr.Failed = 0
	pr.Skipped = 0
	pr.Errors = 0
	for _, r := range pr.Results {
		switch r.Status {
		case tests.StatusPass:
			pr.Passed++
		case tests.StatusFail:
			pr.Failed++
		case tests.StatusSkip:
			pr.Skipped++
		case tests.StatusError:
			pr.Errors++
		}
	}
}
