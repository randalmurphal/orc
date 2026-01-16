#!/bin/bash
#
# doc-lint.sh - Enforce CLAUDE.md line limits per AI documentation standards
#
# Thresholds (configurable via .orc/config.yaml):
#   root:    180 lines (project overview)
#   package: 150 lines (typical package)
#   complex: 250 lines (complex packages: executor, storage)
#   web:     200 lines (frontend)
#
# Tolerance: 15% grace period (WARN vs BLOCK)
#
# Exit codes:
#   0 - All files pass
#   1 - At least one file exceeds threshold + tolerance

set -euo pipefail

# Defaults (override via .orc/config.yaml if available)
THRESHOLD_ROOT=180
THRESHOLD_PACKAGE=150
THRESHOLD_COMPLEX=250
THRESHOLD_WEB=200
TOLERANCE=0.15  # 15%

# Complex packages that get higher threshold
COMPLEX_PACKAGES=(
    "internal/executor"
    "internal/storage"
    "internal/task"
    "internal/git"
)

# Colors
RED='\033[0;31m'
YELLOW='\033[0;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Find project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load config if available (basic yaml parsing)
CONFIG_FILE="$PROJECT_ROOT/.orc/config.yaml"
if [[ -f "$CONFIG_FILE" ]]; then
    # Extract docs.lint values if present
    if grep -q "^docs:" "$CONFIG_FILE" 2>/dev/null; then
        THRESHOLD_ROOT=$(grep -A20 "^docs:" "$CONFIG_FILE" | grep "root:" | head -1 | awk '{print $2}' || echo $THRESHOLD_ROOT)
        THRESHOLD_PACKAGE=$(grep -A20 "^docs:" "$CONFIG_FILE" | grep "package:" | head -1 | awk '{print $2}' || echo $THRESHOLD_PACKAGE)
        THRESHOLD_COMPLEX=$(grep -A20 "^docs:" "$CONFIG_FILE" | grep "complex:" | head -1 | awk '{print $2}' || echo $THRESHOLD_COMPLEX)
        THRESHOLD_WEB=$(grep -A20 "^docs:" "$CONFIG_FILE" | grep "web:" | head -1 | awk '{print $2}' || echo $THRESHOLD_WEB)
        TOLERANCE=$(grep -A20 "^docs:" "$CONFIG_FILE" | grep "tolerance:" | head -1 | awk '{print $2}' || echo $TOLERANCE)
    fi
fi

# Calculate threshold with tolerance
calc_with_tolerance() {
    local threshold=$1
    echo $(awk "BEGIN {printf \"%.0f\", $threshold * (1 + $TOLERANCE)}")
}

# Check if path is a complex package
is_complex_package() {
    local path="$1"
    for pkg in "${COMPLEX_PACKAGES[@]}"; do
        if [[ "$path" == *"$pkg"* ]]; then
            return 0
        fi
    done
    return 1
}

# Get threshold for a file
get_threshold() {
    local file="$1"
    local rel_path="${file#$PROJECT_ROOT/}"

    # Root CLAUDE.md
    if [[ "$rel_path" == "CLAUDE.md" ]]; then
        echo $THRESHOLD_ROOT
        return
    fi

    # Web frontend
    if [[ "$rel_path" == web/* ]]; then
        echo $THRESHOLD_WEB
        return
    fi

    # Complex packages
    if is_complex_package "$rel_path"; then
        echo $THRESHOLD_COMPLEX
        return
    fi

    # Default package
    echo $THRESHOLD_PACKAGE
}

# Main
main() {
    local has_failure=0
    local has_warning=0

    echo "=== CLAUDE.md Line Limit Check ==="
    echo ""
    printf "%-50s %8s %8s %10s\n" "File" "Lines" "Limit" "Status"
    printf "%-50s %8s %8s %10s\n" "----" "-----" "-----" "------"

    # Find all CLAUDE.md files
    while IFS= read -r file; do
        if [[ -f "$file" ]]; then
            local lines=$(wc -l < "$file")
            local rel_path="${file#$PROJECT_ROOT/}"
            local threshold=$(get_threshold "$file")
            local max_allowed=$(calc_with_tolerance $threshold)

            local status=""
            local color=""

            if (( lines > max_allowed )); then
                status="BLOCK"
                color=$RED
                has_failure=1
            elif (( lines > threshold )); then
                status="WARN"
                color=$YELLOW
                has_warning=1
            else
                status="OK"
                color=$GREEN
            fi

            printf "%-50s %8d %8d ${color}%10s${NC}\n" "$rel_path" "$lines" "$threshold" "$status"
        fi
    done < <(find "$PROJECT_ROOT" -name "CLAUDE.md" -type f \
        -not -path "*/node_modules/*" \
        -not -path "*/.git/*" \
        -not -path "*/vendor/*" \
        -not -path "*/web-svelte-archive/*" \
        | sort)

    echo ""
    echo "Thresholds: root=$THRESHOLD_ROOT, package=$THRESHOLD_PACKAGE, complex=$THRESHOLD_COMPLEX, web=$THRESHOLD_WEB"
    echo "Tolerance: ${TOLERANCE} ($(awk "BEGIN {printf \"%.0f\", $TOLERANCE * 100}")% grace)"
    echo ""

    if (( has_failure )); then
        echo -e "${RED}FAILED: One or more files exceed threshold + tolerance${NC}"
        echo ""
        echo "To fix:"
        echo "  1. Extract detailed content to reference docs (SCHEMA.md, ENDPOINTS.md, etc.)"
        echo "  2. Add pointer in CLAUDE.md: 'See [file.md](file.md) for details'"
        echo "  3. Re-run: make doc-lint"
        exit 1
    elif (( has_warning )); then
        echo -e "${YELLOW}WARNING: Files within tolerance but over threshold${NC}"
        echo "Consider reducing before they become blockers."
        exit 0
    else
        echo -e "${GREEN}PASSED: All files within limits${NC}"
        exit 0
    fi
}

main "$@"
