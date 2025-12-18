package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gohypo/internal/adforensics"
)

func main() {
	out := flag.String("out", "ad_performance_forensics.xlsx", "output file path")
	rows := flag.Int("rows", 500, "number of rows (days)")
	format := flag.String("format", "", "output format: xlsx or csv (default inferred from -out)")
	seed := flag.Int64("seed", 42, "RNG seed (deterministic)")
	start := flag.String("start", "2025-01-01", "start date (YYYY-MM-DD)")
	flag.Parse()

	if *rows <= 0 {
		fmt.Fprintln(os.Stderr, "rows must be > 0")
		os.Exit(2)
	}

	startDate, err := time.ParseInLocation("2006-01-02", *start, time.UTC)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid -start (expected YYYY-MM-DD):", err)
		os.Exit(2)
	}

	fmtName := strings.ToLower(strings.TrimSpace(*format))
	if fmtName == "" {
		ext := strings.ToLower(filepath.Ext(*out))
		switch ext {
		case ".xlsx":
			fmtName = "xlsx"
		case ".csv":
			fmtName = "csv"
		default:
			fmtName = "xlsx"
		}
	}

	cfg := adforensics.DefaultConfig()
	cfg.Rows = *rows
	cfg.Seed = *seed
	cfg.StartDate = startDate

	ds, err := adforensics.Generate(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error generating dataset:", err)
		os.Exit(1)
	}

	switch fmtName {
	case "csv":
		if err := adforensics.WriteCSV(*out, ds); err != nil {
			fmt.Fprintln(os.Stderr, "error writing csv:", err)
			os.Exit(1)
		}
	case "xlsx":
		if err := adforensics.WriteXLSX(*out, ds); err != nil {
			fmt.Fprintln(os.Stderr, "error writing xlsx:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "unsupported format:", fmtName)
		os.Exit(2)
	}

	fmt.Printf("Elite Gold Standard Created: %s\n", *out)
	fmt.Printf("Total Columns: %d | Total Rows: %d\n", len(ds.Headers), len(ds.Rows))
}
