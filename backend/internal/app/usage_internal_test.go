package app

import (
	"context"
	"database/sql"
	"testing"
)

func TestSaveUsageMessageStoresReasoningEffortAndTTFT(t *testing.T) {
	t.Setenv("CPA_HELPER_DATA_DIR", t.TempDir())
	app, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Close()

	raw := `{"api_key":"sk-usage-ttft","provider":"openai","model":"gpt-5.5","request_id":"usage-ttft","reasoning_effort":"xhigh","ttft_ms":710,"input_tokens":10,"output_tokens":2}`
	record, created, err := app.saveUsageMessage(context.Background(), []byte(raw))
	if err != nil || !created {
		t.Fatalf("saveUsageMessage created=%v err=%v", created, err)
	}
	if record.ReasoningEffort == nil || *record.ReasoningEffort != "xhigh" {
		t.Fatalf("record reasoning_effort = %#v, want xhigh", record.ReasoningEffort)
	}
	if record.TTFTMS == nil || *record.TTFTMS != 710 {
		t.Fatalf("record ttft_ms = %#v, want 710", record.TTFTMS)
	}

	var reasoningEffort sql.NullString
	var ttftMS sql.NullFloat64
	if err := app.db.QueryRow(`SELECT reasoning_effort, ttft_ms FROM usage_records WHERE id = ?`, record.ID).Scan(&reasoningEffort, &ttftMS); err != nil {
		t.Fatal(err)
	}
	if !reasoningEffort.Valid || reasoningEffort.String != "xhigh" || !ttftMS.Valid || ttftMS.Float64 != 710 {
		t.Fatalf("stored reasoning/ttft = %#v/%#v, want xhigh/710", reasoningEffort, ttftMS)
	}
}

func TestSaveUsageMessageIgnoresZeroTTFT(t *testing.T) {
	t.Setenv("CPA_HELPER_DATA_DIR", t.TempDir())
	app, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Close()

	raw := `{"api_key":"sk-usage-ttft-zero","provider":"openai","model":"gpt-5.5","request_id":"usage-ttft-zero","ttft_ms":0,"input_tokens":10,"output_tokens":2}`
	record, created, err := app.saveUsageMessage(context.Background(), []byte(raw))
	if err != nil || !created {
		t.Fatalf("saveUsageMessage created=%v err=%v", created, err)
	}
	if record.TTFTMS != nil {
		t.Fatalf("record ttft_ms = %#v, want nil", record.TTFTMS)
	}

	var ttftMS sql.NullFloat64
	if err := app.db.QueryRow(`SELECT ttft_ms FROM usage_records WHERE id = ?`, record.ID).Scan(&ttftMS); err != nil {
		t.Fatal(err)
	}
	if ttftMS.Valid {
		t.Fatalf("stored ttft_ms = %v, want NULL", ttftMS.Float64)
	}
}

func TestFilteredUsageRecordsForStatsSkipsRawJSON(t *testing.T) {
	t.Setenv("CPA_HELPER_DATA_DIR", t.TempDir())
	app, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer app.Close()

	raw := `{"api_key":"sk-usage-stats-raw","timestamp":"2026-05-31T10:00:00+08:00","provider":"openai","model":"gpt-5.5","request_id":"usage-stats-raw","input_tokens":10,"output_tokens":2}`
	record, created, err := app.saveUsageMessage(context.Background(), []byte(raw))
	if err != nil || !created {
		t.Fatalf("saveUsageMessage created=%v err=%v", created, err)
	}
	if record.RawJSON == "" {
		t.Fatal("saved record raw_json is empty")
	}

	start, err := parseQueryTime("2026-05-31T00:00:00+08:00")
	if err != nil {
		t.Fatal(err)
	}
	end, err := parseQueryTime("2026-06-01T00:00:00+08:00")
	if err != nil {
		t.Fatal(err)
	}
	filters := UsageFilters{Start: &start, End: &end}

	statsRecords, err := app.filteredUsageRecordsForStats(context.Background(), filters, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(statsRecords) != 1 {
		t.Fatalf("stats record count = %d, want 1", len(statsRecords))
	}
	if statsRecords[0].RawJSON != "" {
		t.Fatalf("stats raw_json = %q, want empty lightweight projection", statsRecords[0].RawJSON)
	}

	fullRecords, err := app.filteredUsageRecords(context.Background(), filters, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(fullRecords) != 1 {
		t.Fatalf("full record count = %d, want 1", len(fullRecords))
	}
	if fullRecords[0].RawJSON == "" {
		t.Fatal("full raw_json is empty, want detail projection to keep raw_json")
	}
}
