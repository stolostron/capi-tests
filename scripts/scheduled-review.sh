#!/usr/bin/env bash
# Run the scheduled repository review locally using Claude Code.
#
# Prerequisites:
#   - ANTHROPIC_API_KEY set in environment (or ~/.anthropic/api_key)
#   - claude CLI installed (npm install -g @anthropic-ai/claude-code)
#   - gh CLI authenticated
#
# Usage:
#   ./scripts/scheduled-review.sh              # Run review, save output to results/
#   ./scripts/scheduled-review.sh --dry-run    # Show what would be reviewed without creating PRs
#   ./scripts/scheduled-review.sh --hours 48   # Review last 48 hours instead of 24

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
HOURS=24
DRY_RUN=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --hours)
            HOURS="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [--dry-run] [--hours N]"
            echo ""
            echo "Options:"
            echo "  --dry-run    Show what would be reviewed without creating PRs"
            echo "  --hours N    Review last N hours (default: 24)"
            echo "  --help       Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Run '$0 --help' for usage"
            exit 1
            ;;
    esac
done

# Check prerequisites
check_prerequisites() {
    local missing=()

    if ! command -v claude &>/dev/null; then
        missing+=("claude (install: npm install -g @anthropic-ai/claude-code)")
    fi

    if ! command -v gh &>/dev/null; then
        missing+=("gh (install: brew install gh)")
    fi

    if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
        # Check if claude can authenticate via other means
        if [[ ! -f "$HOME/.anthropic/api_key" ]]; then
            missing+=("ANTHROPIC_API_KEY environment variable")
        fi
    fi

    if [[ ${#missing[@]} -gt 0 ]]; then
        echo "Missing prerequisites:"
        for item in "${missing[@]}"; do
            echo "  - $item"
        done
        exit 1
    fi

    # Verify gh is authenticated
    if ! gh auth status &>/dev/null; then
        echo "Error: gh CLI is not authenticated. Run 'gh auth login' first."
        exit 1
    fi
}

check_prerequisites

# Setup output directory
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
OUTPUT_DIR="$REPO_DIR/results/scheduled-review"
OUTPUT_FILE="$OUTPUT_DIR/review-${TIMESTAMP}.md"
mkdir -p "$OUTPUT_DIR"

echo "========================================"
echo "=== Scheduled Repository Review ==="
echo "========================================"
echo ""
echo "Repository: stolostron/capi-tests"
echo "Time window: last ${HOURS} hours"
echo "Dry run: ${DRY_RUN}"
echo "Output: ${OUTPUT_FILE}"
echo ""

# Build the prompt
COMMAND_FILE="$REPO_DIR/.claude/commands/scheduled-review.md"
if [[ ! -f "$COMMAND_FILE" ]]; then
    echo "Error: Command file not found: $COMMAND_FILE"
    exit 1
fi

PROMPT=$(cat "$COMMAND_FILE")

# Adjust time window if not default
if [[ "$HOURS" != "24" ]]; then
    PROMPT=$(echo "$PROMPT" | sed "s/86400/$((HOURS * 3600))/g")
    PROMPT=$(echo "$PROMPT" | sed "s/last 24 hours/last ${HOURS} hours/g")
fi

# Add dry-run instruction if requested
if [[ "$DRY_RUN" == "true" ]]; then
    PROMPT="$PROMPT

IMPORTANT: This is a DRY RUN. Do NOT create any branches, commits, or pull requests.
Instead, for each issue or PR you would act on, describe what you would do:
- What branch you would create
- What changes you would make
- What the draft PR would contain
Output the summary report as if the actions were taken, but mark each action as '[DRY RUN]'."
fi

# Run Claude Code headlessly
echo "Starting Claude Code review..."
echo ""

cd "$REPO_DIR"
claude -p "$PROMPT" \
    --allowedTools "Bash,Read,Write,Edit,Glob,Grep,Agent" \
    2>&1 | tee "$OUTPUT_FILE"

EXIT_CODE=${PIPESTATUS[0]}

echo ""
echo "========================================"
echo "Review output saved to: $OUTPUT_FILE"
echo "========================================"

# Symlink latest
ln -sf "review-${TIMESTAMP}.md" "$OUTPUT_DIR/latest.md"

exit $EXIT_CODE
