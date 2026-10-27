package service

import (
	"bytes"
	"context"

	"github.com/pursuit/pursuit/internal/impex"
	"github.com/pursuit/pursuit/internal/seed"
)

// ImportSeed loads the example CSV, then any batch roles whose posting URL
// is not already present. New batch rows are to_apply.
func (s *Service) ImportSeed(ctx context.Context) (ImportResult, error) {
	csvResult, err := s.ImportCSV(ctx, bytes.NewReader(seed.ApplicationsCSV))
	if err != nil {
		return csvResult, err
	}
	var extra []impex.Posting
	for _, blob := range [][]byte{seed.Batch3, seed.Batch4} {
		rows, err := impex.ParseMarkdownTables(bytes.NewReader(blob))
		if err != nil {
			return csvResult, err
		}
		extra = append(extra, rows...)
	}
	md, err := s.ImportPostings(ctx, extra)
	if err != nil {
		return csvResult, err
	}
	csvResult.Rows += md.Rows
	csvResult.CompaniesCreated += md.CompaniesCreated
	csvResult.CompaniesExisting += md.CompaniesExisting
	csvResult.RolesCreated += md.RolesCreated
	csvResult.ApplicationsCreated += md.ApplicationsCreated
	csvResult.Skipped += md.Skipped
	csvResult.Warnings = append(csvResult.Warnings, md.Warnings...)
	return csvResult, nil
}
