#!/bin/bash
# A simple file counter plugin for Inspectre
# Usage: file_counter.sh <repository-path>

# Check if repository path is provided
if [ $# -lt 1 ]; then
  echo "Usage: file_counter.sh <repository-path>"
  exit 1
fi

REPO_PATH="$1"
EXCLUDE_DIRS="${INSPECTRE_CONFIG_exclude_dirs:-.git,node_modules,vendor}"

# Change to repository directory
cd "$REPO_PATH" || exit 1

# Convert comma-separated exclude dirs to find format
FIND_EXCLUDES=""
IFS=',' read -ra EXCLUDE_ARRAY <<< "$EXCLUDE_DIRS"
for i in "${EXCLUDE_ARRAY[@]}"; do
  FIND_EXCLUDES+=" -not -path \"*/$i/*\""
done

# Count total files (excluding specified directories)
TOTAL=$(eval "find . -type f $FIND_EXCLUDES | wc -l")

# Count by extension
echo "["
echo "  {"
echo "    \"name\": \"file_count_total\","
echo "    \"value\": $TOTAL,"
echo "    \"timestamp\": \"$(date -u +"%Y-%m-%dT%H:%M:%SZ")\""
echo "  }"

# Get file counts by extension
eval "find . -type f $FIND_EXCLUDES" | grep -o '\.[^./]*$' | sort | uniq -c | sort -nr | while read -r count ext; do
  if [ -n "$ext" ]; then
    echo "  ,"
    echo "  {"
    echo "    \"name\": \"file_count_by_extension\","
    echo "    \"value\": $count,"
    echo "    \"labels\": {"
    echo "      \"extension\": \"${ext}\""
    echo "    },"
    echo "    \"timestamp\": \"$(date -u +"%Y-%m-%dT%H:%M:%SZ")\""
    echo "  }"
  fi
done

echo "]"