package storage

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/config"
	"github.com/sli/backend/internal/domain"
)

type Storage interface {
	Save(ctx context.Context, fileKey string, data []byte) error
	Get(ctx context.Context, fileKey string) ([]byte, error)
	Delete(ctx context.Context, fileKey string) error
	// GenerateFileKey includes examType because an internship now has up to
	// two marksheets (ISE and ESE) - keying on internshipID alone would let
	// one overwrite the other on disk.
	GenerateFileKey(internshipID uuid.UUID, examType domain.ExamType) string
}

type localStorage struct {
	basePath string
}

func NewLocalStorage(cfg *config.Config) Storage {
	// Ensure base path exists
	_ = os.MkdirAll(cfg.PDFStoragePath, 0755)

	return &localStorage{
		basePath: cfg.PDFStoragePath,
	}
}

func (s *localStorage) Save(ctx context.Context, fileKey string, data []byte) error {
	fullPath := filepath.Join(s.basePath, fileKey)
	return ioutil.WriteFile(fullPath, data, 0644)
}

func (s *localStorage) Get(ctx context.Context, fileKey string) ([]byte, error) {
	fullPath := filepath.Join(s.basePath, fileKey)
	return ioutil.ReadFile(fullPath)
}

func (s *localStorage) Delete(ctx context.Context, fileKey string) error {
	fullPath := filepath.Join(s.basePath, fileKey)
	return os.Remove(fullPath)
}

func (s *localStorage) GenerateFileKey(internshipID uuid.UUID, examType domain.ExamType) string {
	return internshipID.String() + "-" + strings.ToLower(string(examType)) + ".pdf"
}
