package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var version = "development"

// Configuration holds the application configuration
type Config struct {
	SourceDir string
	OutputDir string
	Move      bool
	DryRun    bool
	Workers   int
}

// MediaFile represents a media file to be processed
type MediaFile struct {
	Path     string
	BaseName string
	Dir      string
}

// SidecarData represents the structure of Google Photos JSON sidecar files
type SidecarData struct {
	Title          string `json:"title"`
	PhotoTakenTime struct {
		Timestamp string `json:"timestamp"`
	} `json:"photoTakenTime"`
}

// AlbumMetadata represents the structure of album metadata.json files
type AlbumMetadata struct {
	Title string `json:"title"`
}

// ExifToolProcess represents a single persistent ExifTool process
type ExifToolProcess struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	scanner *bufio.Scanner
	mu      sync.Mutex
}

// ExifToolManager manages multiple ExifTool processes (one per worker)
type ExifToolManager struct {
	processes []*ExifToolProcess
}

// Job represents a work item for the worker pool
type Job struct {
	File MediaFile
}

// Result represents the result of processing a file
type Result struct {
	File    MediaFile
	Success bool
	Action  string
	Error   error
}

var (
	// Supported media file extensions
	supportedExts = map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".tiff": true, ".tif": true,
		".bmp": true, ".gif": true, ".webp": true, ".heic": true, ".heif": true,
		".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".wmv": true,
		".m4v": true, ".3gp": true, ".webm": true, ".flv": true, ".mts": true,
		".m2ts": true, ".ts": true, ".mxf": true,
	}

	// EXIF date tags to check in order of priority
	exifDateTags = []string{
		"DateTimeOriginal",
		"CreationDate",
		"CreateDate",
		"MediaCreateDate",
		"DateTimeCreated",
	}
)

func main() {
	config := parseFlags()

	if err := validateConfig(config); err != nil {
		log.Fatal("Configuration error:", err)
	}

	fmt.Printf("Google Photos Takeout Cleanup Tool v%s\n", version)
	fmt.Printf("===========================================\n\n")
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Source: %s\n", config.SourceDir)
	fmt.Printf("  Output: %s\n", config.OutputDir)
	fmt.Printf("  Move files: %t\n", config.Move)
	fmt.Printf("  Dry run: %t\n", config.DryRun)
	fmt.Printf("  Workers: %d\n\n", config.Workers)

	// Initialize ExifTool manager with one process per worker
	exifTool, err := NewExifToolManager(config.Workers)
	if err != nil {
		log.Fatal("Failed to initialize ExifTool:", err)
	}
	defer exifTool.Close()

	// Scan for media files
	fmt.Println("Scanning for media files...")
	mediaFiles, err := scanMediaFiles(config.SourceDir)
	if err != nil {
		log.Fatal("Failed to scan media files:", err)
	}

	fmt.Printf("Found %d media files\n\n", len(mediaFiles))

	if len(mediaFiles) == 0 {
		fmt.Println("No media files found to process.")
		return
	}

	// Process files using worker pool
	results := processFiles(config, exifTool, mediaFiles)

	// Print summary
	printSummary(results)
}

func parseFlags() *Config {
	var showVersion bool
	config := &Config{}

	flag.StringVar(&config.SourceDir, "source", "", "Path to the Google Photos Takeout root directory")
	flag.StringVar(&config.OutputDir, "output", "", "Path to the output directory for cleaned files")
	flag.BoolVar(&config.Move, "move", false, "Move files to organized directory structure")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Simulate process without making changes")
	flag.IntVar(&config.Workers, "workers", 4, "Number of worker goroutines")
	flag.BoolVar(&showVersion, "version", false, "Show version information")

	flag.Usage = func() {
		fmt.Printf("Google Photos Takeout Cleanup Tool v%s\n\n", version)
		fmt.Printf("Usage: %s [OPTIONS]\n\n", os.Args[0])
		fmt.Printf("Required flags:\n")
		fmt.Printf("  -source string    Path to the Google Photos Takeout root directory\n")
		fmt.Printf("  -output string    Path to the output directory for cleaned files\n\n")
		fmt.Printf("Optional flags:\n")
		fmt.Printf("  -move             Move files to organized directory structure (YYYY/MM/DD)\n")
		fmt.Printf("  -dry-run          Simulate process without making changes\n")
		fmt.Printf("  -workers int      Number of worker goroutines (default 4)\n")
		fmt.Printf("  -version          Show version information\n")
		fmt.Printf("  -help             Show this help message\n\n")
		fmt.Printf("Examples:\n")
		fmt.Printf("  %s -source ./takeout -output ./cleaned\n", os.Args[0])
		fmt.Printf("  %s -source ./takeout -output ./organized -move -dry-run\n", os.Args[0])
		fmt.Printf("  %s -source ./takeout -output ./cleaned -workers 8\n\n", os.Args[0])
	}

	flag.Parse()

	if showVersion {
		fmt.Printf("Google Photos Takeout Cleanup Tool v%s\n", version)
		os.Exit(0)
	}

	return config
}

func validateConfig(config *Config) error {
	if config.SourceDir == "" {
		return errors.New("source directory is required")
	}

	if config.OutputDir == "" {
		return errors.New("output directory is required")
	}

	if config.Workers <= 0 {
		config.Workers = 4
	}

	// Check if source directory exists
	if _, err := os.Stat(config.SourceDir); os.IsNotExist(err) {
		return fmt.Errorf("source directory does not exist: %s", config.SourceDir)
	}

	// Create output directory if it doesn't exist (unless dry run)
	if !config.DryRun {
		if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %v", err)
		}
	}

	return nil
}

func scanMediaFiles(sourceDir string) ([]MediaFile, error) {
	var mediaFiles []MediaFile

	err := filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if supportedExts[ext] {
			baseName := filepath.Base(path)
			dir := filepath.Dir(path)

			mediaFiles = append(mediaFiles, MediaFile{
				Path:     path,
				BaseName: baseName,
				Dir:      dir,
			})
		}

		return nil
	})

	return mediaFiles, err
}

func processFiles(config *Config, exifTool *ExifToolManager, mediaFiles []MediaFile) []Result {
	jobs := make(chan Job, len(mediaFiles))
	results := make(chan Result, len(mediaFiles))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < config.Workers; i++ {
		wg.Add(1)
		go worker(i, config, exifTool, jobs, results, &wg)
	}

	// Send jobs
	go func() {
		defer close(jobs)
		for _, file := range mediaFiles {
			jobs <- Job{File: file}
		}
	}()

	// Close results channel when all workers finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allResults []Result
	processed := 0
	total := len(mediaFiles)

	for result := range results {
		allResults = append(allResults, result)
		processed++

		if processed%10 == 0 || processed == total {
			fmt.Printf("Processed: %d/%d files\r", processed, total)
		}
	}

	fmt.Printf("\nProcessing complete!\n\n")
	return allResults
}

func worker(workerID int, config *Config, exifTool *ExifToolManager, jobs <-chan Job, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()

	// Get dedicated ExifTool process for this worker
	process := exifTool.GetProcessForWorker(workerID)

	for job := range jobs {
		result := processMediaFile(config, process, job.File)
		results <- result
	}
}

func processMediaFile(config *Config, exifTool *ExifToolProcess, file MediaFile) Result {
	result := Result{File: file}

	// Extract existing EXIF metadata
	exifData, err := exifTool.GetMetadata(file.Path)
	if err != nil {
		result.Error = fmt.Errorf("failed to get EXIF data: %v", err)
		return result
	}

	// Try to find a valid date from EXIF
	var creationDate time.Time
	var foundExifDate bool

	for _, tag := range exifDateTags {
		if dateStr, exists := exifData[tag]; exists && dateStr != "" {
			if date, err := parseExifDate(dateStr); err == nil {
				creationDate = date
				foundExifDate = true
				break
			}
		}
	}

	// If no EXIF date found, check for JSON sidecar
	if !foundExifDate {
		if sidecarPath := findSidecarFile(file); sidecarPath != "" {
			if date, err := parseSidecarDate(sidecarPath); err == nil {
				creationDate = date

				// Update EXIF tags with sidecar date
				if err := updateExifDate(config, exifTool, file.Path, creationDate); err != nil {
					result.Error = fmt.Errorf("failed to update EXIF date: %v", err)
					return result
				}
				result.Action = "Updated EXIF from sidecar"
			} else {
				result.Error = fmt.Errorf("failed to parse sidecar date: %v", err)
				return result
			}
		} else {
			result.Error = fmt.Errorf("no creation date found in EXIF or sidecar")
			return result
		}
	}

	// Move file if requested and we have a valid date
	if config.Move && !creationDate.IsZero() {
		destPath := generateDestinationPath(config.OutputDir, file.BaseName, creationDate)

		if config.DryRun {
			result.Action += fmt.Sprintf(" | Would move to: %s", destPath)
		} else {
			if err := moveFile(file.Path, destPath); err != nil {
				result.Error = fmt.Errorf("failed to move file: %v", err)
				return result
			}
			result.Action += fmt.Sprintf(" | Moved to: %s", destPath)
		}

		// Create album symlink if album metadata exists
		if albumName := getAlbumName(file.Dir); albumName != "" {
			symlinkPath := generateAlbumSymlinkPath(config.OutputDir, albumName, file.BaseName)

			if config.DryRun {
				result.Action += fmt.Sprintf(" | Would create album symlink: %s", symlinkPath)
			} else {
				if err := createAlbumSymlink(destPath, symlinkPath); err != nil {
					// Don't fail the entire operation for symlink errors, just log
					result.Action += fmt.Sprintf(" | Symlink error: %v", err)
				} else {
					result.Action += fmt.Sprintf(" | Album symlink created: %s", symlinkPath)
				}
			}
		}
	}

	if result.Action == "" {
		result.Action = "No action needed"
	}

	result.Success = true
	return result
}

func findSidecarFile(file MediaFile) string {
	baseName := file.BaseName
	baseNameNoExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	// Handle files with (number) suffix
	numberSuffixRegex := regexp.MustCompile(`^(.+)\((\d+)\)$`)
	var baseForSidecar string
	var numberSuffix string

	if matches := numberSuffixRegex.FindStringSubmatch(baseNameNoExt); len(matches) == 3 {
		baseForSidecar = matches[1] + filepath.Ext(baseName) // e.g., "IMG_456.jpg" from "IMG_456(1).jpg"
		numberSuffix = `\(` + matches[2] + `\)`              // e.g., "\(1\)" for regex
	} else {
		baseForSidecar = baseName
		numberSuffix = ""
	}

	// Handle trailing underscore removal edge case
	// Google Photos sometimes removes trailing underscores from filenames when creating JSON sidecars
	baseForSidecarNoUnderscore := baseForSidecar
	baseNameNoUnderscoreNoExt := ""

	if strings.HasSuffix(strings.TrimSuffix(baseForSidecar, filepath.Ext(baseForSidecar)), "_") {
		// Remove trailing underscore from the base name (but keep the extension for one pattern)
		ext := filepath.Ext(baseForSidecar)
		nameWithoutExt := strings.TrimSuffix(baseForSidecar, ext)
		baseForSidecarNoUnderscore = strings.TrimSuffix(nameWithoutExt, "_") + ext
		baseNameNoUnderscoreNoExt = strings.TrimSuffix(nameWithoutExt, "_")
	}

	// Create regex patterns for sidecar matching
	escapedBase := regexp.QuoteMeta(baseForSidecar)
	escapedBaseNoUnderscore := regexp.QuoteMeta(baseForSidecarNoUnderscore)
	escapedBaseNoUnderscoreNoExt := regexp.QuoteMeta(baseNameNoUnderscoreNoExt)

	var patterns []string
	if numberSuffix != "" {
		// Pattern with original filename
		patterns = append(patterns, fmt.Sprintf(`^%s(?:\..*)?%s\.json$`, escapedBase, numberSuffix))
		// Pattern without trailing underscore (if different)
		if baseForSidecarNoUnderscore != baseForSidecar {
			patterns = append(patterns, fmt.Sprintf(`^%s(?:\..*)?%s\.json$`, escapedBaseNoUnderscore, numberSuffix))
			// Pattern for base name without extension and underscore (e.g., "name_.jpg" -> "name.json")
			patterns = append(patterns, fmt.Sprintf(`^%s%s\.json$`, escapedBaseNoUnderscoreNoExt, numberSuffix))
		}
	} else {
		// Pattern with original filename
		patterns = append(patterns, fmt.Sprintf(`^%s(?:\..*)?\.json$`, escapedBase))
		// Pattern without trailing underscore (if different)
		if baseForSidecarNoUnderscore != baseForSidecar {
			patterns = append(patterns, fmt.Sprintf(`^%s(?:\..*)?\.json$`, escapedBaseNoUnderscore))
			// Pattern for base name without extension and underscore (e.g., "name_.jpg" -> "name.json")
			patterns = append(patterns, fmt.Sprintf(`^%s\.json$`, escapedBaseNoUnderscoreNoExt))
		}
	}

	// Read directory contents once
	entries, err := os.ReadDir(file.Dir)
	if err != nil {
		return ""
	}

	// Find matching JSON files using all patterns
	for _, pattern := range patterns {
		sidecarRegex, err := regexp.Compile(pattern)
		if err != nil {
			continue // Skip invalid regex patterns
		}

		for _, entry := range entries {
			if !entry.IsDir() && sidecarRegex.MatchString(entry.Name()) {
				sidecarPath := filepath.Join(file.Dir, entry.Name())
				// Verify it's actually a Google Photos sidecar by checking content
				if isGooglePhotosSidecar(sidecarPath) {
					return sidecarPath
				}
			}
		}
	}

	// If no exact patterns match, try progressive prefix matching for arbitrary truncation
	// This handles cases like "BonannoJohn1959VacavilleCalifWithEvaAndDelgadoK.jpg" -> "BonannoJohn1959VacavilleCalifWithEvaAndDelgado.json"
	return findSidecarWithPrefixMatching(file, entries)
}

func findSidecarWithPrefixMatching(file MediaFile, entries []os.DirEntry) string {
	baseName := file.BaseName
	baseNameNoExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	// Handle numbered files
	numberSuffixRegex := regexp.MustCompile(`^(.+)\((\d+)\)$`)
	var baseForMatching string
	var numberSuffix string

	if matches := numberSuffixRegex.FindStringSubmatch(baseNameNoExt); len(matches) == 3 {
		baseForMatching = matches[1]
		numberSuffix = "(" + matches[2] + ")"
	} else {
		baseForMatching = baseNameNoExt
		numberSuffix = ""
	}

	// Try progressively shorter prefixes (minimum 10 characters to avoid false positives)
	minPrefixLength := 10
	if len(baseForMatching) < minPrefixLength {
		minPrefixLength = len(baseForMatching)
	}

	for prefixLen := len(baseForMatching); prefixLen >= minPrefixLength; prefixLen-- {
		prefix := baseForMatching[:prefixLen]
		escapedPrefix := regexp.QuoteMeta(prefix)

		var candidatePattern string
		if numberSuffix != "" {
			escapedNumberSuffix := regexp.QuoteMeta(numberSuffix)
			candidatePattern = fmt.Sprintf(`^%s.*%s\.json$`, escapedPrefix, escapedNumberSuffix)
		} else {
			candidatePattern = fmt.Sprintf(`^%s.*\.json$`, escapedPrefix)
		}

		regex, err := regexp.Compile(candidatePattern)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() && regex.MatchString(entry.Name()) {
				sidecarPath := filepath.Join(file.Dir, entry.Name())
				// Additional validation: check content and ensure it's not too different in length
				if isGooglePhotosSidecar(sidecarPath) {
					sidecarName := strings.TrimSuffix(entry.Name(), ".json")
					// Remove number suffix from sidecar name for length comparison
					if numberSuffix != "" {
						sidecarName = strings.TrimSuffix(sidecarName, numberSuffix)
					}
					// Allow up to 30% length difference to handle arbitrary truncation
					maxLenDiff := len(baseForMatching) / 3
					if len(baseForMatching)-len(sidecarName) <= maxLenDiff && len(baseForMatching)-len(sidecarName) >= 0 {
						return sidecarPath
					}
				}
			}
		}
	}

	return ""
}

func isGooglePhotosSidecar(filePath string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	// Quick check for Google Photos sidecar structure
	content := string(data)
	return strings.Contains(content, "photoTakenTime") &&
		strings.Contains(content, "timestamp") &&
		(strings.Contains(content, "title") || len(content) < 1000) // Basic validation
}

func parseSidecarDate(sidecarPath string) (time.Time, error) {
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		return time.Time{}, err
	}

	var sidecar SidecarData
	if err := json.Unmarshal(data, &sidecar); err != nil {
		return time.Time{}, err
	}

	timestamp, err := strconv.ParseInt(sidecar.PhotoTakenTime.Timestamp, 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(timestamp, 0), nil
}

func parseExifDate(dateStr string) (time.Time, error) {
	// Common EXIF date formats
	formats := []string{
		"2006:01:02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return date, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

func updateExifDate(config *Config, exifTool *ExifToolProcess, filePath string, date time.Time) error {
	if config.DryRun {
		return nil
	}

	dateStr := date.Format("2006:01:02 15:04:05")
	return exifTool.UpdateAllDates(filePath, dateStr)
}

func generateDestinationPath(outputDir, fileName string, date time.Time) string {
	year := fmt.Sprintf("%04d", date.Year())
	month := fmt.Sprintf("%02d", date.Month())
	day := fmt.Sprintf("%02d", date.Day())

	return filepath.Join(outputDir, "ALL_PHOTOS", year, month, day, fileName)
}

func generateAlbumSymlinkPath(outputDir, albumName, fileName string) string {
	return filepath.Join(outputDir, "ALBUMS", albumName, fileName)
}

func getAlbumName(dir string) string {
	metadataPath := filepath.Join(dir, "metadata.json")

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return ""
	}

	var metadata AlbumMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return ""
	}

	return metadata.Title
}

func createAlbumSymlink(targetPath, symlinkPath string) error {
	// Create the album directory if it doesn't exist
	albumDir := filepath.Dir(symlinkPath)
	if err := os.MkdirAll(albumDir, 0755); err != nil {
		return fmt.Errorf("failed to create album directory: %v", err)
	}

	// Calculate relative path from symlink to target
	relPath, err := filepath.Rel(albumDir, targetPath)
	if err != nil {
		return fmt.Errorf("failed to calculate relative path: %v", err)
	}

	// Remove existing symlink if it exists
	if _, err := os.Lstat(symlinkPath); err == nil {
		if err := os.Remove(symlinkPath); err != nil {
			return fmt.Errorf("failed to remove existing symlink: %v", err)
		}
	}

	// Create the symlink
	if err := os.Symlink(relPath, symlinkPath); err != nil {
		return fmt.Errorf("failed to create symlink: %v", err)
	}

	return nil
}

func moveFile(srcPath, destPath string) error {
	// Create destination directory
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Move file
	return os.Rename(srcPath, destPath)
}

func printSummary(results []Result) {
	successful := 0
	failed := 0
	actions := make(map[string]int)

	for _, result := range results {
		if result.Success {
			successful++
			actions[result.Action]++
		} else {
			failed++
			fmt.Printf("ERROR: %s - %v\n", result.File.Path, result.Error)
		}
	}

	fmt.Printf("=== SUMMARY ===\n")
	fmt.Printf("Total files processed: %d\n", len(results))
	fmt.Printf("Successful: %d\n", successful)
	fmt.Printf("Failed: %d\n\n", failed)

	if len(actions) > 0 {
		fmt.Printf("Actions taken:\n")
		for action, count := range actions {
			fmt.Printf("  %s: %d\n", action, count)
		}
	}
}

// NewExifToolManager creates a new ExifTool manager with one process per worker
func NewExifToolManager(workerCount int) (*ExifToolManager, error) {
	// Check if exiftool is available
	if _, err := exec.LookPath("exiftool"); err != nil {
		return nil, fmt.Errorf("exiftool not found in PATH: %v", err)
	}

	processes := make([]*ExifToolProcess, workerCount)
	for i := 0; i < workerCount; i++ {
		process, err := startExifToolProcess()
		if err != nil {
			// Clean up any processes that were already started
			for j := 0; j < i; j++ {
				processes[j].Close()
			}
			return nil, fmt.Errorf("failed to start ExifTool process %d: %v", i, err)
		}
		processes[i] = process
	}

	return &ExifToolManager{
		processes: processes,
	}, nil
}

// startExifToolProcess starts a single ExifTool process in persistent mode
func startExifToolProcess() (*ExifToolProcess, error) {
	// Start ExifTool in persistent mode with -stay_open
	cmd := exec.Command("exiftool", "-stay_open", "True", "-@", "-")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to start exiftool: %v", err)
	}

	scanner := bufio.NewScanner(stdout)

	return &ExifToolProcess{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		scanner: scanner,
	}, nil
}

// GetProcessForWorker returns the ExifTool process assigned to a specific worker
func (etm *ExifToolManager) GetProcessForWorker(workerID int) *ExifToolProcess {
	return etm.processes[workerID]
}

// GetMetadata extracts metadata from a file using ExifTool
func (etp *ExifToolProcess) GetMetadata(filePath string) (map[string]string, error) {
	etp.mu.Lock()
	defer etp.mu.Unlock()

	// Send command to persistent ExifTool process
	command := fmt.Sprintf("-json\n-dateFormat\n%%Y:%%m:%%d %%H:%%M:%%S\n%s\n-execute\n", filePath)

	if _, err := etp.stdin.Write([]byte(command)); err != nil {
		return nil, fmt.Errorf("failed to write to exiftool stdin: %v", err)
	}

	// Read response until we see {ready} marker
	var output strings.Builder
	for etp.scanner.Scan() {
		line := etp.scanner.Text()
		if line == "{ready}" {
			break
		}
		output.WriteString(line)
		output.WriteString("\n")
	}

	if err := etp.scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read exiftool output: %v", err)
	}

	outputStr := strings.TrimSpace(output.String())
	if outputStr == "" {
		return make(map[string]string), nil
	}

	var exifData []map[string]interface{}
	if err := json.Unmarshal([]byte(outputStr), &exifData); err != nil {
		return nil, fmt.Errorf("failed to parse exiftool JSON: %v", err)
	}

	if len(exifData) == 0 {
		return make(map[string]string), nil
	}

	result := make(map[string]string)
	for key, value := range exifData[0] {
		if str, ok := value.(string); ok {
			result[key] = str
		}
	}

	return result, nil
}

// UpdateAllDates updates all date fields in a file
func (etp *ExifToolProcess) UpdateAllDates(filePath, dateStr string) error {
	etp.mu.Lock()
	defer etp.mu.Unlock()

	// Send update command to persistent ExifTool process
	command := fmt.Sprintf("-overwrite_original\n-AllDates=%s\n%s\n-execute\n", dateStr, filePath)

	if _, err := etp.stdin.Write([]byte(command)); err != nil {
		return fmt.Errorf("failed to write to exiftool stdin: %v", err)
	}

	// Read response until we see {ready} marker
	for etp.scanner.Scan() {
		line := etp.scanner.Text()
		if line == "{ready}" {
			break
		}
		// Check for error messages
		if strings.Contains(line, "Error:") || strings.Contains(line, "Warning:") {
			return fmt.Errorf("exiftool error: %s", line)
		}
	}

	if err := etp.scanner.Err(); err != nil {
		return fmt.Errorf("failed to read exiftool output: %v", err)
	}

	return nil
}

// Close cleans up the ExifTool manager by closing all processes
func (etm *ExifToolManager) Close() error {
	var lastErr error
	for i, process := range etm.processes {
		if err := process.Close(); err != nil {
			log.Printf("Error closing ExifTool process %d: %v", i, err)
			lastErr = err
		}
	}
	return lastErr
}

// Close cleans up a single ExifTool process
func (etp *ExifToolProcess) Close() error {
	etp.mu.Lock()
	defer etp.mu.Unlock()

	// Send -stay_open False to gracefully terminate ExifTool
	if etp.stdin != nil {
		etp.stdin.Write([]byte("-stay_open\nFalse\n"))
		etp.stdin.Close()
	}

	if etp.stdout != nil {
		etp.stdout.Close()
	}

	if etp.cmd != nil && etp.cmd.Process != nil {
		// Wait for the process to exit gracefully, or kill it after a timeout
		done := make(chan error, 1)
		go func() {
			done <- etp.cmd.Wait()
		}()

		select {
		case err := <-done:
			return err
		case <-time.After(5 * time.Second):
			// Force kill if it doesn't exit gracefully
			return etp.cmd.Process.Kill()
		}
	}

	return nil
}
