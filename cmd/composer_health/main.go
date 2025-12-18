package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gohypo/adapters/stats/senses"
	"gohypo/domain/core"

	"github.com/xuri/excelize/v2"
)

type dataset struct {
	Headers []string
	Rows    int
	// Numeric columns only (date/entity columns excluded)
	VarNames []string
	Cols     map[string][]float64

	DateColumn string
	Dates      []time.Time
}

type senseAgg struct {
	SenseName  string
	Weak       int
	Moderate   int
	Strong     int
	VeryStrong int
}

type eliteHit struct {
	X        string
	Y        string
	Sense    string
	Effect   float64
	PValue   float64
	Signal   string
	Metadata map[string]interface{}
}

func main() {
	in := flag.String("in", "", "input dataset path (.xlsx or .csv)")
	sheet := flag.String("sheet", "Sheet1", "xlsx sheet name")
	topN := flag.Int("top", 10, "top N relationships to display per sense")
	flag.Parse()

	if strings.TrimSpace(*in) == "" {
		fmt.Fprintln(os.Stderr, "-in is required")
		os.Exit(2)
	}

	ds, err := loadDataset(*in, *sheet)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading dataset:", err)
		os.Exit(1)
	}

	report(ds, *topN)
}

func report(ds *dataset, topN int) {
	ctx := context.Background()
	engine := senses.NewSenseEngine()

	// Temporal integrity (best-effort)
	dateIntegrity := "(no date column detected)"
	if ds.DateColumn != "" && len(ds.Dates) == ds.Rows {
		miss := 0
		badOrder := 0
		dupes := 0
		seen := make(map[string]struct{}, len(ds.Dates))
		for i := 0; i < len(ds.Dates); i++ {
			if ds.Dates[i].IsZero() {
				miss++
				continue
			}
			key := ds.Dates[i].Format("2006-01-02")
			if _, ok := seen[key]; ok {
				dupes++
			} else {
				seen[key] = struct{}{}
			}
			if i > 0 && !ds.Dates[i-1].IsZero() && ds.Dates[i].Before(ds.Dates[i-1]) {
				badOrder++
			}
		}
		dateIntegrity = fmt.Sprintf("aligned_rows=%d/%d | missing_dates=%d | duplicates=%d | out_of_order=%d", len(ds.Dates)-miss, ds.Rows, miss, dupes, badOrder)
	}

	n := len(ds.VarNames)
	totalPairs := (n * (n - 1)) / 2

	coverage := map[string]*senseAgg{}
	eliteBySense := map[string][]eliteHit{}

	for i := 0; i < n; i++ {
		xName := ds.VarNames[i]
		x := ds.Cols[xName]
		for j := i + 1; j < n; j++ {
			yName := ds.VarNames[j]
			y := ds.Cols[yName]

			results := engine.AnalyzeAll(ctx, x, y, sensesKey(xName), sensesKey(yName))
			for _, r := range results {
				agg := coverage[r.SenseName]
				if agg == nil {
					agg = &senseAgg{SenseName: r.SenseName}
					coverage[r.SenseName] = agg
				}
				switch r.Signal {
				case "very_strong":
					agg.VeryStrong++
				case "strong":
					agg.Strong++
				case "moderate":
					agg.Moderate++
				default:
					agg.Weak++
				}

				if r.Signal == "very_strong" || r.Signal == "strong" {
					eliteBySense[r.SenseName] = append(eliteBySense[r.SenseName], eliteHit{
						X:        xName,
						Y:        yName,
						Sense:    r.SenseName,
						Effect:   r.EffectSize,
						PValue:   r.PValue,
						Signal:   r.Signal,
						Metadata: r.Metadata,
					})
				}
			}
		}
	}

	fmt.Println("=== Composer Health Report ===")
	fmt.Printf("Dataset: rows=%d | numeric_vars=%d | pairs=%d\n", ds.Rows, n, totalPairs)
	fmt.Printf("Temporal integrity: %s\n", dateIntegrity)

	// Coverage summary
	keys := make([]string, 0, len(coverage))
	for k := range coverage {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Println("\n-- Sense coverage --")
	for _, k := range keys {
		a := coverage[k]
		fmt.Printf("%s: weak=%d moderate=%d strong=%d very_strong=%d\n", a.SenseName, a.Weak, a.Moderate, a.Strong, a.VeryStrong)
	}

	// Elite list (by sense)
	fmt.Println("\n-- Elite relationships (top by sense) --")
	priority := []string{"mutual_information", "cross_correlation", "spearman"}
	for _, senseName := range priority {
		list := eliteBySense[senseName]
		if len(list) == 0 {
			continue
		}
		sort.Slice(list, func(i, j int) bool {
			ai := math.Abs(list[i].Effect)
			aj := math.Abs(list[j].Effect)
			if ai == aj {
				return list[i].PValue < list[j].PValue
			}
			return ai > aj
		})

		fmt.Printf("%s:\n", senseName)
		limit := topN
		if limit > len(list) {
			limit = len(list)
		}
		for i := 0; i < limit; i++ {
			e := list[i]
			lag := ""
			if v, ok := e.Metadata["best_lag"].(int); ok && v != 0 {
				lag = fmt.Sprintf(" lag=%d", v)
			}
			fmt.Printf("  %d) %s ~ %s | effect=%.4f p=%.4g signal=%s%s\n", i+1, e.X, e.Y, e.Effect, e.PValue, e.Signal, lag)
		}
	}

	// Gold-standard sanity checks (best-effort)
	fmt.Println("\n-- Gold-standard checks --")
	checkGoldStandard(ds, engine)
}

func checkGoldStandard(ds *dataset, engine *senses.SenseEngine) {
	ctx := context.Background()

	// Echo: spend -> conversion_rate lag 3
	if has(ds, "top_funnel_spend_usd") && has(ds, "conversion_rate") {
		r, ok := engine.AnalyzeSingle(ctx, "cross_correlation", ds.Cols["top_funnel_spend_usd"], ds.Cols["conversion_rate"], sensesKey("top_funnel_spend_usd"), sensesKey("conversion_rate"))
		if ok {
			bestLag, _ := r.Metadata["best_lag"].(int)
			fmt.Printf("Echo (spend->conv): best_lag=%d signal=%s effect=%.4f\n", bestLag, r.Signal, r.EffectSize)
		} else {
			fmt.Println("Echo (spend->conv): cross_correlation sense unavailable")
		}
	} else {
		fmt.Println("Echo (spend->conv): missing required columns")
	}

	// Cliff: impressions_per_user -> click_through_rate should be non-linear
	if has(ds, "impressions_per_user") && has(ds, "click_through_rate") {
		r, ok := engine.AnalyzeSingle(ctx, "mutual_information", ds.Cols["impressions_per_user"], ds.Cols["click_through_rate"], sensesKey("impressions_per_user"), sensesKey("click_through_rate"))
		if ok {
			fmt.Printf("Cliff (freq->ctr): MI=%.4f p=%.4g signal=%s\n", r.EffectSize, r.PValue, r.Signal)
		} else {
			fmt.Println("Cliff (freq->ctr): mutual_information sense unavailable")
		}
	} else {
		fmt.Println("Cliff (freq->ctr): missing required columns")
	}

	// Hub: brand_search_volume should correlate with campaign_sub_metric_1 more than a noise metric
	if has(ds, "brand_search_volume") && has(ds, "campaign_sub_metric_1") {
		corr, _ := engine.AnalyzeSingle(ctx, "mutual_information", ds.Cols["brand_search_volume"], ds.Cols["campaign_sub_metric_1"], sensesKey("brand_search_volume"), sensesKey("campaign_sub_metric_1"))
		noise := senses.SenseResult{}
		if has(ds, "campaign_sub_metric_20") {
			noise, _ = engine.AnalyzeSingle(ctx, "mutual_information", ds.Cols["brand_search_volume"], ds.Cols["campaign_sub_metric_20"], sensesKey("brand_search_volume"), sensesKey("campaign_sub_metric_20"))
		}

		if noise.SenseName != "" {
			fmt.Printf("Hub: MI(brand,sub1)=%.4f vs MI(brand,sub20)=%.4f\n", corr.EffectSize, noise.EffectSize)
		} else {
			fmt.Printf("Hub: MI(brand,sub1)=%.4f (noise metric missing)\n", corr.EffectSize)
		}
	} else {
		fmt.Println("Hub: missing required columns")
	}
}

func loadDataset(path string, sheet string) (*dataset, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".xlsx":
		return loadXLSX(path, sheet)
	case ".csv":
		return loadCSV(path)
	default:
		return nil, fmt.Errorf("unsupported input extension: %s", ext)
	}
}

func loadXLSX(path string, sheet string) (*dataset, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	return loadFromRows(rows)
}

func loadCSV(path string) (*dataset, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	return loadFromRows(rows)
}

func loadFromRows(rows [][]string) (*dataset, error) {
	if len(rows) < 2 {
		return nil, fmt.Errorf("need header + at least one row")
	}

	headers := rows[0]
	dataRows := rows[1:]

	// Identify a date column (if present)
	dateCol := ""
	for _, h := range headers {
		if strings.EqualFold(strings.TrimSpace(h), "date") {
			dateCol = h
			break
		}
	}

	cols := make(map[string][]float64, len(headers))
	varNames := make([]string, 0, len(headers))
	dates := make([]time.Time, 0, len(dataRows))

	for ci, h := range headers {
		h = strings.TrimSpace(h)
		if h == "" {
			h = fmt.Sprintf("col_%d", ci)
		}
		headers[ci] = h

		if h == dateCol {
			// parse dates for integrity checks but do not include as numeric var
			for _, row := range dataRows {
				if ci >= len(row) {
					dates = append(dates, time.Time{})
					continue
				}
				ts, _ := time.Parse("2006-01-02", strings.TrimSpace(row[ci]))
				dates = append(dates, ts)
			}
			continue
		}

		vals := make([]float64, len(dataRows))
		allNaN := true
		for ri, row := range dataRows {
			if ci >= len(row) {
				vals[ri] = math.NaN()
				continue
			}
			s := strings.TrimSpace(row[ci])
			if s == "" {
				vals[ri] = math.NaN()
				continue
			}
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				vals[ri] = math.NaN()
				continue
			}
			vals[ri] = v
			allNaN = false
		}
		if allNaN {
			// Skip non-numeric columns
			continue
		}

		cols[h] = vals
		varNames = append(varNames, h)
	}

	sort.Strings(varNames)

	return &dataset{
		Headers:    headers,
		Rows:       len(dataRows),
		VarNames:   varNames,
		Cols:       cols,
		DateColumn: dateCol,
		Dates:      dates,
	}, nil
}

func has(ds *dataset, name string) bool {
	_, ok := ds.Cols[name]
	return ok
}

func sensesKey(name string) core.VariableKey {
	// Use the raw column name as the variable key.
	return core.VariableKey(name)
}
