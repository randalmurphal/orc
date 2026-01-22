package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

func setupCostTestServer(t *testing.T) (*Server, *db.GlobalDB) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	// Open GlobalDB using standard Open path (SQLite)
	tempGlobalDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := tempGlobalDB.Migrate("global"); err != nil {
		_ = tempGlobalDB.Close()
		t.Fatalf("failed to migrate global db: %v", err)
	}

	globalDB := &db.GlobalDB{DB: tempGlobalDB}

	// Create minimal server with just globalDB for cost endpoint testing
	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	s := New(cfg)
	s.globalDB = globalDB

	return s, globalDB
}

func seedCostData(t *testing.T, gdb *db.GlobalDB, projectID string) {
	t.Helper()

	now := time.Now().UTC()
	entries := []db.CostEntry{
		{
			ProjectID:           projectID,
			TaskID:              "TASK-001",
			Phase:               "implement",
			Model:               "sonnet",
			Iteration:           1,
			CostUSD:             0.15,
			InputTokens:         1000,
			OutputTokens:        500,
			CacheCreationTokens: 100,
			CacheReadTokens:     50,
			TotalTokens:         1650,
			InitiativeID:        "INIT-001",
			DurationMs:          5000,
			Timestamp:           now.Add(-2 * 24 * time.Hour), // 2 days ago
		},
		{
			ProjectID:           projectID,
			TaskID:              "TASK-001",
			Phase:               "test",
			Model:               "opus",
			Iteration:           1,
			CostUSD:             0.25,
			InputTokens:         800,
			OutputTokens:        400,
			CacheCreationTokens: 80,
			CacheReadTokens:     40,
			TotalTokens:         1320,
			InitiativeID:        "INIT-001",
			DurationMs:          3000,
			Timestamp:           now.Add(-1 * 24 * time.Hour), // 1 day ago
		},
		{
			ProjectID:           projectID,
			TaskID:              "TASK-002",
			Phase:               "implement",
			Model:               "sonnet",
			Iteration:           1,
			CostUSD:             0.10,
			InputTokens:         600,
			OutputTokens:        300,
			CacheCreationTokens: 60,
			CacheReadTokens:     30,
			TotalTokens:         990,
			InitiativeID:        "INIT-002",
			DurationMs:          4000,
			Timestamp:           now.Add(-10 * 24 * time.Hour), // 10 days ago
		},
		{
			ProjectID:           projectID,
			TaskID:              "TASK-003",
			Phase:               "review",
			Model:               "haiku",
			Iteration:           1,
			CostUSD:             0.05,
			InputTokens:         400,
			OutputTokens:        200,
			CacheCreationTokens: 40,
			CacheReadTokens:     20,
			TotalTokens:         660,
			InitiativeID:        "",
			DurationMs:          2000,
			Timestamp:           now.Add(-5 * time.Hour), // 5 hours ago
		},
	}

	for _, entry := range entries {
		if err := gdb.RecordCostExtended(entry); err != nil {
			t.Fatalf("failed to seed cost data: %v", err)
		}
	}
}

func TestHandleCostBreakdown(t *testing.T) {
	t.Parallel()

	s, gdb := setupCostTestServer(t)
	defer func() { _ = gdb.Close() }()

	projectID := s.workDir
	seedCostData(t, gdb, projectID)

	tests := []struct {
		name       string
		by         string
		period     string
		wantStatus int
		wantKeys   []string // expected breakdown keys
	}{
		{
			name:       "breakdown by model all time",
			by:         "model",
			period:     "all",
			wantStatus: http.StatusOK,
			wantKeys:   []string{"sonnet", "opus", "haiku"},
		},
		{
			name:       "breakdown by model 7 days",
			by:         "model",
			period:     "7d",
			wantStatus: http.StatusOK,
			wantKeys:   []string{"sonnet", "opus", "haiku"}, // all within 7 days
		},
		{
			name:       "breakdown by phase",
			by:         "phase",
			period:     "all",
			wantStatus: http.StatusOK,
			wantKeys:   []string{"implement", "test", "review"},
		},
		{
			name:       "breakdown by task",
			by:         "task",
			period:     "all",
			wantStatus: http.StatusOK,
			wantKeys:   []string{"TASK-001", "TASK-002", "TASK-003"},
		},
		{
			name:       "breakdown by initiative",
			by:         "initiative",
			period:     "all",
			wantStatus: http.StatusOK,
			wantKeys:   []string{"INIT-001", "INIT-002"},
		},
		{
			name:       "invalid by parameter",
			by:         "invalid",
			period:     "all",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/cost/breakdown?by="+tt.by+"&period="+tt.period, nil)
			rec := httptest.NewRecorder()

			s.handleCostBreakdown(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var resp CostBreakdownResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				// Verify expected keys present
				for _, key := range tt.wantKeys {
					if _, ok := resp.Breakdown[key]; !ok {
						t.Errorf("missing expected breakdown key: %s", key)
					}
				}

				// Verify percentages sum to ~100%
				var totalPercent float64
				for _, bd := range resp.Breakdown {
					totalPercent += bd.Percent
				}
				if totalPercent > 0 && (totalPercent < 99.9 || totalPercent > 100.1) {
					t.Errorf("percentages sum to %.2f, want ~100", totalPercent)
				}

				// Verify total cost is positive
				if resp.TotalCostUSD <= 0 {
					t.Errorf("total cost = %.2f, want > 0", resp.TotalCostUSD)
				}
			}
		})
	}
}

func TestHandleCostTimeseries(t *testing.T) {
	t.Parallel()

	s, gdb := setupCostTestServer(t)
	defer func() { _ = gdb.Close() }()

	projectID := s.workDir
	seedCostData(t, gdb, projectID)

	tests := []struct {
		name        string
		granularity string
		wantStatus  int
		wantMinLen  int // minimum series length
	}{
		{
			name:        "daily granularity",
			granularity: "day",
			wantStatus:  http.StatusOK,
			wantMinLen:  30, // default 30 days
		},
		{
			name:        "weekly granularity",
			granularity: "week",
			wantStatus:  http.StatusOK,
			wantMinLen:  4, // ~4 weeks in 30 days
		},
		{
			name:        "monthly granularity",
			granularity: "month",
			wantStatus:  http.StatusOK,
			wantMinLen:  1,
		},
		{
			name:        "invalid granularity",
			granularity: "invalid",
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/cost/timeseries?granularity="+tt.granularity, nil)
			rec := httptest.NewRecorder()

			s.handleCostTimeseries(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var resp CostTimeseriesResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if len(resp.Series) < tt.wantMinLen {
					t.Errorf("series length = %d, want >= %d", len(resp.Series), tt.wantMinLen)
				}

				// Verify granularity matches
				if resp.Granularity != tt.granularity {
					t.Errorf("granularity = %s, want %s", resp.Granularity, tt.granularity)
				}
			}
		})
	}
}

func TestHandleCostBudget(t *testing.T) {
	t.Parallel()

	s, gdb := setupCostTestServer(t)
	defer func() { _ = gdb.Close() }()

	projectID := s.workDir

	t.Run("no budget configured", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/cost/budget", nil)
		rec := httptest.NewRecorder()

		s.handleCostBudget(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp BudgetStatusResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.MonthlyLimitUSD != 0 {
			t.Errorf("monthly limit = %.2f, want 0 (no budget set)", resp.MonthlyLimitUSD)
		}
	})

	t.Run("with budget configured", func(t *testing.T) {
		// Set a budget
		budget := db.CostBudget{
			ProjectID:             projectID,
			MonthlyLimitUSD:       100.0,
			AlertThresholdPercent: 80,
			CurrentMonth:          time.Now().UTC().Format("2006-01"),
			CurrentMonthSpent:     25.0,
		}
		if err := gdb.SetBudget(budget); err != nil {
			t.Fatalf("failed to set budget: %v", err)
		}

		req := httptest.NewRequest("GET", "/api/cost/budget", nil)
		rec := httptest.NewRecorder()

		s.handleCostBudget(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp BudgetStatusResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.MonthlyLimitUSD != 100.0 {
			t.Errorf("monthly limit = %.2f, want 100.0", resp.MonthlyLimitUSD)
		}

		if resp.CurrentSpentUSD != 25.0 {
			t.Errorf("current spent = %.2f, want 25.0", resp.CurrentSpentUSD)
		}

		if resp.PercentUsed != 25.0 {
			t.Errorf("percent used = %.2f, want 25.0", resp.PercentUsed)
		}

		if resp.RemainingUSD != 75.0 {
			t.Errorf("remaining = %.2f, want 75.0", resp.RemainingUSD)
		}
	})
}

func TestHandleUpdateCostBudget(t *testing.T) {
	t.Parallel()

	s, gdb := setupCostTestServer(t)
	defer func() { _ = gdb.Close() }()

	tests := []struct {
		name       string
		request    BudgetUpdateRequest
		wantStatus int
	}{
		{
			name: "valid budget update",
			request: BudgetUpdateRequest{
				MonthlyLimitUSD:       150.0,
				AlertThresholdPercent: 75,
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid negative limit",
			request: BudgetUpdateRequest{
				MonthlyLimitUSD:       -10.0,
				AlertThresholdPercent: 80,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid threshold percentage",
			request: BudgetUpdateRequest{
				MonthlyLimitUSD:       100.0,
				AlertThresholdPercent: 150,
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("PUT", "/api/cost/budget", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			s.handleUpdateCostBudget(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				// Verify budget was actually saved
				budget, err := gdb.GetBudget(s.workDir)
				if err != nil {
					t.Fatalf("failed to get budget: %v", err)
				}
				if budget == nil {
					t.Fatal("budget not saved")
				}
				if budget.MonthlyLimitUSD != tt.request.MonthlyLimitUSD {
					t.Errorf("saved limit = %.2f, want %.2f", budget.MonthlyLimitUSD, tt.request.MonthlyLimitUSD)
				}
			}
		})
	}
}

func TestHandleInitiativeCost(t *testing.T) {
	t.Parallel()

	s, gdb := setupCostTestServer(t)
	defer func() { _ = gdb.Close() }()

	projectID := s.workDir
	seedCostData(t, gdb, projectID)

	tests := []struct {
		name         string
		initiativeID string
		wantStatus   int
		wantTasks    int
	}{
		{
			name:         "valid initiative with costs",
			initiativeID: "INIT-001",
			wantStatus:   http.StatusOK,
			wantTasks:    1, // TASK-001 (aggregated across 2 phases)
		},
		{
			name:         "initiative with single task",
			initiativeID: "INIT-002",
			wantStatus:   http.StatusOK,
			wantTasks:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/initiatives/"+tt.initiativeID+"/cost", nil)
			rec := httptest.NewRecorder()

			s.handleInitiativeCost(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var resp InitiativeCostResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if resp.InitiativeID != tt.initiativeID {
					t.Errorf("initiative ID = %s, want %s", resp.InitiativeID, tt.initiativeID)
				}

				if resp.TotalCostUSD <= 0 {
					t.Errorf("total cost = %.2f, want > 0", resp.TotalCostUSD)
				}

				if len(resp.ByTask) != tt.wantTasks {
					t.Errorf("task count = %d, want %d", len(resp.ByTask), tt.wantTasks)
				}

				if len(resp.ByModel) == 0 {
					t.Error("by_model breakdown is empty")
				}

				if len(resp.ByPhase) == 0 {
					t.Error("by_phase breakdown is empty")
				}
			}
		})
	}
}

func TestCostAnalyticsUnavailable(t *testing.T) {
	t.Parallel()

	// Create server without globalDB
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	s := New(cfg)
	s.globalDB = nil // explicitly nil

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/cost/breakdown"},
		{"GET", "/api/cost/timeseries"},
		{"GET", "/api/cost/budget"},
		{"PUT", "/api/cost/budget"},
		{"GET", "/api/initiatives/INIT-001/cost"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			var req *http.Request
			if ep.method == "PUT" {
				body := bytes.NewReader([]byte(`{"monthly_limit_usd": 100}`))
				req = httptest.NewRequest(ep.method, ep.path, body)
			} else {
				req = httptest.NewRequest(ep.method, ep.path, nil)
			}
			rec := httptest.NewRecorder()

			switch ep.path {
			case "/api/cost/breakdown":
				s.handleCostBreakdown(rec, req)
			case "/api/cost/timeseries":
				s.handleCostTimeseries(rec, req)
			case "/api/cost/budget":
				if ep.method == "GET" {
					s.handleCostBudget(rec, req)
				} else {
					s.handleUpdateCostBudget(rec, req)
				}
			default:
				s.handleInitiativeCost(rec, req)
			}

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
			}
		})
	}
}
