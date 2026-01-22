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

// ============================================================================
// Per-Day Stats Tests
// ============================================================================

func TestHandleGetPerDayStats_DefaultDays(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/stats/per-day", nil)
	rr := httptest.NewRecorder()

	server.handleGetPerDayStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response PerDayResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have exactly 7 days
	if len(response.Data) != 7 {
		t.Errorf("expected 7 days, got %d", len(response.Data))
	}

	// Period should be "7d"
	if response.Period != "7d" {
		t.Errorf("expected period=7d, got %s", response.Period)
	}

	// All should have count 0 (empty database)
	for i, day := range response.Data {
		if day.Count != 0 {
			t.Errorf("day %d: expected count 0, got %d", i, day.Count)
		}
		if day.Date == "" {
			t.Errorf("day %d: date is empty", i)
		}
		if day.Day == "" {
			t.Errorf("day %d: day name is empty", i)
		}
	}

	// Max and average should be zero
	if response.Max != 0 {
		t.Errorf("expected max=0, got %d", response.Max)
	}
	if response.Average != 0.0 {
		t.Errorf("expected average=0.0, got %f", response.Average)
	}
}

func TestHandleGetPerDayStats_CustomDays(t *testing.T) {
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

	// Request 14 days
	req := httptest.NewRequest(http.MethodGet, "/api/stats/per-day?days=14", nil)
	rr := httptest.NewRecorder()

	server.handleGetPerDayStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response PerDayResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have exactly 14 days
	if len(response.Data) != 14 {
		t.Errorf("expected 14 days, got %d", len(response.Data))
	}

	// Period should be "14d"
	if response.Period != "14d" {
		t.Errorf("expected period=14d, got %s", response.Period)
	}
}

func TestHandleGetPerDayStats_InvalidDays(t *testing.T) {
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
		name string
		days string
	}{
		{"zero", "0"},
		{"negative", "-1"},
		{"too large", "100"},
		{"non-numeric", "abc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/stats/per-day?days="+tc.days, nil)
			rr := httptest.NewRecorder()

			server.handleGetPerDayStats(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", rr.Code)
			}
		})
	}
}

func TestHandleGetPerDayStats_WithTasks(t *testing.T) {
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
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Create tasks with different completion dates
	testData := []struct {
		daysAgo int
		count   int
	}{
		{0, 5},  // today: 5 tasks
		{1, 3},  // yesterday: 3 tasks
		{2, 0},  // 2 days ago: 0 tasks
		{3, 8},  // 3 days ago: 8 tasks
		{4, 2},  // 4 days ago: 2 tasks
		{5, 12}, // 5 days ago: 12 tasks
		{6, 7},  // 6 days ago: 7 tasks
	}

	taskNum := 1
	for _, td := range testData {
		for i := 0; i < td.count; i++ {
			taskID := "TASK-" + fmt.Sprintf("%03d", taskNum)
			tsk := task.New(taskID, "Task "+fmt.Sprintf("%d", taskNum))
			tsk.Status = task.StatusCompleted
			// Set time to avoid any day boundary issues - use minutes instead of hours
			completedAt := today.AddDate(0, 0, -td.daysAgo).Add(time.Duration(i) * time.Minute)
			tsk.CompletedAt = &completedAt
			if err := backend.SaveTask(tsk); err != nil {
				t.Fatalf("failed to save task: %v", err)
			}
			taskNum++
		}
	}

	// Close backend before creating server
	_ = backend.Close()

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/stats/per-day?days=7", nil)
	rr := httptest.NewRecorder()

	server.handleGetPerDayStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response PerDayResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have 7 days
	if len(response.Data) != 7 {
		t.Errorf("expected 7 days, got %d", len(response.Data))
	}

	// Check max count (should be 12 from 5 days ago)
	if response.Max != 12 {
		t.Errorf("expected max=12, got %d", response.Max)
	}

	// Check average (5+3+0+8+2+12+7 = 37 / 7 = 5.285...)
	expectedAverage := 37.0 / 7.0
	if response.Average < expectedAverage-0.01 || response.Average > expectedAverage+0.01 {
		t.Errorf("expected average≈%.2f, got %.2f", expectedAverage, response.Average)
	}

	// Verify the most recent day is today
	lastDay := response.Data[len(response.Data)-1]
	todayStr := today.Format("2006-01-02")
	if lastDay.Date != todayStr {
		t.Errorf("expected last day to be today (%s), got %s", todayStr, lastDay.Date)
	}
	if lastDay.Count != 5 {
		t.Errorf("expected today count=5, got %d", lastDay.Count)
	}

	// Verify day names are correct
	for i, day := range response.Data {
		date := today.AddDate(0, 0, -6+i)
		expectedDayName := date.Format("Mon")
		if day.Day != expectedDayName {
			t.Errorf("day %d: expected day name=%s, got %s", i, expectedDayName, day.Day)
		}
	}
}

func TestHandleGetPerDayStats_EmptyDatabase(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/stats/per-day", nil)
	rr := httptest.NewRecorder()

	server.handleGetPerDayStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response PerDayResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should return valid response with zeros
	if response.Max != 0 {
		t.Errorf("expected max=0, got %d", response.Max)
	}
	if response.Average != 0.0 {
		t.Errorf("expected average=0.0, got %f", response.Average)
	}

	// All days should have zero counts
	for _, day := range response.Data {
		if day.Count != 0 {
			t.Errorf("expected count=0, got %d", day.Count)
		}
	}
}

// ============================================================================
// Outcomes Endpoint Tests
// ============================================================================

func TestHandleGetOutcomesStats_DefaultPeriod(t *testing.T) {
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

	// Create request without period param
	req := httptest.NewRequest(http.MethodGet, "/api/stats/outcomes", nil)
	rr := httptest.NewRecorder()

	server.handleGetOutcomesStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response OutcomesResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should default to "all" period
	if response.Period != "all" {
		t.Errorf("expected period=all, got %s", response.Period)
	}

	// Empty database should return zeros
	if response.Total != 0 {
		t.Errorf("expected total=0, got %d", response.Total)
	}

	// Should have all three outcome categories with zeros
	if response.Outcomes["completed"].Count != 0 {
		t.Errorf("expected completed count=0, got %d", response.Outcomes["completed"].Count)
	}
	if response.Outcomes["with_retries"].Count != 0 {
		t.Errorf("expected with_retries count=0, got %d", response.Outcomes["with_retries"].Count)
	}
	if response.Outcomes["failed"].Count != 0 {
		t.Errorf("expected failed count=0, got %d", response.Outcomes["failed"].Count)
	}
}

func TestHandleGetOutcomesStats_InvalidPeriod(t *testing.T) {
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
		name   string
		period string
	}{
		{"invalid", "invalid"},
		{"1h", "1h"},
		{"90d", "90d"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/stats/outcomes?period="+tc.period, nil)
			rr := httptest.NewRecorder()

			server.handleGetOutcomesStats(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", rr.Code)
			}
		})
	}
}

func TestHandleGetOutcomesStats_WithTasks(t *testing.T) {
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

	// Create 3 completed tasks (no retries)
	for i := 1; i <= 3; i++ {
		taskID := "TASK-" + fmt.Sprintf("%03d", i)
		tsk := task.New(taskID, "Completed task "+fmt.Sprintf("%d", i))
		tsk.Status = task.StatusCompleted
		completedAt := now.Add(-time.Hour)
		tsk.CompletedAt = &completedAt
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("failed to save task: %v", err)
		}
		// Save state without retry context
		st := &storage.SpecInfo{
			TaskID:    taskID,
			Content:   "test spec",
			Source:    "test",
			CreatedAt: now,
			UpdatedAt: now,
		}
		_ = st // Not needed for this test
	}

	// Create 2 failed tasks
	for i := 1; i <= 2; i++ {
		taskID := "TASK-" + fmt.Sprintf("%03d", 100+i)
		tsk := task.New(taskID, "Failed task "+fmt.Sprintf("%d", i))
		tsk.Status = task.StatusFailed
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

	req := httptest.NewRequest(http.MethodGet, "/api/stats/outcomes", nil)
	rr := httptest.NewRecorder()

	server.handleGetOutcomesStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response OutcomesResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have 5 total tasks
	if response.Total != 5 {
		t.Errorf("expected total=5, got %d", response.Total)
	}

	// Check counts
	if response.Outcomes["completed"].Count != 3 {
		t.Errorf("expected completed count=3, got %d", response.Outcomes["completed"].Count)
	}
	if response.Outcomes["with_retries"].Count != 0 {
		t.Errorf("expected with_retries count=0, got %d", response.Outcomes["with_retries"].Count)
	}
	if response.Outcomes["failed"].Count != 2 {
		t.Errorf("expected failed count=2, got %d", response.Outcomes["failed"].Count)
	}

	// Check percentages (60% completed, 0% retries, 40% failed)
	if response.Outcomes["completed"].Percentage < 59.9 || response.Outcomes["completed"].Percentage > 60.1 {
		t.Errorf("expected completed percentage≈60, got %.2f", response.Outcomes["completed"].Percentage)
	}
	if response.Outcomes["failed"].Percentage < 39.9 || response.Outcomes["failed"].Percentage > 40.1 {
		t.Errorf("expected failed percentage≈40, got %.2f", response.Outcomes["failed"].Percentage)
	}
}

func TestHandleGetOutcomesStats_PeriodFilter(t *testing.T) {
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

	// Create task completed 25 hours ago (outside 24h window)
	tsk1 := task.New("TASK-001", "Old completed task")
	tsk1.Status = task.StatusCompleted
	completedAt1 := now.Add(-25 * time.Hour)
	tsk1.CompletedAt = &completedAt1
	if err := backend.SaveTask(tsk1); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create task completed 1 hour ago (within 24h window)
	tsk2 := task.New("TASK-002", "Recent completed task")
	tsk2.Status = task.StatusCompleted
	completedAt2 := now.Add(-1 * time.Hour)
	tsk2.CompletedAt = &completedAt2
	if err := backend.SaveTask(tsk2); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Close backend before creating server
	_ = backend.Close()

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	// Test 24h period - should only see TASK-002
	req := httptest.NewRequest(http.MethodGet, "/api/stats/outcomes?period=24h", nil)
	rr := httptest.NewRecorder()

	server.handleGetOutcomesStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response OutcomesResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Period != "24h" {
		t.Errorf("expected period=24h, got %s", response.Period)
	}

	// Should only count TASK-002
	if response.Total != 1 {
		t.Errorf("expected total=1, got %d", response.Total)
	}
	if response.Outcomes["completed"].Count != 1 {
		t.Errorf("expected completed count=1, got %d", response.Outcomes["completed"].Count)
	}
}

func TestHandleGetOutcomesStats_PercentageSum(t *testing.T) {
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
	completedAt := now.Add(-time.Hour)

	// Create 10 completed, 3 failed
	for i := 1; i <= 10; i++ {
		tsk := task.New("TASK-"+fmt.Sprintf("%03d", i), "Completed")
		tsk.Status = task.StatusCompleted
		tsk.CompletedAt = &completedAt
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("failed to save task: %v", err)
		}
	}

	for i := 1; i <= 3; i++ {
		tsk := task.New("TASK-"+fmt.Sprintf("%03d", 100+i), "Failed")
		tsk.Status = task.StatusFailed
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("failed to save task: %v", err)
		}
	}

	_ = backend.Close()

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/stats/outcomes", nil)
	rr := httptest.NewRecorder()

	server.handleGetOutcomesStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response OutcomesResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Percentages should sum to approximately 100%
	total := response.Outcomes["completed"].Percentage +
		response.Outcomes["with_retries"].Percentage +
		response.Outcomes["failed"].Percentage

	if total < 99.9 || total > 100.1 {
		t.Errorf("percentages should sum to ≈100%%, got %.2f", total)
	}
}
