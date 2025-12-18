package senses

import (
	"context"
	"testing"

	"gohypo/domain/core"
	"gohypo/internal/adforensics"
)

func TestGoldStandard_EchoDetects3DayLag(t *testing.T) {
	cfg := adforensics.DefaultConfig()
	cfg.Rows = 800
	cfg.Seed = 42
	cfg.EchoLagDays = 3

	ds, err := adforensics.Generate(cfg)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	sense := NewCrossCorrelationSense()
	res := sense.Analyze(context.Background(), ds.Spend, ds.ConvRate, core.VariableKey("top_funnel_spend_usd"), core.VariableKey("conversion_rate"))

	bestLag, ok := res.Metadata["best_lag"].(int)
	if !ok {
		t.Fatalf("expected best_lag metadata int, got %T (%v)", res.Metadata["best_lag"], res.Metadata["best_lag"])
	}
	if bestLag != 3 {
		t.Fatalf("expected best_lag=3, got %d (effect=%.4f, p=%.4g, signal=%s)", bestLag, res.EffectSize, res.PValue, res.Signal)
	}
}

func TestGoldStandard_CliffProducesStrongMutualInformation(t *testing.T) {
	cfg := adforensics.DefaultConfig()
	cfg.Rows = 1000
	cfg.Seed = 42

	ds, err := adforensics.Generate(cfg)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	sense := NewMutualInformationSense()
	res := sense.Analyze(context.Background(), ds.Frequency, ds.CTR, core.VariableKey("impressions_per_user"), core.VariableKey("click_through_rate"))

	// MI is measured in bits with 10-quantile binning. With the baked-in phase shift,
	// this should clear a modest floor reliably.
	if res.EffectSize < 0.10 {
		t.Fatalf("expected MI >= 0.10 for cliff relationship, got %.4f (p=%.4g, signal=%s)", res.EffectSize, res.PValue, res.Signal)
	}
}

func TestGoldStandard_HubCorrelationBeatsNoise(t *testing.T) {
	cfg := adforensics.DefaultConfig()
	cfg.Rows = 1000
	cfg.Seed = 42

	ds, err := adforensics.Generate(cfg)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	sense := NewMutualInformationSense()

	corr := sense.Analyze(context.Background(), ds.BrandVolume, ds.SubMetrics[0], core.VariableKey("brand_search_volume"), core.VariableKey("campaign_sub_metric_1"))
	noise := sense.Analyze(context.Background(), ds.BrandVolume, ds.SubMetrics[19], core.VariableKey("brand_search_volume"), core.VariableKey("campaign_sub_metric_20"))

	if corr.EffectSize <= noise.EffectSize+0.05 {
		t.Fatalf("expected hub-correlated MI to exceed noise by margin; corr=%.4f noise=%.4f", corr.EffectSize, noise.EffectSize)
	}
}
