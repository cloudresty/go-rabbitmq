#!/bin/bash

# GitHub Actions Auto Release Setup Script
# This script helps you configure the required secrets and settings for the automated release pipeline

set -e

echo "Setting up GitHub Actions Auto Release Pipeline"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if we're in a git repository
if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo -e "${RED}Error: This script must be run from within a git repository${NC}"
    exit 1
fi

# Get repository information
REPO_URL=$(git config --get remote.origin.url)
REPO_NAME=$(basename -s .git "$REPO_URL")
REPO_OWNER=$(basename -s .git "$(dirname "$REPO_URL")" | sed 's/.*://')

echo -e "${BLUE}Repository: ${REPO_OWNER}/${REPO_NAME}${NC}"
echo

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo -e "${RED}Error: GitHub CLI (gh) is required but not installed${NC}"
    echo "Please install it from: https://cli.github.com/"
    exit 1
fi

# Check if user is authenticated with gh
if ! gh auth status >/dev/null 2>&1; then
    echo -e "${YELLOW}GitHub CLI not authenticated. Please run 'gh auth login' first${NC}"
    exit 1
fi

echo -e "${GREEN}✓ GitHub CLI is installed and authenticated${NC}"

# Function to set a secret
set_secret() {
    local secret_name=$1
    local secret_description=$2
    local secret_value=$3

    if [ -z "$secret_value" ]; then
        echo -e "${YELLOW}Skipping $secret_name (no value provided)${NC}"
        return
    fi

    if gh secret set "$secret_name" --body "$secret_value"; then
        echo -e "${GREEN}✓ Set secret: $secret_name${NC}"
    else
        echo -e "${RED}✗ Failed to set secret: $secret_name${NC}"
    fi
}

# Check current branch protection
echo -e "${BLUE}Checking branch protection settings...${NC}"

echo -e "${BLUE}=== WORKFLOW SUMMARY ===${NC}"
echo "The following GitHub Actions workflows have been configured:"
echo
echo -e "${GREEN}1. Auto Dependency Update (.github/workflows/auto-dependency-update.yaml)${NC}"
echo "   • Runs every Monday at 2 AM UTC"
echo "   • Checks for updates to:"
echo "     - github.com/cloudresty/emit (current: v1.2.5)"
echo "     - github.com/cloudresty/ulid (current: v1.2.1)"
echo "     - github.com/cloudresty/go-env (current: v1.0.1)"
echo "     - github.com/rabbitmq/amqp091-go (current: v1.10.0)"
echo "     - google.golang.org/protobuf (current: v1.36.6)"
echo "   • Creates PR to develop branch if updates found"
echo "   • Auto-merges if all tests pass"
echo
echo -e "${GREEN}2. Auto Merge to Main (.github/workflows/auto-merge-to-main.yaml)${NC}"
echo "   • Triggers on push to develop branch"
echo "   • Runs comprehensive test suite:"
echo "     - Unit tests with race detection"
echo "     - Integration tests with RabbitMQ streams"
echo "     - Linting and code quality checks"
echo "     - Module verification"
echo "   • Auto-merges develop to main if all tests pass"
echo
echo -e "${GREEN}3. Auto Release (.github/workflows/auto-release.yaml)${NC}"
echo "   • Triggers on push to main branch"
echo "   • Determines version bump from commit messages:"
echo "     - 'BREAKING' or 'major' -> major version bump"
echo "     - 'feat' or 'minor' -> minor version bump"
echo "     - 'fix', 'bug', 'chore', 'deps' -> patch version bump"
echo "   • Creates GitHub release with changelog"
echo "   • Supports manual triggering with custom version bump"
echo
echo -e "${BLUE}=== AUTOMATION FLOW ===${NC}"
echo "1. Weekly dependency check (Mondays 2 AM UTC)"
echo "2. Updates found → PR created to develop"
echo "3. Tests pass → Auto-merge PR to develop"
echo "4. Develop updated → Auto-merge to main (if tests pass)"
echo "5. Main updated → Auto-release created"
echo "6. Notifications sent (if configured)"
echo

# Create PAT token instructions
echo
echo -e "${YELLOW}=== REQUIRED SETUP ===${NC}"
echo "1. Personal Access Token (PAT)"
echo "   - Go to: https://github.com/settings/tokens"
echo "   - Create a new token with these permissions:"
echo "     • repo (Full control of private repositories)"
echo "     • workflow (Update GitHub Action workflows)"
echo "     • write:packages (Upload packages to GitHub Package Registry)"
echo "   - Copy the token value"
echo

read -p "Enter your GitHub PAT token (or press Enter to skip): " -s CLOUDRESTY_GITBOT_PAT
echo

if [ ! -z "$CLOUDRESTY_GITBOT_PAT" ]; then
    set_secret "CLOUDRESTY_GITBOT_PAT" "Cloudresty GitBot Personal Access Token for automation" "$CLOUDRESTY_GITBOT_PAT"
else
    echo -e "${YELLOW}  CLOUDRESTY_GITBOT_PAT not set. Some features may not work properly.${NC}"
fi

# Optional setup complete - using GitHub App for notifications

# Set up branch protection
echo
echo -e "${BLUE}Setting up branch protection...${NC}"

# Protect main branch
if gh api repos/$REPO_OWNER/$REPO_NAME/branches/main/protection \
    --method PUT \
    --field required_status_checks='{"strict":true,"contexts":["test-full-suite"]}' \
    --field enforce_admins=false \
    --field required_pull_request_reviews='{"required_approving_review_count":1,"dismiss_stale_reviews":true}' \
    --field restrictions=null >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Protected main branch${NC}"
else
    echo -e "${YELLOW}  Could not set up branch protection (may require admin privileges)${NC}"
fi

# Enable auto-merge
echo
echo -e "${BLUE}Enabling repository features...${NC}"

# Enable auto-merge
if gh api repos/$REPO_OWNER/$REPO_NAME \
    --method PATCH \
    --field allow_auto_merge=true >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Enabled auto-merge${NC}"
else
    echo -e "${YELLOW}  Could not enable auto-merge${NC}"
fi

# Create initial workflow run
echo
echo -e "${BLUE}Testing workflow setup...${NC}"

if gh workflow run auto-dependency-update.yaml >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Triggered test workflow run${NC}"
else
    echo -e "${YELLOW}  Could not trigger workflow (workflows may need to be pushed first)${NC}"
fi

echo
echo -e "${GREEN} Setup complete!${NC}"
echo
echo -e "${BLUE}Next steps:${NC}"
echo "1. Push these workflow files to your repository"
echo "2. The dependency update workflow will run automatically every Monday"
echo "3. Go version checks will run every Tuesday"
echo "4. Any push to develop will trigger the full test suite"
echo "5. Successful tests will auto-merge to main and create a release"
echo
echo -e "${BLUE}Manual triggers:${NC}"
echo "• Force dependency update: gh workflow run auto-dependency-update.yaml"
echo "• Force Go version check: gh workflow run go-version-update.yaml"
echo
echo -e "${BLUE}Monitor workflows at:${NC}"
echo "https://github.com/$REPO_OWNER/$REPO_NAME/actions"
echo
echo -e "${YELLOW}Note: Make sure to review the auto-release-config.yaml file and adjust settings as needed.${NC}"
