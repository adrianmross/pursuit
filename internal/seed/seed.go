// Package seed holds a tiny example tracker and role batches.
// The rows are fictional. Applicant contact details come from config.
package seed

import _ "embed"

//go:embed data/applications.csv
var ApplicationsCSV []byte

//go:embed data/batch3.md
var Batch3 []byte

//go:embed data/batch4.md
var Batch4 []byte
