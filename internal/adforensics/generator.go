package adforensics

import (
	"encoding/csv"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"
)

// Dataset is the canonical in-memory representation of the Synthetic Truth set.
// It mirrors the original Python generator in app.py.
//
// Columns:
// - date
// - top_funnel_spend_usd
// - conversion_rate
// - impressions_per_user
// - click_through_rate
// - brand_search_volume
// - campaign_sub_metric_1..46
//
// Total columns: 52.
type Dataset struct {
	Headers []string
	Rows    [][]string // already formatted/rounded strings

	// Numeric series for validation/tests
	Dates       []time.Time
	Spend       []float64
	ConvRate    []float64
	Frequency   []float64
	CTR         []float64
	BrandVolume []float64
	SubMetrics  [][]float64 // [46][rows]
}

type Config struct {
	Rows      int
	Seed      int64
	StartDate time.Time

	// Echo parameters
	EchoLagDays int

	// Cliff parameters
	CliffThreshold float64
}

func DefaultConfig() Config {
	return Config{
		Rows:           500,
		Seed:           42,
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EchoLagDays:    3,
		CliffThreshold: 2000,
	}
}

func Generate(cfg Config) (*Dataset, error) {
	if cfg.Rows <= 0 {
		return nil, fmt.Errorf("rows must be > 0")
	}
	if cfg.EchoLagDays < 0 {
		return nil, fmt.Errorf("echo lag must be >= 0")
	}

	rng := rand.New(rand.NewSource(cfg.Seed))

	dates := make([]time.Time, cfg.Rows)
	for i := 0; i < cfg.Rows; i++ {
		dates[i] = cfg.StartDate.AddDate(0, 0, i)
	}

	// Base Drivers
	spend := make([]float64, cfg.Rows)
	for i := 0; i < cfg.Rows; i++ {
		spend[i] = 500 + rng.Float64()*1500
		wd := dates[i].Weekday()
		if wd == time.Saturday || wd == time.Sunday {
			spend[i] *= 1.4
		}
	}

	frequency := make([]float64, cfg.Rows)
	for i := 0; i < cfg.Rows; i++ {
		frequency[i] = 500 + rng.Float64()*3000
	}

	brandVolume := make([]float64, cfg.Rows)
	for i := 0; i < cfg.Rows; i++ {
		brandVolume[i] = 100 + rng.Float64()*900
	}

	// SIGNAL 1: The Retargeting Echo (lag)
	convRate := make([]float64, cfg.Rows)
	for i := 0; i < cfg.Rows; i++ {
		convRate[i] = 0.02
	}
	for t := cfg.EchoLagDays; t < cfg.Rows; t++ {
		influence := (spend[t-cfg.EchoLagDays] / 2000.0) * 0.03
		noise := rng.NormFloat64() * 0.002
		convRate[t] = 0.02 + influence + noise
	}

	// SIGNAL 2: The Saturation Cliff (threshold)
	ctr := make([]float64, cfg.Rows)
	for t := 0; t < cfg.Rows; t++ {
		if frequency[t] > cfg.CliffThreshold {
			ctr[t] = 0.005 + rng.NormFloat64()*0.001
		} else {
			ctr[t] = 0.04 - (frequency[t] / 100000.0) + rng.NormFloat64()*0.002
		}
	}

	// SIGNAL 3: The Brand Hub (15 correlated sub-metrics)
	sub := make([][]float64, 46)
	for i := 0; i < 46; i++ {
		sub[i] = make([]float64, cfg.Rows)
	}
	for t := 0; t < cfg.Rows; t++ {
		for i := 1; i <= 46; i++ {
			idx := i - 1
			if i <= 15 {
				noise := rng.NormFloat64() * 10
				sub[idx][t] = (brandVolume[t] * 0.6) + noise
			} else {
				sub[idx][t] = rng.Float64() * 1000
			}
		}
	}

	headers := []string{
		"date",
		"top_funnel_spend_usd",
		"conversion_rate",
		"impressions_per_user",
		"click_through_rate",
		"brand_search_volume",
	}
	for i := 1; i <= 46; i++ {
		headers = append(headers, fmt.Sprintf("campaign_sub_metric_%d", i))
	}

	rows := make([][]string, cfg.Rows)
	for t := 0; t < cfg.Rows; t++ {
		r := make([]string, 0, len(headers))
		r = append(r, dates[t].Format("2006-01-02"))
		r = append(r, fToStr(spend[t], 2))
		r = append(r, fToStr(convRate[t], 4))
		r = append(r, fToStr(frequency[t], 2))
		r = append(r, fToStr(ctr[t], 4))
		r = append(r, fToStr(brandVolume[t], 2))
		for i := 0; i < 46; i++ {
			r = append(r, fToStr(sub[i][t], 2))
		}
		rows[t] = r
	}

	return &Dataset{
		Headers:     headers,
		Rows:        rows,
		Dates:       dates,
		Spend:       spend,
		ConvRate:    convRate,
		Frequency:   frequency,
		CTR:         ctr,
		BrandVolume: brandVolume,
		SubMetrics:  sub,
	}, nil
}

func WriteCSV(path string, ds *Dataset) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write(ds.Headers); err != nil {
		return err
	}
	for _, row := range ds.Rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return w.Error()
}

func WriteXLSX(path string, ds *Dataset) error {
	f := excelize.NewFile()

	// Ensure Sheet1 exists and is active.
	sheet := "Sheet1"
	if idx, err := f.GetSheetIndex(sheet); err != nil || idx == -1 {
		idx, err := f.NewSheet(sheet)
		if err != nil {
			return err
		}
		f.SetActiveSheet(idx)
	}

	// Header row
	for i, h := range ds.Headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return err
		}
	}

	// Data rows
	for r := 0; r < len(ds.Rows); r++ {
		rowIdx := r + 2
		for c, v := range ds.Rows[r] {
			cell, _ := excelize.CoordinatesToCellName(c+1, rowIdx)
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return err
			}
		}
	}

	if err := f.SaveAs(path); err != nil {
		return err
	}
	return nil
}

func fToStr(x float64, decimals int) string {
	p := math.Pow10(decimals)
	x = math.Round(x*p) / p
	return strconv.FormatFloat(x, 'f', decimals, 64)
}
