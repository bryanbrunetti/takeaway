#!/bin/bash
# Example usage scenarios for Google Photos Takeout Cleanup Tool

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Google Photos Takeout Cleanup Tool - Example Usage${NC}"
echo "============================================================"

# Check if exiftool is available
if ! command -v exiftool &> /dev/null; then
    echo -e "${RED}ERROR: exiftool is not installed or not in PATH${NC}"
    echo "Please install exiftool first:"
    echo "  macOS: brew install exiftool"
    echo "  Ubuntu/Debian: sudo apt-get install exiftool"
    echo "  Windows: choco install exiftool"
    exit 1
fi

# Build the application if it doesn't exist
BINARY="./takeaway-cleanup"
if [ ! -f "$BINARY" ]; then
    echo -e "${YELLOW}Building application...${NC}"
    go build -o takeaway-cleanup
    echo -e "${GREEN}Build complete!${NC}"
fi

echo ""
echo -e "${GREEN}ExifTool found: $(exiftool -ver)${NC}"
echo ""

# Example 1: Basic dry run with test data
echo -e "${BLUE}Example 1: Basic dry run with test data${NC}"
echo "Command: $BINARY -source ./test/src -output ./test/example1 -dry-run"
echo "----------------------------------------"
$BINARY -source ./test/src -output ./test/example1 -dry-run
echo ""

# Example 2: Organize files by date (dry run)
echo -e "${BLUE}Example 2: Organize files by date (dry run)${NC}"
echo "Command: $BINARY -source ./test/src -output ./test/example2 -move -dry-run"
echo "----------------------------------------"
$BINARY -source ./test/src -output ./test/example2 -move -dry-run
echo ""

# Example 3: High-performance processing with more workers
echo -e "${BLUE}Example 3: High-performance processing with more workers (dry run)${NC}"
echo "Command: $BINARY -source ./test/src -output ./test/example3 -move -workers 8 -dry-run"
echo "----------------------------------------"
$BINARY -source ./test/src -output ./test/example3 -move -workers 8 -dry-run
echo ""

# Example 4: Actual file processing and organization
echo -e "${BLUE}Example 4: Actual file processing and organization${NC}"
echo "Command: $BINARY -source ./test/src -output ./test/example4 -move"
echo "This will make real changes to files!"
echo "----------------------------------------"

# Clean up any previous example4 output
rm -rf ./test/example4

# Copy test files to a temporary location so we don't modify originals
cp -r ./test/src ./test/temp_src
$BINARY -source ./test/temp_src -output ./test/example4 -move

echo ""
echo -e "${GREEN}Files organized in ./test/example4:${NC}"
find ./test/example4 -type f | sort

echo ""
echo -e "${GREEN}Directory structure:${NC}"
tree ./test/example4 2>/dev/null || find ./test/example4 -type d | sort

# Clean up temporary directory
rm -rf ./test/temp_src

echo ""
echo -e "${BLUE}Example 5: Processing with different file types${NC}"
echo "Creating sample files with different extensions..."

# Create a temporary directory with various file types
mkdir -p ./test/mixed_files
cp ./test/src/My\ Album/* ./test/mixed_files/

# Create some additional test files with different extensions
touch "./test/mixed_files/sample.png"
touch "./test/mixed_files/video.mp4"
touch "./test/mixed_files/document.pdf"  # This should be ignored
touch "./test/mixed_files/archive.zip"   # This should be ignored

echo "Command: $BINARY -source ./test/mixed_files -output ./test/example5 -dry-run"
echo "----------------------------------------"
$BINARY -source ./test/mixed_files -output ./test/example5 -dry-run

# Clean up
rm -rf ./test/mixed_files
echo ""

# Example 6: Error handling demonstration
echo -e "${BLUE}Example 6: Error handling demonstration${NC}"
echo "Testing with non-existent source directory..."
echo "Command: $BINARY -source ./nonexistent -output ./test/example6 -dry-run"
echo "----------------------------------------"
if ! $BINARY -source ./nonexistent -output ./test/example6 -dry-run 2>/dev/null; then
    echo -e "${GREEN}✓ Correctly handled non-existent source directory${NC}"
fi
echo ""

# Example 7: Version and help information
echo -e "${BLUE}Example 7: Version and help information${NC}"
echo "Version command:"
$BINARY -version
echo ""

echo "Help command:"
$BINARY -help
echo ""

# Performance benchmark example
echo -e "${BLUE}Example 8: Performance demonstration${NC}"
echo "Creating larger test dataset..."

# Create a larger test dataset
mkdir -p ./test/large_dataset/Album1 ./test/large_dataset/Album2
for i in {1..20}; do
    cp "./test/src/My Album/test.jpg" "./test/large_dataset/Album1/img_${i}.jpg"
    cp "./test/src/My Album/test.jpg.json" "./test/large_dataset/Album1/img_${i}.jpg.json"

    cp "./test/src/My Album/blank.jpg" "./test/large_dataset/Album2/photo_${i}.jpg"
    cp "./test/src/My Album/blank.jpg.su.json" "./test/large_dataset/Album2/photo_${i}.jpg.json"
done

echo "Processing 40 files with timing..."
echo "Command: time $BINARY -source ./test/large_dataset -output ./test/example8 -move -dry-run"
echo "----------------------------------------"
time $BINARY -source ./test/large_dataset -output ./test/example8 -move -dry-run

# Clean up
rm -rf ./test/large_dataset
echo ""

# Summary
echo -e "${GREEN}============================================================${NC}"
echo -e "${GREEN}Examples completed successfully!${NC}"
echo ""
echo -e "${YELLOW}Key takeaways:${NC}"
echo "• Use -dry-run to preview changes before making them"
echo "• Use -move to organize files by date (YYYY/MM/DD structure)"
echo "• Adjust -workers for optimal performance with your dataset"
echo "• The tool handles various JSON sidecar naming conventions"
echo "• Only supported media file types are processed"
echo "• Comprehensive error handling for various scenarios"
echo ""
echo -e "${BLUE}For production use:${NC}"
echo "1. Always test with -dry-run first"
echo "2. Backup your original Takeout data"
echo "3. Use appropriate worker count based on your system"
echo "4. Ensure exiftool is properly installed and updated"
echo ""
echo -e "${GREEN}Happy organizing! 📸${NC}"
