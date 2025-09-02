#!/bin/bash
# Demonstration of the Google Photos Takeout Cleanup Tool directory structure
# This script shows the expected output structure with ALL_PHOTOS and ALBUMS

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${BLUE}Google Photos Takeout Cleanup Tool - Directory Structure Demo${NC}"
echo "=============================================================="

# Clean up any previous demo
rm -rf demo_input demo_output

# Create demo input structure mimicking Google Photos Takeout
echo -e "${YELLOW}Creating demo Google Photos Takeout structure...${NC}"
mkdir -p "demo_input/Vacation 2023"
mkdir -p "demo_input/Family Photos"
mkdir -p "demo_input/Wedding Album"

# Create album metadata files
echo '{"title": "Vacation 2023"}' > "demo_input/Vacation 2023/metadata.json"
echo '{"title": "Family Photos"}' > "demo_input/Family Photos/metadata.json"
echo '{"title": "Wedding Album"}' > "demo_input/Wedding Album/metadata.json"

# Create sample photos with JSON sidecars (different dates for demonstration)
echo '{"title": "beach_sunset.jpg", "photoTakenTime": {"timestamp": "1672531200"}}' > "demo_input/Vacation 2023/beach_sunset.jpg.json"
echo '{"title": "mountain_hike.jpg", "photoTakenTime": {"timestamp": "1675209600"}}' > "demo_input/Vacation 2023/mountain_hike.jpg.json"
echo '{"title": "family_dinner.jpg", "photoTakenTime": {"timestamp": "1677628800"}}' > "demo_input/Family Photos/family_dinner.jpg.json"
echo '{"title": "kids_playing.jpg", "photoTakenTime": {"timestamp": "1680307200"}}' > "demo_input/Family Photos/kids_playing.jpg.json"
echo '{"title": "ceremony.jpg", "photoTakenTime": {"timestamp": "1683936000"}}' > "demo_input/Wedding Album/ceremony.jpg.json"
echo '{"title": "reception_dance.jpg", "photoTakenTime": {"timestamp": "1683939600"}}' > "demo_input/Wedding Album/reception_dance.jpg.json"

# Create placeholder image files
echo "Beautiful beach sunset photo" > "demo_input/Vacation 2023/beach_sunset.jpg"
echo "Mountain hiking adventure" > "demo_input/Vacation 2023/mountain_hike.jpg"
echo "Family dinner celebration" > "demo_input/Family Photos/family_dinner.jpg"
echo "Kids playing in the park" > "demo_input/Family Photos/kids_playing.jpg"
echo "Wedding ceremony moment" > "demo_input/Wedding Album/ceremony.jpg"
echo "Reception dance celebration" > "demo_input/Wedding Album/reception_dance.jpg"

echo -e "${GREEN}Demo input structure created:${NC}"
find demo_input -type f | sort

echo ""
echo -e "${YELLOW}Expected output structure after processing:${NC}"
echo ""
echo -e "${CYAN}demo_output/${NC}"
echo -e "${CYAN}├── ALL_PHOTOS/                    ${GREEN}# All photos organized by date${NC}"
echo -e "${CYAN}│   ├── 2022/                      ${GREEN}# beach_sunset.jpg (Jan 1, 2023 UTC)${NC}"
echo -e "${CYAN}│   │   └── 12/31/                 ${GREEN}# (Dec 31, 2022 local time)${NC}"
echo -e "${CYAN}│   │       └── beach_sunset.jpg${NC}"
echo -e "${CYAN}│   ├── 2023/                      ${GREEN}# Other photos from 2023${NC}"
echo -e "${CYAN}│   │   ├── 01/31/                 ${GREEN}# mountain_hike.jpg${NC}"
echo -e "${CYAN}│   │   │   └── mountain_hike.jpg${NC}"
echo -e "${CYAN}│   │   ├── 02/28/                 ${GREEN}# family_dinner.jpg${NC}"
echo -e "${CYAN}│   │   │   └── family_dinner.jpg${NC}"
echo -e "${CYAN}│   │   ├── 04/01/                 ${GREEN}# kids_playing.jpg${NC}"
echo -e "${CYAN}│   │   │   └── kids_playing.jpg${NC}"
echo -e "${CYAN}│   │   └── 05/13/                 ${GREEN}# Wedding photos${NC}"
echo -e "${CYAN}│   │       ├── ceremony.jpg${NC}"
echo -e "${CYAN}│   │       └── reception_dance.jpg${NC}"
echo -e "${CYAN}└── ALBUMS/                        ${GREEN}# Albums with symlinks to ALL_PHOTOS${NC}"
echo -e "${CYAN}    ├── Vacation 2023/             ${GREEN}# Album from metadata.json${NC}"
echo -e "${CYAN}    │   ├── beach_sunset.jpg -> ../../ALL_PHOTOS/2022/12/31/beach_sunset.jpg${NC}"
echo -e "${CYAN}    │   └── mountain_hike.jpg -> ../../ALL_PHOTOS/2023/01/31/mountain_hike.jpg${NC}"
echo -e "${CYAN}    ├── Family Photos/             ${GREEN}# Another album${NC}"
echo -e "${CYAN}    │   ├── family_dinner.jpg -> ../../ALL_PHOTOS/2023/02/28/family_dinner.jpg${NC}"
echo -e "${CYAN}    │   └── kids_playing.jpg -> ../../ALL_PHOTOS/2023/04/01/kids_playing.jpg${NC}"
echo -e "${CYAN}    └── Wedding Album/             ${GREEN}# Wedding album${NC}"
echo -e "${CYAN}        ├── ceremony.jpg -> ../../ALL_PHOTOS/2023/05/13/ceremony.jpg${NC}"
echo -e "${CYAN}        └── reception_dance.jpg -> ../../ALL_PHOTOS/2023/05/13/reception_dance.jpg${NC}"

echo ""
echo -e "${YELLOW}Running the cleanup tool with dry-run to show what it would do:${NC}"
echo ""

# Build the tool if it doesn't exist
if [ ! -f "bin/takeaway-cleanup" ]; then
    echo -e "${YELLOW}Building the cleanup tool...${NC}"
    make build
fi

# Run with dry-run to show the planned operations
./bin/takeaway-cleanup -source ./demo_input -output ./demo_output -move -dry-run

echo ""
echo -e "${YELLOW}Note about timestamps:${NC}"
echo -e "${GREEN}• 1672531200 = January 1, 2023 00:00:00 UTC${NC}"
echo -e "${GREEN}• 1675209600 = February 1, 2023 00:00:00 UTC${NC}"
echo -e "${GREEN}• 1677628800 = March 1, 2023 00:00:00 UTC${NC}"
echo -e "${GREEN}• 1680307200 = April 1, 2023 00:00:00 UTC${NC}"
echo -e "${GREEN}• 1683936000 = May 13, 2023 00:00:00 UTC${NC}"
echo -e "${GREEN}• 1683939600 = May 13, 2023 01:00:00 UTC${NC}"

echo ""
echo -e "${BLUE}Key Benefits of this Structure:${NC}"
echo -e "${GREEN}✓ Chronological access:${NC} Browse photos by date in ALL_PHOTOS"
echo -e "${GREEN}✓ Album organization:${NC} Access photos by album in ALBUMS"
echo -e "${GREEN}✓ No duplication:${NC} Symlinks save disk space"
echo -e "${GREEN}✓ Preserved context:${NC} Original album organization maintained"
echo -e "${GREEN}✓ Flexible viewing:${NC} Choose chronological or album-based browsing"

echo ""
echo -e "${YELLOW}To actually process the files (remove -dry-run):${NC}"
echo "./bin/takeaway-cleanup -source ./demo_input -output ./demo_output -move"

echo ""
echo -e "${YELLOW}For large datasets with high performance:${NC}"
echo "./bin/takeaway-cleanup -source /path/to/takeout -output /path/to/organized -move -workers 8"

# Clean up demo files
echo ""
echo -e "${YELLOW}Cleaning up demo files...${NC}"
rm -rf demo_input demo_output

echo -e "${GREEN}Directory structure demonstration complete!${NC}"
