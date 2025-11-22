#!/bin/bash
set -euo pipefail

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║   Documentation Cleanup - Removing Outdated Files             ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Create archive directory
ARCHIVE_DIR="docs/archive/$(date +%Y%m%d)"
mkdir -p "$ARCHIVE_DIR"

echo -e "${BLUE}Archive directory: $ARCHIVE_DIR${NC}"
echo ""

# Files to remove (outdated/duplicate)
OUTDATED_FILES=(
  "LOCAL_SETUP_GUIDE.md"
  "IMPLEMENTATION_COMPLETE.md"
  "DOCKER_SETUP.md"
)

# Files to keep (latest & most relevant)
KEEP_FILES=(
  "START_HERE.md"
  "QUICK_START.md"
  "UPDATED_LOCAL_SETUP.md"
  "COMPLETE_SOLUTION_SUMMARY.md"
  "IMPLEMENTATION_IMPROVEMENTS.md"
  "ARCHITECTURE_DIAGRAM.md"
  "PREREQUISITES_CHECKLIST.md"
  "README.md"
)

echo -e "${YELLOW}Files to archive (outdated/duplicate):${NC}"
for file in "${OUTDATED_FILES[@]}"; do
  if [ -f "$file" ]; then
    echo "  • $file"
  fi
done
echo ""

echo -e "${GREEN}Files to keep (latest & relevant):${NC}"
for file in "${KEEP_FILES[@]}"; do
  if [ -f "$file" ]; then
    echo "  ✓ $file"
  fi
done
echo ""

read -p "Proceed with cleanup? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Cleanup cancelled."
  exit 0
fi

# Archive outdated files
echo ""
echo -e "${BLUE}Archiving outdated files...${NC}"
for file in "${OUTDATED_FILES[@]}"; do
  if [ -f "$file" ]; then
    mv "$file" "$ARCHIVE_DIR/"
    echo -e "${GREEN}✓ Archived: $file${NC}"
  else
    echo -e "${YELLOW}⚠ Not found: $file${NC}"
  fi
done

# Create archive README
cat > "$ARCHIVE_DIR/README.md" << 'EOF'
# Archived Documentation

This directory contains documentation files that have been superseded by newer, more comprehensive guides.

## Why These Were Archived

These files were created during development iterations and have been replaced by:

- **START_HERE.md** - Main entry point for all users
- **QUICK_START.md** - Fast setup guide (5 minutes)
- **UPDATED_LOCAL_SETUP.md** - Complete setup with UI-driven operations
- **COMPLETE_SOLUTION_SUMMARY.md** - Answers to common questions
- **IMPLEMENTATION_IMPROVEMENTS.md** - What changed and why

## Archived Files

1. **LOCAL_SETUP_GUIDE.md** - Superseded by UPDATED_LOCAL_SETUP.md
   - Old version with manual steps
   - New version has UI-driven operations

2. **IMPLEMENTATION_COMPLETE.md** - Superseded by COMPLETE_SOLUTION_SUMMARY.md
   - Redundant content
   - New version is more comprehensive

3. **DOCKER_SETUP.md** - Superseded by QUICK_START.md
   - Referenced old JuiceFS approach
   - New version uses direct S3 streaming

## Date Archived

$(date +%Y-%m-%d)

## Restoration

If you need to restore any of these files:

```bash
cp docs/archive/$(date +%Y%m%d)/<filename> ./
```

However, we strongly recommend using the newer documentation instead.
EOF

echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║                  🎉 Cleanup Complete!                          ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
echo -e "${GREEN}Archived:${NC}"
echo "  • $(ls -1 $ARCHIVE_DIR/*.md 2>/dev/null | wc -l | tr -d ' ') file(s) moved to $ARCHIVE_DIR"
echo ""
echo -e "${GREEN}Active Documentation:${NC}"
for file in "${KEEP_FILES[@]}"; do
  if [ -f "$file" ]; then
    SIZE=$(du -h "$file" | cut -f1)
    echo "  ✓ $file ($SIZE)"
  fi
done
echo ""
echo -e "${BLUE}Start with: START_HERE.md${NC}"
echo ""


