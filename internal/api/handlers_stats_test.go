package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

func TestHandleGetActivityStats_DefaultWeeks(t *testing.T) {
	t.Parallel()

	// Create temp directory for test
	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create server
	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	// Create request without weeks param
	req := httptest.NewRequest(http.MethodGet, "/api/stats/activity", nil)
	rr := httptest.NewRecorder()

	server.handleGetActivityStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response ActivityResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have exactly 16*7 = 112 days
	if len(response.Data) != 112 {
		t.Errorf("expected 112 days, got %d", len(response.Data))
	}

	// All should have level 0 and count 0 (empty database)
	for i, day := range response.Data {
		if day.Count != 0 {
			t.Errorf("day %d: expected count 0, got %d", i, day.Count)
		}
		if day.Level != 0 {
			t.Errorf("day %d: expected level 0, got %d", i, day.Level)
		}
	}

	// Stats should be zero
	if response.Stats.TotalTasks != 0 {
		t.Errorf("expected TotalTasks=0, got %d", response.Stats.TotalTasks)
	}
	if response.Stats.CurrentStreak != 0 {
		t.Errorf("expected CurrentStreak=0, got %d", response.Stats.CurrentStreak)
	}
	if response.Stats.LongestStreak != 0 {
		t.Errorf("expected LongestStreak=0, got %d", response.Stats.LongestStreak)
	}
	if response.Stats.BusiestDay != nil {
		t.Errorf("expected BusiestDay=nil, got %+v", response.Stats.BusiestDay)
	}
}

func TestHandleGetActivityStats_CustomWeeks(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	// Request 8 weeks
	req := httptest.NewRequest(http.MethodGet, "/api/stats/activity?weeks=8", nil)
	rr := httptest.NewRecorder()

	server.handleGetActivityStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response ActivityResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have exactly 8*7 = 56 days
	if len(response.Data) != 56 {
		t.Errorf("expected 56 days, got %d", len(response.Data))
	}
}

func TestHandleGetActivityStats_InvalidWeeks(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	tests := []struct {
		name  string
		weeks string
	}{
		{"zero", "0"},
		{"negative", "-1"},
		{"too large", "100"},
		{"non-numeric", "abc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/stats/activity?weeks="+tc.weeks, nil)
			rr := httptest.NewRecorder()

			server.handleGetActivityStats(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", rr.Code)
			}
		})
	}
}

func TestHandleGetActivityStats_LevelCalculation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		count    int
		expected int
	}{
		{0, 0},
		{1, 1},
		{2, 1},
		{3, 2},
		{5, 2},
		{6, 3},
		{10, 3},
		{11, 4},
		{100, 4},
	}

	for _, tc := range tests {
		result := calculateActivityLevel(tc.count)
		if result != tc.expected {
			t.Errorf("calculateActivityLevel(%d) = %d, expected %d", tc.count, result, tc.expected)
		}
	}
}

func TestHandleGetActivityStats_StreakCalculation(t *testing.T) {
	t.Parallel()

	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	twoDaysAgo := now.AddDate(0, 0, -2).Format("2006-01-02")
	threeDaysAgo := now.AddDate(0, 0, -3).Format("2006-01-02")
	fourDaysAgo := now.AddDate(0, 0, -4).Format("2006-01-02")

	tests := []struct {
		name            string
		data            []ActivityDay
		expectedCurrent int
		expectedLongest int
	}{
		{
			name:            "empty",
			data:            []ActivityDay{},
			expectedCurrent: 0,
			expectedLongest: 0,
		},
		{
			name: "no activity today",
			data: []ActivityDay{
				{Date: yesterday, Count: 0, Level: 0},
				{Date: today, Count: 0, Level: 0},
			},
			expectedCurrent: 0,
			expectedLongest: 0,
		},
		{
			name: "activity only today",
			data: []ActivityDay{
				{Date: yesterday, Count: 0, Level: 0},
				{Date: today, Count: 5, Level: 2},
			},
			expectedCurrent: 1,
			expectedLongest: 1,
		},
		{
			name: "consecutive days including today",
			data: []ActivityDay{
				{Date: threeDaysAgo, Count: 0, Level: 0},
				{Date: twoDaysAgo, Count: 3, Level: 2},
				{Date: yesterday, Count: 5, Level: 2},
				{Date: today, Count: 1, Level: 1},
			},
			expectedCurrent: 3,
			expectedLongest: 3,
		},
		{
			name: "gap in activity",
			data: []ActivityDay{
				{Date: fourDaysAgo, Count: 2, Level: 1},
				{Date: threeDaysAgo, Count: 3, Level: 2},
				{Date: twoDaysAgo, Count: 0, Level: 0}, // gap
				{Date: yesterday, Count: 1, Level: 1},
				{Date: today, Count: 2, Level: 1},
			},
			expectedCurrent: 2,
			expectedLongest: 2,
		},
		{
			name: "no today but yesterday active",
			data: []ActivityDay{
				{Date: twoDaysAgo, Count: 3, Level: 2},
				{Date: yesterday, Count: 5, Level: 2},
				{Date: today, Count: 0, Level: 0},
			},
			expectedCurrent: 2,
			expectedLongest: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			current, longest := calculateStreaks(tc.data, now)
			if current != tc.expectedCurrent {
				t.Errorf("current streak: got %d, expected %d", current, tc.expectedCurrent)
			}
			if longest != tc.expectedLongest {
				t.Errorf("longest streak: got %d, expected %d", longest, tc.expectedLongest)
			}
		})
	}
}

func TestHandleGetActivityStats_WithTasks(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create backend and save test data
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)

	// Create 3 tasks completed today
	for i := 1; i <= 3; i++ {
		taskID := "TASK-" + fmt.Sprintf("%03d", i)
		tsk := task.New(taskID, "Task today "+fmt.Sprintf("%d", i))
		tsk.Status = task.StatusCompleted
		completedAt := today.Add(time.Duration(i) * time.Hour)
		tsk.CompletedAt = &completedAt
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("failed to save task: %v", err)
		}
	}

	// Create 5 tasks completed yesterday
	for i := 1; i <= 5; i++ {
		taskID := "TASK-" + fmt.Sprintf("%03d", 100+i)
		tsk := task.New(taskID, "Task yesterday "+fmt.Sprintf("%d", i))
		tsk.Status = task.StatusCompleted
		completedAt := yesterday.Add(time.Duration(i) * time.Hour)
		tsk.CompletedAt = &completedAt
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("failed to save task: %v", err)
		}
	}

	// Close backend before creating server
	_ = backend.Close()

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/stats/activity?weeks=1", nil)
	rr := httptest.NewRecorder()

	server.handleGetActivityStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response ActivityResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have 7 days
	if len(response.Data) != 7 {
		t.Errorf("expected 7 days, got %d", len(response.Data))
	}

	// Check total tasks
	if response.Stats.TotalTasks != 8 {
		t.Errorf("expected TotalTasks=8, got %d", response.Stats.TotalTasks)
	}

	// Check streak (2 consecutive days)
	if response.Stats.CurrentStreak != 2 {
		t.Errorf("expected CurrentStreak=2, got %d", response.Stats.CurrentStreak)
	}

	// Check busiest day (yesterday with 5 tasks)
	if response.Stats.BusiestDay == nil {
		t.Fatal("expected BusiestDay to be set")
	}
	if response.Stats.BusiestDay.Count != 5 {
		t.Errorf("expected BusiestDay.Count=5, got %d", response.Stats.BusiestDay.Count)
	}

	// Find today and yesterday in data
	todayStr := today.Format("2006-01-02")
	yesterdayStr := yesterday.Format("2006-01-02")

	var todayData, yesterdayData *ActivityDay
	for i := range response.Data {
		if response.Data[i].Date == todayStr {
			todayData = &response.Data[i]
		}
		if response.Data[i].Date == yesterdayStr {
			yesterdayData = &response.Data[i]
		}
	}

	if todayData == nil {
		t.Fatal("today's data not found")
	}
	if todayData.Count != 3 {
		t.Errorf("expected today count=3, got %d", todayData.Count)
	}
	if todayData.Level != 2 { // 3 tasks = level 2
		t.Errorf("expected today level=2, got %d", todayData.Level)
	}

	if yesterdayData == nil {
		t.Fatal("yesterday's data not found")
	}
	if yesterdayData.Count != 5 {
		t.Errorf("expected yesterday count=5, got %d", yesterdayData.Count)
	}
	if yesterdayData.Level != 2 { // 5 tasks = level 2
		t.Errorf("expected yesterday level=2, got %d", yesterdayData.Level)
	}
}

func TestHandleGetActivityStats_EmptyDatabase(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/stats/activity", nil)
	rr := httptest.NewRecorder()

	server.handleGetActivityStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response ActivityResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should return valid response with zeros
	if response.Stats.TotalTasks != 0 {
		t.Errorf("expected TotalTasks=0, got %d", response.Stats.TotalTasks)
	}
	if response.Stats.CurrentStreak != 0 {
		t.Errorf("expected CurrentStreak=0, got %d", response.Stats.CurrentStreak)
	}
	if response.Stats.LongestStreak != 0 {
		t.Errorf("expected LongestStreak=0, got %d", response.Stats.LongestStreak)
	}
	if response.Stats.BusiestDay != nil {
		t.Errorf("expected BusiestDay=nil, got %+v", response.Stats.BusiestDay)
	}

	// All days should have zero counts
	for _, day := range response.Data {
		if day.Count != 0 || day.Level != 0 {
			t.Errorf("expected empty day, got count=%d, level=%d", day.Count, day.Level)
		}
	}
}
