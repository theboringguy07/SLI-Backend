package pdf

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
)

// findChrome mirrors chromedp's own auto-detection list. Returns "" if none
// are on $PATH, in which case the test skips - this generator needs a real
// headless Chrome/Chromium binary, which isn't guaranteed to be installed
// wherever `go test` runs (see .github/workflows/ci.yml, which installs
// Chromium specifically so this test isn't skipped there).
func findChrome() string {
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func TestGenerateMarksheetReturnsPDFBytes(t *testing.T) {
	if findChrome() == "" {
		t.Skip("no chrome/chromium binary on PATH; install one to run this test (see .github/workflows/ci.yml)")
	}

	gen := NewPDFGenerator()
	internship := &domain.Internship{
		ID:          uuid.New(),
		CompanyName: "Acme Labs",
		RoleTitle:   "SDE Intern",
		Student: &domain.User{
			Name:  "Test Student",
			Email: "student@somaiya.edu",
		},
	}
	schedule := &domain.EvaluationSchedule{
		EndSemesterAt: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
		Venue:         "Seminar Hall",
	}
	score := &domain.EvaluationScore{
		ReportQuality:       18,
		OralPresentation:    27,
		WorkQuality:         14,
		Understanding:       13,
		PeriodicInteraction: 19,
		Remarks:             "Strong performance",
		SubmittedAt:         time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC),
	}
	faculty := &domain.User{Name: "Faculty Mentor"}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	content, err := gen.GenerateMarksheet(ctx, internship, schedule, score, faculty)
	if err != nil {
		t.Fatalf("GenerateMarksheet returned error: %v", err)
	}
	if !bytes.HasPrefix(content, []byte("%PDF-")) {
		t.Fatalf("expected PDF header, got %q", content[:min(8, len(content))])
	}
	if len(content) < 500 {
		t.Fatalf("expected a non-trivial rendered PDF, got only %d bytes", len(content))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
