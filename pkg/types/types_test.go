package types

import (
	"encoding/json"
	"testing"
)

func TestEnrichmentProviderPrecedenceEnabledDefaults(t *testing.T) {
	// Test 1: absent fields in JSON
	jsonInput := `{"id":"test","name":"Test","kinds":["titles"]}`
	var ep EnrichmentProvider
	if err := json.Unmarshal([]byte(jsonInput), &ep); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if ep.Precedence != 0 {
		t.Errorf("expected precedence 0 when omitted, got %d", ep.Precedence)
	}
	if ep.Enabled != nil {
		t.Errorf("expected enabled nil when omitted, got %v", *ep.Enabled)
	}

	// Test 2: fields present in JSON
	jsonInput2 := `{"id":"test2","name":"Test2","kinds":["summaries"],"precedence":5,"enabled":true}`
	var ep2 EnrichmentProvider
	if err := json.Unmarshal([]byte(jsonInput2), &ep2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if ep2.Precedence != 5 {
		t.Errorf("expected precedence 5, got %d", ep2.Precedence)
	}
	if ep2.Enabled == nil || !*ep2.Enabled {
		t.Errorf("expected enabled true, got %v", ep2.Enabled)
	}

	// Test 3: round-trip preserves declared values
	jsonInput3 := `{"id":"test3","name":"Test3","kinds":["categories"],"precedence":100,"enabled":false}`
	var ep3 EnrichmentProvider
	if err := json.Unmarshal([]byte(jsonInput3), &ep3); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	data, err := json.Marshal(&ep3)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var ep3RoundTrip EnrichmentProvider
	if err := json.Unmarshal(data, &ep3RoundTrip); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}
	if ep3RoundTrip.Precedence != 100 {
		t.Errorf("round-trip: expected precedence 100, got %d", ep3RoundTrip.Precedence)
	}
	if ep3RoundTrip.Enabled == nil {
		t.Errorf("round-trip: enabled should not be nil, was nil after round-trip")
	} else if *ep3RoundTrip.Enabled {
		t.Errorf("round-trip: expected enabled false, got true")
	}
}

func TestPluginMetaEnrichmentProvidersRoundTrip(t *testing.T) {
	jsonInput := `{
		"verify_url": "https://example.com/verify",
		"needs_human_verify": true,
		"thumb_ratio": 1.7,
		"enrichment_providers": [
			{"id":"source1","name":"Source 1","kinds":["titles"],"precedence":1,"enabled":true},
			{"id":"source2","name":"Source 2","kinds":["categories"],"precedence":10}
		]
	}`

	var meta PluginMeta
	if err := json.Unmarshal([]byte(jsonInput), &meta); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(meta.EnrichmentProviders) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(meta.EnrichmentProviders))
	}

	// Verify first provider
	if meta.EnrichmentProviders[0].Precedence != 1 {
		t.Errorf("expected precedence 1, got %d", meta.EnrichmentProviders[0].Precedence)
	}
	if meta.EnrichmentProviders[0].Enabled == nil || !*meta.EnrichmentProviders[0].Enabled {
		t.Errorf("expected enabled true, got %v", meta.EnrichmentProviders[0].Enabled)
	}

	// Round-trip
	data, err := json.Marshal(&meta)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var metaRoundTrip PluginMeta
	if err := json.Unmarshal(data, &metaRoundTrip); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}

	if len(metaRoundTrip.EnrichmentProviders) != 2 {
		t.Fatalf("round-trip: expected 2 providers, got %d", len(metaRoundTrip.EnrichmentProviders))
	}
	if metaRoundTrip.EnrichmentProviders[0].Precedence != 1 {
		t.Errorf("round-trip: expected precedence 1, got %d", metaRoundTrip.EnrichmentProviders[0].Precedence)
	}
	if metaRoundTrip.EnrichmentProviders[1].Precedence != 10 {
		t.Errorf("round-trip: expected precedence 10, got %d", metaRoundTrip.EnrichmentProviders[1].Precedence)
	}
}
