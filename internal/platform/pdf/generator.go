package pdf

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/sli/backend/internal/domain"
)

//go:embed templates
var templatesFS embed.FS

const templateFileName = "marksheet.html"

type Generator interface {
	GenerateMarksheet(ctx context.Context, internship *domain.Internship, schedule *domain.EvaluationSchedule, score *domain.EvaluationScore, faculty *domain.User) ([]byte, error)
}

// MarksheetData is everything internal/platform/pdf/templates/marksheet.html
// can reference via {{.FieldName}}. See templates/README.md for the full
// field list.
type MarksheetData struct {
	StudentName         string
	StudentEmail        string
	DepartmentName      string
	CompanyName         string
	RoleTitle           string
	FacultyMentorName   string
	ExamType            string
	EvaluationDate      string
	Venue               string
	ReportQuality       int
	OralPresentation    int
	WorkQuality         int
	Understanding       int
	PeriodicInteraction int
	// ...Bucket is which column ("excellent", "good", "average",
	// "satisfactory") the marksheet template should place this row's bullet
	// point in - computed from the score as a percentage of its own max.
	ReportQualityBucket       string
	OralPresentationBucket    string
	WorkQualityBucket         string
	UnderstandingBucket       string
	PeriodicInteractionBucket string
	TotalMarks                int
	Remarks                   string
	GeneratedAt               string
}

// Max points for each rubric category - mirrors the CHECK constraints on
// domain.EvaluationScore (internal/domain/evaluation.go).
const (
	maxReportQuality       = 20
	maxOralPresentation    = 30
	maxWorkQuality         = 15
	maxUnderstanding       = 15
	maxPeriodicInteraction = 20
)

// bucketFor maps a score/max ratio to the marksheet's four qualitative
// columns: Excellent (>=90%), Good (70-89%), Average (50-69%), Satisfactory
// (<50%).
func bucketFor(score, max int) string {
	if max <= 0 {
		return "satisfactory"
	}
	pct := float64(score) / float64(max) * 100
	switch {
	case pct >= 90:
		return "excellent"
	case pct >= 70:
		return "good"
	case pct >= 50:
		return "average"
	default:
		return "satisfactory"
	}
}

type chromeGenerator struct {
	// execPath overrides which Chrome/Chromium binary chromedp launches.
	// Empty means chromedp auto-detects one on $PATH (chromium,
	// chromium-browser, google-chrome, ...). Set CHROME_EXEC_PATH if the
	// binary in your environment has a name chromedp doesn't already know.
	execPath string
}

func NewPDFGenerator() Generator {
	return &chromeGenerator{execPath: os.Getenv("CHROME_EXEC_PATH")}
}

func (g *chromeGenerator) GenerateMarksheet(ctx context.Context, internship *domain.Internship, schedule *domain.EvaluationSchedule, score *domain.EvaluationScore, faculty *domain.User) ([]byte, error) {
	data := MarksheetData{
		StudentName:         internship.Student.Name,
		StudentEmail:        internship.Student.Email,
		DepartmentName:      internship.Student.Department,
		CompanyName:         internship.CompanyName,
		RoleTitle:           internship.RoleTitle,
		FacultyMentorName:   faculty.Name,
		ExamType:            string(score.ExamType),
		EvaluationDate:      schedule.EndSemesterAt.Format("2 January 2006"),
		Venue:               schedule.Venue,
		ReportQuality:             score.ReportQuality,
		OralPresentation:          score.OralPresentation,
		WorkQuality:               score.WorkQuality,
		Understanding:             score.Understanding,
		PeriodicInteraction:       score.PeriodicInteraction,
		ReportQualityBucket:       bucketFor(score.ReportQuality, maxReportQuality),
		OralPresentationBucket:    bucketFor(score.OralPresentation, maxOralPresentation),
		WorkQualityBucket:         bucketFor(score.WorkQuality, maxWorkQuality),
		UnderstandingBucket:       bucketFor(score.Understanding, maxUnderstanding),
		PeriodicInteractionBucket: bucketFor(score.PeriodicInteraction, maxPeriodicInteraction),
		TotalMarks:                score.ReportQuality + score.OralPresentation + score.WorkQuality + score.Understanding + score.PeriodicInteraction,
		Remarks:                   score.Remarks,
		GeneratedAt:               time.Now().Format("2 January 2006, 3:04 PM"),
	}

	renderDir, err := stageTemplate(data)
	if err != nil {
		return nil, fmt.Errorf("staging marksheet template: %w", err)
	}
	defer os.RemoveAll(renderDir)

	return renderPDF(ctx, filepath.Join(renderDir, templateFileName), g.execPath)
}

// stageTemplate fills templates/marksheet.html with data and writes the
// result, plus every other embedded file under templates/ (logos and any
// other assets), into a fresh temp directory - so relative <img src="...">
// paths in the template resolve when Chrome loads it from disk.
func stageTemplate(data MarksheetData) (string, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/"+templateFileName)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	dir, err := os.MkdirTemp("", "marksheet-*")
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("executing template: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, templateFileName), buf.Bytes(), 0644); err != nil {
		os.RemoveAll(dir)
		return "", err
	}

	err = fs.WalkDir(templatesFS, "templates", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := filepath.Base(path)
		if d.IsDir() || name == templateFileName || name == "README.md" {
			return nil
		}
		content, readErr := templatesFS.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(filepath.Join(dir, name), content, 0644)
	})
	if err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("copying template assets: %w", err)
	}

	return dir, nil
}

// renderPDF drives a headless Chrome/Chromium instance to load the staged
// HTML file and print it to PDF - the same output you'd get from Chrome's
// own print-to-PDF, so full CSS (flexbox, grid, @page rules) is supported.
func renderPDF(ctx context.Context, htmlPath string, execPath string) ([]byte, error) {
	opts := append(append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...),
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),            // required to run Chrome as a container's non-root user
		chromedp.Flag("disable-dev-shm-usage", true), // /dev/shm is tiny in most containers; avoids Chrome crashing
		chromedp.Flag("disable-gpu", true),
	)
	if execPath != "" {
		opts = append(opts, chromedp.ExecPath(execPath))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	renderCtx, cancelTimeout := context.WithTimeout(browserCtx, 20*time.Second)
	defer cancelTimeout()

	var pdfBytes []byte
	err := chromedp.Run(renderCtx,
		chromedp.Navigate("file://"+htmlPath),
		chromedp.WaitReady("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			buf, _, printErr := page.PrintToPDF().
				WithPrintBackground(true).
				WithPreferCSSPageSize(true).
				Do(ctx)
			if printErr != nil {
				return printErr
			}
			pdfBytes = buf
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("rendering PDF via headless chrome: %w", err)
	}
	return pdfBytes, nil
}
