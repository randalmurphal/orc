// Package executor contains task execution logic.
package executor

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ProcessInfo represents information about a running process.
type ProcessInfo struct {
	PID          int
	PPID         int
	Command      string
	MemoryMB     float64
	IsMCP        bool // true if command matches MCP-related patterns (browsers)
	IsOrcRelated bool // true if command matches orc-spawned process patterns
}

// ProcessSnapshot captures the state of system processes at a point in time.
type ProcessSnapshot struct {
	Timestamp     time.Time
	Processes     []ProcessInfo
	TotalMemoryMB float64
	ProcessCount  int
}

// ResourceTrackerConfig configures the resource tracker behavior.
type ResourceTrackerConfig struct {
	Enabled            bool
	MemoryThresholdMB  int
	LogOrphanedMCPOnly bool // deprecated: use FilterSystemProcesses instead
	// FilterSystemProcesses controls whether to filter out system processes from orphan detection.
	// When true (default), only processes that match orc-related patterns (claude, node, playwright,
	// chromium, etc.) are flagged as potential orphans. System processes like systemd-timedated,
	// snapper, etc. are ignored even if they started during task execution.
	// When false, all new orphaned processes are flagged (original behavior, prone to false positives).
	FilterSystemProcesses bool
}

// ResourceTracker tracks process and memory state to detect orphaned processes.
type ResourceTracker struct {
	config         ResourceTrackerConfig
	logger         *slog.Logger
	beforeSnapshot *ProcessSnapshot
	afterSnapshot  *ProcessSnapshot
}

// mcpProcessPattern matches MCP-related process names (browsers).
var mcpProcessPattern = regexp.MustCompile(`(?i)(playwright|chromium|chrome|firefox|webkit|puppeteer|selenium)`)

// orcRelatedProcessPattern matches processes that orc might spawn.
// This includes:
// - Browser automation: playwright, chromium, chrome, firefox, webkit, puppeteer, selenium
// - Claude Code and Node.js: claude, node, npx, npm
// - MCP servers: mcp-server, mcp
// System processes (systemd, snapper, etc.) should NOT match this pattern.
var orcRelatedProcessPattern = regexp.MustCompile(`(?i)(playwright|chromium|chrome|firefox|webkit|puppeteer|selenium|claude|node(?:$|[^a-z])|npx|npm|mcp)`)

// NewResourceTracker creates a new resource tracker.
func NewResourceTracker(config ResourceTrackerConfig, logger *slog.Logger) *ResourceTracker {
	if logger == nil {
		logger = slog.Default()
	}
	return &ResourceTracker{
		config: config,
		logger: logger,
	}
}

// SnapshotBefore takes a snapshot of system processes before task execution.
func (rt *ResourceTracker) SnapshotBefore() error {
	if !rt.config.Enabled {
		return nil
	}

	snapshot, err := rt.captureSnapshot()
	if err != nil {
		return fmt.Errorf("capture before snapshot: %w", err)
	}

	rt.beforeSnapshot = snapshot
	rt.logger.Info("resource snapshot taken (before)",
		"processes", snapshot.ProcessCount,
		"memory_mb", fmt.Sprintf("%.1f", snapshot.TotalMemoryMB),
	)
	return nil
}

// SnapshotAfter takes a snapshot of system processes after task execution.
func (rt *ResourceTracker) SnapshotAfter() error {
	if !rt.config.Enabled {
		return nil
	}

	snapshot, err := rt.captureSnapshot()
	if err != nil {
		return fmt.Errorf("capture after snapshot: %w", err)
	}

	rt.afterSnapshot = snapshot
	rt.logger.Info("resource snapshot taken (after)",
		"processes", snapshot.ProcessCount,
		"memory_mb", fmt.Sprintf("%.1f", snapshot.TotalMemoryMB),
	)
	return nil
}

// DetectOrphans finds processes that were spawned during task execution
// and are still running (orphaned) after the task completes.
func (rt *ResourceTracker) DetectOrphans() []ProcessInfo {
	if !rt.config.Enabled || rt.beforeSnapshot == nil || rt.afterSnapshot == nil {
		return nil
	}

	// Build set of PIDs from before snapshot
	beforePIDs := make(map[int]bool)
	for _, p := range rt.beforeSnapshot.Processes {
		beforePIDs[p.PID] = true
	}

	// Build set of all current PIDs for parent checking
	afterPIDSet := make(map[int]bool)
	for _, p := range rt.afterSnapshot.Processes {
		afterPIDSet[p.PID] = true
	}

	// Find orphaned processes
	var orphans []ProcessInfo
	for _, p := range rt.afterSnapshot.Processes {
		// Skip if process existed before task
		if beforePIDs[p.PID] {
			continue
		}

		// Check if this is an orphan:
		// - Parent is init (PID 1) which means it was reparented
		// - Parent no longer exists
		isOrphan := p.PPID == 1 || !afterPIDSet[p.PPID]

		if isOrphan {
			// Filter based on configuration
			// Priority: FilterSystemProcesses (new) > LogOrphanedMCPOnly (deprecated)
			if rt.config.FilterSystemProcesses {
				// Only flag orc-related processes (claude, node, playwright, etc.)
				// This filters out system processes like systemd-timedated, snapper, etc.
				if !p.IsOrcRelated {
					continue
				}
			} else if rt.config.LogOrphanedMCPOnly {
				// Deprecated: only flag MCP browser processes
				if !p.IsMCP {
					continue
				}
			}
			// If neither filter is enabled, all orphans are flagged (original behavior)
			orphans = append(orphans, p)
		}
	}

	if len(orphans) > 0 {
		// Build process list for logging
		var procList []string
		for _, p := range orphans {
			tag := ""
			if p.IsMCP {
				tag = " [MCP]"
			} else if p.IsOrcRelated {
				tag = " [orc]"
			}
			procList = append(procList, fmt.Sprintf("%s (PID=%d)%s", p.Command, p.PID, tag))
		}

		rt.logger.Warn("orphaned processes detected",
			"count", len(orphans),
			"processes", strings.Join(procList, ", "),
		)
	}

	return orphans
}

// CheckMemoryGrowth checks if memory grew beyond the threshold and logs a warning.
// Returns the memory delta in MB.
func (rt *ResourceTracker) CheckMemoryGrowth() float64 {
	if !rt.config.Enabled || rt.beforeSnapshot == nil || rt.afterSnapshot == nil {
		return 0
	}

	delta := rt.afterSnapshot.TotalMemoryMB - rt.beforeSnapshot.TotalMemoryMB

	if delta > float64(rt.config.MemoryThresholdMB) {
		rt.logger.Warn("memory growth exceeded threshold",
			"delta_mb", fmt.Sprintf("%.1f", delta),
			"threshold_mb", rt.config.MemoryThresholdMB,
			"before_mb", fmt.Sprintf("%.1f", rt.beforeSnapshot.TotalMemoryMB),
			"after_mb", fmt.Sprintf("%.1f", rt.afterSnapshot.TotalMemoryMB),
		)
	}

	return delta
}

// Reset clears the snapshots for reuse.
func (rt *ResourceTracker) Reset() {
	rt.beforeSnapshot = nil
	rt.afterSnapshot = nil
}

// GetBeforeSnapshot returns the before snapshot for testing.
func (rt *ResourceTracker) GetBeforeSnapshot() *ProcessSnapshot {
	return rt.beforeSnapshot
}

// GetAfterSnapshot returns the after snapshot for testing.
func (rt *ResourceTracker) GetAfterSnapshot() *ProcessSnapshot {
	return rt.afterSnapshot
}

// captureSnapshot enumerates all processes and captures their state.
func (rt *ResourceTracker) captureSnapshot() (*ProcessSnapshot, error) {
	var processes []ProcessInfo
	var err error

	switch runtime.GOOS {
	case "linux":
		processes, err = rt.enumerateLinux()
	case "darwin":
		processes, err = rt.enumerateDarwin()
	case "windows":
		processes, err = rt.enumerateWindows()
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	if err != nil {
		return nil, fmt.Errorf("enumerate processes: %w", err)
	}

	// Calculate totals
	var totalMemory float64
	for _, p := range processes {
		totalMemory += p.MemoryMB
	}

	return &ProcessSnapshot{
		Timestamp:     time.Now(),
		Processes:     processes,
		TotalMemoryMB: totalMemory,
		ProcessCount:  len(processes),
	}, nil
}

// enumerateLinux lists processes using /proc filesystem.
func (rt *ResourceTracker) enumerateLinux() ([]ProcessInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("read /proc: %w", err)
	}

	var processes []ProcessInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue // Not a PID directory
		}

		info, err := rt.readLinuxProcess(pid)
		if err != nil {
			continue // Process may have exited
		}

		processes = append(processes, info)
	}

	return processes, nil
}

// readLinuxProcess reads process info from /proc/[pid].
func (rt *ResourceTracker) readLinuxProcess(pid int) (ProcessInfo, error) {
	info := ProcessInfo{PID: pid}

	// Read /proc/[pid]/stat for PPID
	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	statData, err := os.ReadFile(statPath)
	if err != nil {
		return info, err
	}

	// Parse stat: pid (comm) state ppid ...
	// The comm field is in parentheses and may contain spaces
	statStr := string(statData)
	lastParen := strings.LastIndex(statStr, ")")
	if lastParen == -1 {
		return info, fmt.Errorf("invalid stat format")
	}

	// Fields after the command
	fields := strings.Fields(statStr[lastParen+1:])
	if len(fields) < 2 {
		return info, fmt.Errorf("insufficient stat fields")
	}

	info.PPID, _ = strconv.Atoi(fields[1])

	// Read /proc/[pid]/cmdline for command
	cmdPath := filepath.Join("/proc", strconv.Itoa(pid), "cmdline")
	cmdData, err := os.ReadFile(cmdPath)
	if err == nil && len(cmdData) > 0 {
		// cmdline uses null bytes as separators
		cmdStr := strings.ReplaceAll(string(cmdData), "\x00", " ")
		info.Command = strings.TrimSpace(cmdStr)
		// Truncate long commands
		if len(info.Command) > 100 {
			info.Command = info.Command[:100] + "..."
		}
	} else {
		// Fall back to comm
		commPath := filepath.Join("/proc", strconv.Itoa(pid), "comm")
		commData, _ := os.ReadFile(commPath)
		info.Command = strings.TrimSpace(string(commData))
	}

	// Read /proc/[pid]/status for memory (VmRSS)
	statusPath := filepath.Join("/proc", strconv.Itoa(pid), "status")
	statusFile, err := os.Open(statusPath)
	if err == nil {
		defer func() { _ = statusFile.Close() }()
		scanner := bufio.NewScanner(statusFile)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "VmRSS:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					rssKB, _ := strconv.ParseFloat(fields[1], 64)
					info.MemoryMB = rssKB / 1024
				}
				break
			}
		}
	}

	// Check if MCP-related (browsers)
	info.IsMCP = mcpProcessPattern.MatchString(info.Command)
	// Check if orc-related (any process orc might spawn)
	info.IsOrcRelated = orcRelatedProcessPattern.MatchString(info.Command)

	return info, nil
}

// enumerateDarwin lists processes using ps command.
func (rt *ResourceTracker) enumerateDarwin() ([]ProcessInfo, error) {
	// ps -axo pid,ppid,rss,comm
	cmd := exec.Command("ps", "-axo", "pid,ppid,rss,comm")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("execute ps: %w", err)
	}

	var processes []ProcessInfo
	lines := strings.Split(string(output), "\n")

	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // Skip header and empty lines
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		pid, _ := strconv.Atoi(fields[0])
		ppid, _ := strconv.Atoi(fields[1])
		rssKB, _ := strconv.ParseFloat(fields[2], 64)
		command := strings.Join(fields[3:], " ")

		if len(command) > 100 {
			command = command[:100] + "..."
		}

		info := ProcessInfo{
			PID:          pid,
			PPID:         ppid,
			Command:      command,
			MemoryMB:     rssKB / 1024,
			IsMCP:        mcpProcessPattern.MatchString(command),
			IsOrcRelated: orcRelatedProcessPattern.MatchString(command),
		}

		processes = append(processes, info)
	}

	return processes, nil
}

// enumerateWindows lists processes using WMIC or tasklist.
func (rt *ResourceTracker) enumerateWindows() ([]ProcessInfo, error) {
	// Try WMIC first (more detailed)
	cmd := exec.Command("wmic", "process", "get", "ProcessId,ParentProcessId,WorkingSetSize,Name", "/format:csv")
	output, err := cmd.Output()
	if err == nil {
		return rt.parseWMICOutput(string(output))
	}

	// Fall back to tasklist
	cmd = exec.Command("tasklist", "/fo", "csv", "/v")
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("execute tasklist: %w", err)
	}

	return rt.parseTasklistOutput(string(output))
}

// parseWMICOutput parses WMIC CSV output.
func (rt *ResourceTracker) parseWMICOutput(output string) ([]ProcessInfo, error) {
	var processes []ProcessInfo
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // Skip header and empty lines
		}

		// CSV format: Node,Name,ParentProcessId,ProcessId,WorkingSetSize
		fields := strings.Split(line, ",")
		if len(fields) < 5 {
			continue
		}

		name := strings.TrimSpace(fields[1])
		ppid, _ := strconv.Atoi(strings.TrimSpace(fields[2]))
		pid, _ := strconv.Atoi(strings.TrimSpace(fields[3]))
		wsBytes, _ := strconv.ParseFloat(strings.TrimSpace(fields[4]), 64)

		info := ProcessInfo{
			PID:          pid,
			PPID:         ppid,
			Command:      name,
			MemoryMB:     wsBytes / (1024 * 1024),
			IsMCP:        mcpProcessPattern.MatchString(name),
			IsOrcRelated: orcRelatedProcessPattern.MatchString(name),
		}

		processes = append(processes, info)
	}

	return processes, nil
}

// parseTasklistOutput parses tasklist CSV output (limited - no PPID).
func (rt *ResourceTracker) parseTasklistOutput(output string) ([]ProcessInfo, error) {
	var processes []ProcessInfo
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // Skip header and empty lines
		}

		// Parse CSV (with quotes)
		reader := strings.NewReader(line)
		var fields []string
		var field strings.Builder
		inQuotes := false

		for {
			r, _, err := reader.ReadRune()
			if err != nil {
				if field.Len() > 0 {
					fields = append(fields, field.String())
				}
				break
			}

			if r == '"' {
				inQuotes = !inQuotes
			} else if r == ',' && !inQuotes {
				fields = append(fields, field.String())
				field.Reset()
			} else {
				field.WriteRune(r)
			}
		}

		if len(fields) < 5 {
			continue
		}

		name := strings.TrimSpace(fields[0])
		pid, _ := strconv.Atoi(strings.TrimSpace(fields[1]))
		// Memory is in format "123,456 K"
		memStr := strings.ReplaceAll(fields[4], ",", "")
		memStr = strings.ReplaceAll(memStr, " K", "")
		memKB, _ := strconv.ParseFloat(strings.TrimSpace(memStr), 64)

		info := ProcessInfo{
			PID:          pid,
			PPID:         0, // tasklist doesn't provide PPID
			Command:      name,
			MemoryMB:     memKB / 1024,
			IsMCP:        mcpProcessPattern.MatchString(name),
			IsOrcRelated: orcRelatedProcessPattern.MatchString(name),
		}

		processes = append(processes, info)
	}

	return processes, nil
}

// IsMCPProcess checks if a command string matches MCP-related patterns.
func IsMCPProcess(command string) bool {
	return mcpProcessPattern.MatchString(command)
}

// IsOrcRelatedProcess checks if a command string matches processes that orc might spawn.
func IsOrcRelatedProcess(command string) bool {
	return orcRelatedProcessPattern.MatchString(command)
}

// RunResourceAnalysis performs resource tracking analysis.
// Takes an after snapshot, detects orphans, checks memory growth, and resets the tracker.
// Safe to call with nil tracker (no-op).
func RunResourceAnalysis(tracker *ResourceTracker, logger *slog.Logger) {
	if tracker == nil {
		return
	}

	// Take after snapshot
	if err := tracker.SnapshotAfter(); err != nil {
		logger.Warn("failed to take resource snapshot after task", "error", err)
		return
	}

	// Detect orphaned processes (logs warnings for any found)
	tracker.DetectOrphans()

	// Check memory growth against threshold (logs warning if exceeded)
	tracker.CheckMemoryGrowth()

	// Reset tracker for next task
	tracker.Reset()
}
