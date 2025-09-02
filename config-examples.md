# Configuration Examples for Google Photos Takeout Cleanup Tool

This document provides various configuration examples for different use cases and scenarios.

## Basic Usage Patterns

### 1. Simple EXIF Cleanup (No File Movement)
```bash
./takeaway-cleanup -source /path/to/takeout -output /path/to/cleaned
```
- Updates missing EXIF dates from JSON sidecars
- Files remain in original directory structure
- Good for preserving existing organization

### 2. Full Organization by Date
```bash
./takeaway-cleanup -source /path/to/takeout -output /path/to/organized -move
```
- Updates EXIF dates AND moves files
- Creates YYYY/MM/DD directory structure
- Best for complete reorganization

### 3. Dry Run (Preview Changes)
```bash
./takeaway-cleanup -source /path/to/takeout -output /path/to/organized -move -dry-run
```
- Shows what would be done without making changes
- Essential for testing before actual processing
- Safe way to verify expected behavior

## Performance Optimization

### Small Datasets (< 1,000 files)
```bash
./takeaway-cleanup -source /path/to/takeout -output /path/to/organized -move -workers 2
```
- Fewer workers to avoid overhead
- Sufficient for small collections

### Medium Datasets (1,000 - 10,000 files)
```bash
./takeaway-cleanup -source /path/to/takeout -output /path/to/organized -move -workers 4
```
- Default worker count
- Balanced performance for most use cases

### Large Datasets (> 10,000 files)
```bash
./takeaway-cleanup -source /path/to/takeout -output /path/to/organized -move -workers 8
```
- Higher worker count for large datasets
- Ensure your system can handle the load
- Monitor CPU and memory usage

### Very Large Datasets (> 50,000 files)
```bash
./takeaway-cleanup -source /path/to/takeout -output /path/to/organized -move -workers 12
```
- Maximum recommended workers
- Consider processing in batches if memory becomes an issue
- May benefit from SSD storage for better I/O performance

## Specific Scenarios

### Scenario 1: Preserving Album Structure
If you want to keep your album organization but fix dates:
```bash
# Process each album separately
./takeaway-cleanup -source "/path/to/takeout/Album 1" -output "/path/to/cleaned/Album 1"
./takeaway-cleanup -source "/path/to/takeout/Album 2" -output "/path/to/cleaned/Album 2"
```

### Scenario 2: Testing with Subset
For testing on a small portion of your data:
```bash
# Create a test directory with a few files first
mkdir test_subset
cp -r "/path/to/takeout/some_album" test_subset/
./takeaway-cleanup -source test_subset -output test_output -move -dry-run
```

### Scenario 3: Handling Mixed Content
For takeouts with both photos and videos:
```bash
./takeaway-cleanup -source /path/to/takeout -output /path/to/organized -move -workers 6
```
- Standard processing works for both photos and videos
- Videos may take slightly longer to process

### Scenario 4: Network Storage
When working with network-attached storage or cloud drives:
```bash
./takeaway-cleanup -source /network/takeout -output /local/organized -move -workers 2
```
- Use fewer workers to reduce network load
- Consider copying to local storage first for better performance

## Batch Processing Examples

### Processing Multiple Takeout Archives
```bash
#!/bin/bash
# Script to process multiple takeout archives

for takeout_dir in /path/to/takeouts/*/; do
    dirname=$(basename "$takeout_dir")
    echo "Processing $dirname..."
    ./takeaway-cleanup -source "$takeout_dir" -output "/path/to/organized/$dirname" -move
done
```

### Processing by Year
```bash
#!/bin/bash
# Process different years separately

./takeaway-cleanup -source "/path/to/takeout/Google Photos/2020" -output "/path/to/organized/2020" -move
./takeaway-cleanup -source "/path/to/takeout/Google Photos/2021" -output "/path/to/organized/2021" -move
./takeaway-cleanup -source "/path/to/takeout/Google Photos/2022" -output "/path/to/organized/2022" -move
```

## Error Recovery and Troubleshooting

### Resume Interrupted Processing
If processing was interrupted:
```bash
# First, check what was already processed
find /path/to/output -name "*.jpg" | wc -l

# Then continue with remaining files
./takeaway-cleanup -source /path/to/remaining -output /path/to/output -move
```

### Handling Permission Issues
```bash
# Ensure proper permissions on source and destination
chmod -R 755 /path/to/takeout
mkdir -p /path/to/output
chmod 755 /path/to/output

./takeaway-cleanup -source /path/to/takeout -output /path/to/output -move
```

### Memory-Constrained Systems
For systems with limited memory:
```bash
./takeaway-cleanup -source /path/to/takeout -output /path/to/output -move -workers 1
```
- Single worker to minimize memory usage
- Slower but more stable on constrained systems

## Quality Assurance Commands

### Verify Processing Results
```bash
# Count original files
find /path/to/takeout -name "*.jpg" -o -name "*.mp4" | wc -l

# Count processed files in ALL_PHOTOS
find /path/to/output/ALL_PHOTOS -name "*.jpg" -o -name "*.mp4" | wc -l

# Count symlinks in ALBUMS
find /path/to/output/ALBUMS -type l | wc -l

# Check for files with missing dates (should be minimal)
exiftool -r -if 'not $DateTimeOriginal' /path/to/output/ALL_PHOTOS
```

### Validate Directory Structure
```bash
# Check main directories are created correctly
ls -la /path/to/output/
ls -la /path/to/output/ALL_PHOTOS/
ls -la /path/to/output/ALBUMS/

# Verify files are in correct date directories
find /path/to/output/ALL_PHOTOS -name "*.jpg" | head -10 | while read file; do
    echo "File: $file"
    exiftool -DateTimeOriginal "$file"
    echo "---"
done

# Verify album symlinks work correctly
find /path/to/output/ALBUMS -type l | head -5 | while read link; do
    echo "Symlink: $link"
    echo "Target: $(readlink "$link")"
    echo "Valid: $(test -e "$link" && echo "Yes" || echo "No")"
    echo "---"
done
```

## Platform-Specific Notes

### Windows PowerShell
```powershell
# Use PowerShell syntax for Windows
.\takeaway-cleanup.exe -source "C:\Users\YourName\Downloads\Takeout" -output "C:\Photos\Organized" -move -dry-run
```

### macOS with Homebrew ExifTool
```bash
# Ensure exiftool is in PATH
export PATH="/usr/local/bin:$PATH"
./takeaway-cleanup -source ~/Downloads/Takeout -output ~/Pictures/Organized -move
```

### Linux with Snap ExifTool
```bash
# If exiftool installed via snap
export PATH="/snap/bin:$PATH"
./takeaway-cleanup -source ~/Downloads/Takeout -output ~/Pictures/Organized -move
```

## Best Practices Summary

1. **Always test first**: Use `-dry-run` before making changes
2. **Backup originals**: Keep a copy of your original Takeout data
3. **Start small**: Test with a subset of files first
4. **Monitor resources**: Watch CPU, memory, and disk I/O during processing
5. **Verify results**: Check file counts and dates after processing
6. **Use appropriate workers**: Match worker count to your system capabilities
7. **Handle errors**: Check the summary output for any failed files
8. **Validate organization**: Spot-check that files are in correct date directories

## Troubleshooting Common Issues

### ExifTool Not Found
```bash
# Check if exiftool is installed
which exiftool
exiftool -ver

# Install if missing (macOS)
brew install exiftool
```

### Permission Denied Errors
```bash
# Fix permissions
chmod -R 755 /path/to/source
sudo chown -R $USER:$USER /path/to/destination
```

### Out of Memory Errors
```bash
# Reduce worker count
./takeaway-cleanup -source /path/to/takeout -output /path/to/output -workers 1
```

### Disk Space Issues
```bash
# Check available space
df -h /path/to/output

# Process in smaller batches if needed
```
