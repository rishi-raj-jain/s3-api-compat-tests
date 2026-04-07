package tests

import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Env holds the shared state for all tests against a single provider.
type Env struct {
	Client        *s3.Client
	PresignClient *s3.PresignClient
	Bucket        string
	Provider      string
	Region        string
}

// TestDef defines a single test to run.
type TestDef struct {
	Name     string
	Category string
	Fn       TestFunc
}

// TestResult records the outcome of a test. Operation, Category, and DurationMs
// are filled in by the runner; the test function sets Status, Error, and Notes.
type TestResult struct {
	Operation  string `json:"operation"`
	Category   string `json:"category"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

const (
	StatusPass  = "pass"
	StatusFail  = "fail"
	StatusSkip  = "skip"
	StatusError = "error"
)

func Pass() TestResult                    { return TestResult{Status: StatusPass} }
func Fail(err string) TestResult          { return TestResult{Status: StatusFail, Error: err} }
func Skip(reason string) TestResult       { return TestResult{Status: StatusSkip, Notes: reason} }
func Error(err string) TestResult         { return TestResult{Status: StatusError, Error: err} }
func PassWithNote(note string) TestResult { return TestResult{Status: StatusPass, Notes: note} }
