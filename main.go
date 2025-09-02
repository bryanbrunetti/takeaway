package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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

// ExifToolManager manages a persistent ExifTool process
type ExifToolManager struct {
	cmd    *exec.Cmd
	stdin  *os.File
	stdout *os.File
	mu     sync.Mutex
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

	// Initialize ExifTool in persistent mode
	exifTool, err := NewExifToolManager()
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
		go worker(config, exifTool, jobs, results, &wg)
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

func worker(config *Config, exifTool *ExifToolManager, jobs <-chan Job, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		result := processMediaFile(config, exifTool, job.File)
		results <- result
	}
}

func processMediaFile(config *Config, exifTool *ExifToolManager, file MediaFile) Result {
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
	ext := filepath.Ext(baseName)

	// Handle files with (number) suffix
	numberSuffixRegex := regexp.MustCompile(`^(.+)\((\d+)\)$`)
	var numberSuffix string
	var baseForSidecar string

	if matches := numberSuffixRegex.FindStringSubmatch(baseNameNoExt); len(matches) == 3 {
		baseForSidecar = matches[1] + ext     // e.g., "IMG_456.jpg" from "IMG_456(1).jpg"
		numberSuffix = "(" + matches[2] + ")" // e.g., "(1)"
	} else {
		baseForSidecar = baseName
		numberSuffix = ""
	}

	// Possible sidecar patterns
	patterns := []string{
		// Full supplemental-metadata name
		fmt.Sprintf("%s.supplemental-metadata%s.json", baseForSidecar, numberSuffix),
		// Truncated versions
		fmt.Sprintf("%s.supplemental-meta%s.json", baseForSidecar, numberSuffix),
		fmt.Sprintf("%s.su%s.json", baseForSidecar, numberSuffix),
		// Simple .json
		fmt.Sprintf("%s%s.json", baseForSidecar, numberSuffix),
	}

	for _, pattern := range patterns {
		sidecarPath := filepath.Join(file.Dir, pattern)
		if _, err := os.Stat(sidecarPath); err == nil {
			return sidecarPath
		}
	}

	return ""
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

func updateExifDate(config *Config, exifTool *ExifToolManager, filePath string, date time.Time) error {
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

// NewExifToolManager creates a new ExifTool manager with persistent process
func NewExifToolManager() (*ExifToolManager, error) {
	// Check if exiftool is available
	if _, err := exec.LookPath("exiftool"); err != nil {
		return nil, fmt.Errorf("exiftool not found in PATH: %v", err)
	}

	return &ExifToolManager{}, nil
}

// GetMetadata extracts metadata from a file using ExifTool
func (etm *ExifToolManager) GetMetadata(filePath string) (map[string]string, error) {
	etm.mu.Lock()
	defer etm.mu.Unlock()

	cmd := exec.Command("exiftool", "-json", "-dateFormat", "%Y:%m:%d %H:%M:%S", filePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("exiftool command failed: %v", err)
	}

	var exifData []map[string]interface{}
	if err := json.Unmarshal(output, &exifData); err != nil {
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
func (etm *ExifToolManager) UpdateAllDates(filePath, dateStr string) error {
	etm.mu.Lock()
	defer etm.mu.Unlock()

	cmd := exec.Command("exiftool",
		"-overwrite_original",
		fmt.Sprintf("-AllDates=%s", dateStr),
		filePath,
	)

	return cmd.Run()
}

// Close cleans up the ExifTool manager
func (etm *ExifToolManager) Close() error {
	etm.mu.Lock()
	defer etm.mu.Unlock()

	if etm.cmd != nil && etm.cmd.Process != nil {
		return etm.cmd.Process.Kill()
	}
	return nil
}
