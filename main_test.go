package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseSidecarDate(t *testing.T) {
	// Create a temporary sidecar file
	tmpDir := t.TempDir()
	sidecarPath := filepath.Join(tmpDir, "test.json")

	sidecarData := SidecarData{
		Title: "test.jpg",
		PhotoTakenTime: struct {
			Timestamp string `json:"timestamp"`
		}{
			Timestamp: "1672531200", // 2023-01-01 00:00:00 UTC
		},
	}

	data, err := json.Marshal(sidecarData)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(sidecarPath, data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test parsing
	date, err := parseSidecarDate(sidecarPath)
	if err != nil {
		t.Fatalf("Failed to parse sidecar date: %v", err)
	}

	expected := time.Unix(1672531200, 0)
	if !date.Equal(expected) {
		t.Errorf("Expected date %v, got %v", expected, date)
	}
}

func TestParseExifDate(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{
			input:    "2023:01:01 12:30:45",
			expected: "2023-01-01T12:30:45Z",
			wantErr:  false,
		},
		{
			input:    "2023-01-01T12:30:45Z",
			expected: "2023-01-01T12:30:45Z",
			wantErr:  false,
		},
		{
			input:    "2023-01-01 12:30:45",
			expected: "2023-01-01T12:30:45Z",
			wantErr:  false,
		},
		{
			input:   "invalid date",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			date, err := parseExifDate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if date.UTC().Format(time.RFC3339) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, date.UTC().Format(time.RFC3339))
			}
		})
	}
}

func TestFindSidecarFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files including heavily truncated patterns and trailing underscore edge case
	testFiles := []string{
		"IMG_123.jpg",
		"IMG_123.jpg.json",
		"IMG_456(1).jpg",
		"IMG_456.jpg.supplemental-metadata(1).json",
		"blank.jpg",
		"blank.jpg.su.json",
		"Photo on 11-1-15 at 6.24 PM #3.jpg",
		"Photo on 11-1-15 at 6.24 PM #3.jpg.supplementa.json",
		"Bonanno1979BryanAndGrandpa45yrsOld_1.jpg",
		"Bonanno1979BryanAndGrandpa45yrsOld_1.jpg.suppl.json",
		"VeryLongFileNameThatGetsHeavilyTruncated.jpg",
		"VeryLongFileNameThatGetsHeavilyTruncated.jpg.s.json",
		"47931_1530376731723_1003886514_1589589_6447425_.jpg",
		"47931_1530376731723_1003886514_1589589_6447425.json",
		"TrailingUnderscore_.jpg",
		"TrailingUnderscore.jpg.supplemental-metadata.json",
		"BonannoJohn1959VacavilleCalifWithEvaAndDelgadoK(1).jpg",
		"BonannoJohn1959VacavilleCalifWithEvaAndDelgado(1).json",
		"BonannoJohn1959VacavilleCalifWithEvaAndDelgadoK.jpg",
		"BonannoJohn1959VacavilleCalifWithEvaAndDelgado.json",
		"25159_1382026223053_1003886514_1164687_6240_n.jpg",
		"25159_1382026223053_1003886514_1164687_6240_n..json",
	}

	// Create Google Photos-style sidecar content
	sidecarContent := `{"title": "test.jpg", "photoTakenTime": {"timestamp": "1672531200"}}`

	for _, filename := range testFiles {
		path := filepath.Join(tmpDir, filename)
		content := "test"
		if strings.HasSuffix(filename, ".json") {
			content = sidecarContent
		}
		err := os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		mediaFile    MediaFile
		expectedName string
	}{
		{
			mediaFile: MediaFile{
				Path:     filepath.Join(tmpDir, "IMG_123.jpg"),
				BaseName: "IMG_123.jpg",
				Dir:      tmpDir,
			},
			expectedName: "IMG_123.jpg.json",
		},
		{
			mediaFile: MediaFile{
				Path:     filepath.Join(tmpDir, "IMG_456(1).jpg"),
				BaseName: "IMG_456(1).jpg",
				Dir:      tmpDir,
			},
			expectedName: "IMG_456.jpg.supplemental-metadata(1).json",
		},
		{
			mediaFile: MediaFile{
				Path:     filepath.Join(tmpDir, "blank.jpg"),
				BaseName: "blank.jpg",
				Dir:      tmpDir,
			},
			expectedName: "blank.jpg.su.json",
		},
		{
			mediaFile: MediaFile{
				Path:     filepath.Join(tmpDir, "Photo on 11-1-15 at 6.24 PM #3.jpg"),
				BaseName: "Photo on 11-1-15 at 6.24 PM #3.jpg",
				Dir:      tmpDir,
			},
			expectedName: "Photo on 11-1-15 at 6.24 PM #3.jpg.supplementa.json",
		},
		{
			mediaFile: MediaFile{
				Path:     filepath.Join(tmpDir, "Bonanno1979BryanAndGrandpa45yrsOld_1.jpg"),
				BaseName: "Bonanno1979BryanAndGrandpa45yrsOld_1.jpg",
				Dir:      tmpDir,
			},
			expectedName: "Bonanno1979BryanAndGrandpa45yrsOld_1.jpg.suppl.json",
		},
		{
			mediaFile: MediaFile{
				Path:     filepath.Join(tmpDir, "VeryLongFileNameThatGetsHeavilyTruncated.jpg"),
				BaseName: "VeryLongFileNameThatGetsHeavilyTruncated.jpg",
				Dir:      tmpDir,
			},
			expectedName: "VeryLongFileNameThatGetsHeavilyTruncated.jpg.s.json",
		},
		{
			mediaFile: MediaFile{
				Path:     filepath.Join(tmpDir, "47931_1530376731723_1003886514_1589589_6447425_.jpg"),
				BaseName: "47931_1530376731723_1003886514_1589589_6447425_.jpg",
				Dir:      tmpDir,
			},
			expectedName: "47931_1530376731723_1003886514_1589589_6447425.json",
		},
		{
			mediaFile: MediaFile{
				Path:     filepath.Join(tmpDir, "TrailingUnderscore_.jpg"),
				BaseName: "TrailingUnderscore_.jpg",
				Dir:      tmpDir,
			},
			expectedName: "TrailingUnderscore.jpg.supplemental-metadata.json",
		},
		{
			mediaFile: MediaFile{
				Path:     filepath.Join(tmpDir, "BonannoJohn1959VacavilleCalifWithEvaAndDelgadoK(1).jpg"),
				BaseName: "BonannoJohn1959VacavilleCalifWithEvaAndDelgadoK(1).jpg",
				Dir:      tmpDir,
			},
			expectedName: "BonannoJohn1959VacavilleCalifWithEvaAndDelgado(1).json",
		},
		{
			mediaFile: MediaFile{
				Path:     filepath.Join(tmpDir, "BonannoJohn1959VacavilleCalifWithEvaAndDelgadoK.jpg"),
				BaseName: "BonannoJohn1959VacavilleCalifWithEvaAndDelgadoK.jpg",
				Dir:      tmpDir,
			},
			expectedName: "BonannoJohn1959VacavilleCalifWithEvaAndDelgado.json",
		},
		{
			mediaFile: MediaFile{
				Path:     filepath.Join(tmpDir, "25159_1382026223053_1003886514_1164687_6240_n.jpg"),
				BaseName: "25159_1382026223053_1003886514_1164687_6240_n.jpg",
				Dir:      tmpDir,
			},
			expectedName: "25159_1382026223053_1003886514_1164687_6240_n..json",
		},
		// Tests for '-edited' suffix: should match sidecar for non-edited base name
		{
			mediaFile: MediaFile{
				Path:     filepath.Join(tmpDir, "IMG_123-edited.jpg"),
				BaseName: "IMG_123-edited.jpg",
				Dir:      tmpDir,
			},
			expectedName: "IMG_123.jpg.json",
		},
		{
			mediaFile: MediaFile{
				Path:     filepath.Join(tmpDir, "blank-edited.jpg"),
				BaseName: "blank-edited.jpg",
				Dir:      tmpDir,
			},
			expectedName: "blank.jpg.su.json",
		},
		{
			mediaFile: MediaFile{
				Path:     filepath.Join(tmpDir, "VeryLongFileNameThatGetsHeavilyTruncated-edited.jpg"),
				BaseName: "VeryLongFileNameThatGetsHeavilyTruncated-edited.jpg",
				Dir:      tmpDir,
			},
			expectedName: "VeryLongFileNameThatGetsHeavilyTruncated.jpg.s.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.mediaFile.BaseName, func(t *testing.T) {
			sidecarPath := findSidecarFile(tt.mediaFile)
			expectedPath := filepath.Join(tmpDir, tt.expectedName)

			if sidecarPath != expectedPath {
				t.Errorf("Expected sidecar path %s, got %s", expectedPath, sidecarPath)
			}
		})
	}

	// Explicitly create '-edited' files to ensure they exist for sidecar lookup
	editedFiles := []string{
		"IMG_123-edited.jpg",
		"blank-edited.jpg",
		"VeryLongFileNameThatGetsHeavilyTruncated-edited.jpg",
	}
	for _, name := range editedFiles {
		path := filepath.Join(tmpDir, name)
		err := os.WriteFile(path, []byte("media"), 0644)
		if err != nil {
			t.Fatalf("failed to create edited file %s: %v", name, err)
		}
	}

	// Edge case: parenthesis is part of the "real" filename (not a numbering suffix)
	t.Run("literal_parenthesis_in_filename", func(t *testing.T) {
		filename := "3x (1).jpg"
		sidecar := "3x (1).jpg.supplemental-metadata.json"
		mediaPath := filepath.Join(tmpDir, filename)
		sidecarPath := filepath.Join(tmpDir, sidecar)
		err := os.WriteFile(mediaPath, []byte("media"), 0644)
		if err != nil {
			t.Fatalf("failed to create media file: %v", err)
		}
		sidecarContent := `{"title": "3x (1).jpg", "photoTakenTime": {"timestamp": "1672531200"}}`
		err = os.WriteFile(sidecarPath, []byte(sidecarContent), 0644)
		if err != nil {
			t.Fatalf("failed to create sidecar: %v", err)
		}

		mediaFile := MediaFile{
			Path:     mediaPath,
			BaseName: filename,
			Dir:      tmpDir,
		}
		found := findSidecarFile(mediaFile)
		if found != sidecarPath {
			t.Errorf("Expected literal parenthesis file to match sidecar %s, got %s", sidecarPath, found)
		}
	})

	// Edge case: truncated extension in sidecar file (e.g. .jpeg => .j.json)
	t.Run("truncated_extension_sidecar", func(t *testing.T) {
		filename := "02574B20-038B-49E5-ABCB-F8DEF39EBEF1_1_102_o.jpeg"
		sidecar := "02574B20-038B-49E5-ABCB-F8DEF39EBEF1_1_102_o.j.json"
		mediaPath := filepath.Join(tmpDir, filename)
		sidecarPath := filepath.Join(tmpDir, sidecar)
		err := os.WriteFile(mediaPath, []byte("media"), 0644)
		if err != nil {
			t.Fatalf("failed to create media file: %v", err)
		}
		sidecarContent := `{"title": "02574B20-038B-49E5-ABCB-F8DEF39EBEF1_1_102_o.jpeg", "photoTakenTime": {"timestamp": "1672531200"}}`
		err = os.WriteFile(sidecarPath, []byte(sidecarContent), 0644)
		if err != nil {
			t.Fatalf("failed to create truncated sidecar: %v", err)
		}

		mediaFile := MediaFile{
			Path:     mediaPath,
			BaseName: filename,
			Dir:      tmpDir,
		}
		found := findSidecarFile(mediaFile)
		if found != sidecarPath {
			t.Errorf("Expected truncated extension file to match sidecar %s, got %s", sidecarPath, found)
		}
	})
}

func TestGenerateDestinationPath(t *testing.T) {
	outputDir := "/output"
	fileName := "IMG_123.jpg"
	date := time.Date(2023, 4, 15, 14, 30, 0, 0, time.UTC)

	expected := filepath.Join("/output", "ALL_PHOTOS", "2023", "04", "15", "IMG_123.jpg")
	result := generateDestinationPath(outputDir, fileName, date)

	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				SourceDir: ".",
				OutputDir: "/tmp/test",
				Workers:   4,
				DryRun:    true, // Use dry run to avoid creating directories
			},
			wantErr: false,
		},
		{
			name: "missing source",
			config: &Config{
				OutputDir: "/tmp/test",
			},
			wantErr: true,
		},
		{
			name: "missing output",
			config: &Config{
				SourceDir: ".",
			},
			wantErr: true,
		},
		{
			name: "invalid workers count",
			config: &Config{
				SourceDir: ".",
				OutputDir: "/tmp/test",
				Workers:   -1,
				DryRun:    true,
			},
			wantErr: false, // Should be corrected to 4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)

			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check that negative workers get corrected
			if tt.config.Workers <= 0 && err == nil {
				if tt.config.Workers != 4 {
					t.Errorf("Expected workers to be corrected to 4, got %d", tt.config.Workers)
				}
			}
		})
	}
}

func TestSupportedFileTypes(t *testing.T) {
	testCases := []struct {
		filename  string
		supported bool
	}{
		{"image.jpg", true},
		{"image.JPEG", true},
		{"image.png", true},
		{"video.mp4", true},
		{"video.MOV", true},
		{"document.pdf", false},
		{"text.txt", false},
		{"archive.zip", false},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			ext := filepath.Ext(tc.filename)
			ext = strings.ToLower(ext)

			isSupported := supportedExts[ext]

			if isSupported != tc.supported {
				t.Errorf("File %s: expected supported=%t, got supported=%t",
					tc.filename, tc.supported, isSupported)
			}
		})
	}
}

func BenchmarkExifToolPersistent(b *testing.B) {
	// Skip if exiftool is not available
	if _, err := exec.LookPath("exiftool"); err != nil {
		b.Skip("ExifTool not available, skipping benchmark")
	}

	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)

	etm, err := NewExifToolManager(1) // Single worker for benchmark
	if err != nil {
		b.Fatalf("Failed to create ExifTool manager: %v", err)
	}
	defer etm.Close()

	process := etm.GetProcessForWorker(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		process.GetMetadata(testFile)
	}
}

func BenchmarkParseSidecarDate(b *testing.B) {
	tmpDir := b.TempDir()
	sidecarPath := filepath.Join(tmpDir, "test.json")

	sidecarData := SidecarData{
		Title: "test.jpg",
		PhotoTakenTime: struct {
			Timestamp string `json:"timestamp"`
		}{
			Timestamp: "1672531200",
		},
	}

	data, _ := json.Marshal(sidecarData)
	os.WriteFile(sidecarPath, data, 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseSidecarDate(sidecarPath)
	}
}

func TestGetAlbumName(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with valid album metadata
	albumMetadata := AlbumMetadata{
		Title: "Test Album",
	}

	data, err := json.Marshal(albumMetadata)
	if err != nil {
		t.Fatal(err)
	}

	metadataPath := filepath.Join(tmpDir, "metadata.json")
	err = os.WriteFile(metadataPath, data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	albumName := getAlbumName(tmpDir)
	if albumName != "Test Album" {
		t.Errorf("Expected album name 'Test Album', got '%s'", albumName)
	}

	// Test with no metadata file
	tmpDir2 := t.TempDir()
	albumName2 := getAlbumName(tmpDir2)
	if albumName2 != "" {
		t.Errorf("Expected empty album name for directory without metadata, got '%s'", albumName2)
	}
}

func TestGenerateAlbumSymlinkPath(t *testing.T) {
	outputDir := "/output"
	albumName := "My Vacation"
	fileName := "IMG_123.jpg"

	expected := filepath.Join("/output", "ALBUMS", "My Vacation", "IMG_123.jpg")
	result := generateAlbumSymlinkPath(outputDir, albumName, fileName)

	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestGenerateDestinationPathWithAllPhotos(t *testing.T) {
	outputDir := "/output"
	fileName := "IMG_123.jpg"
	date := time.Date(2023, 4, 15, 14, 30, 0, 0, time.UTC)

	expected := filepath.Join("/output", "ALL_PHOTOS", "2023", "04", "15", "IMG_123.jpg")
	result := generateDestinationPath(outputDir, fileName, date)

	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestCreateAlbumSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create target file structure
	targetDir := filepath.Join(tmpDir, "ALL_PHOTOS", "2023", "04", "15")
	err := os.MkdirAll(targetDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	targetFile := filepath.Join(targetDir, "test.jpg")
	err = os.WriteFile(targetFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create symlink
	symlinkPath := filepath.Join(tmpDir, "ALBUMS", "Test Album", "test.jpg")
	err = createAlbumSymlink(targetFile, symlinkPath)
	if err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Verify symlink exists and points to correct file
	linkTarget, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	// The link should be relative
	expectedRelPath := "../../ALL_PHOTOS/2023/04/15/test.jpg"
	if linkTarget != expectedRelPath {
		t.Errorf("Expected symlink target %s, got %s", expectedRelPath, linkTarget)
	}

	// Verify we can read through the symlink
	content, err := os.ReadFile(symlinkPath)
	if err != nil {
		t.Fatalf("Failed to read through symlink: %v", err)
	}

	if string(content) != "test content" {
		t.Errorf("Expected content 'test content', got '%s'", string(content))
	}
}

func TestExifToolPersistentMode(t *testing.T) {
	// Skip if exiftool is not available
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("ExifTool not available, skipping persistent mode test")
	}

	etm, err := NewExifToolManager(2) // Create manager with 2 workers
	if err != nil {
		t.Fatalf("Failed to create ExifTool manager: %v", err)
	}
	defer etm.Close()

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test both worker processes
	for workerID := 0; workerID < 2; workerID++ {
		process := etm.GetProcessForWorker(workerID)

		// Test multiple calls to ensure persistent mode is working
		for i := 0; i < 3; i++ {
			metadata, err := process.GetMetadata(testFile)
			if err != nil {
				t.Fatalf("Worker %d failed to get metadata on iteration %d: %v", workerID, i, err)
			}

			// Should at least have FileName
			if filename, exists := metadata["FileName"]; !exists || filename != "test.txt" {
				t.Errorf("Worker %d expected FileName 'test.txt', got '%s'", workerID, filename)
			}
		}
	}
}

func TestExifToolConcurrency(t *testing.T) {
	// Skip if exiftool is not available
	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("ExifTool not available, skipping concurrency test")
	}

	workerCount := 4
	etm, err := NewExifToolManager(workerCount)
	if err != nil {
		t.Fatalf("Failed to create ExifTool manager: %v", err)
	}
	defer etm.Close()

	// Create test files
	tmpDir := t.TempDir()
	testFiles := make([]string, 10)
	for i := 0; i < 10; i++ {
		testFile := filepath.Join(tmpDir, fmt.Sprintf("test%d.txt", i))
		err := os.WriteFile(testFile, []byte(fmt.Sprintf("test content %d", i)), 0644)
		if err != nil {
			t.Fatal(err)
		}
		testFiles[i] = testFile
	}

	// Test concurrent access to different ExifTool processes
	var wg sync.WaitGroup
	results := make(chan error, workerCount*len(testFiles))

	for workerID := 0; workerID < workerCount; workerID++ {
		wg.Add(1)
		go func(wID int) {
			defer wg.Done()
			process := etm.GetProcessForWorker(wID)

			// Each worker processes all files with its dedicated ExifTool process
			for _, file := range testFiles {
				_, err := process.GetMetadata(file)
				if err != nil {
					results <- fmt.Errorf("worker %d failed on %s: %v", wID, file, err)
					return
				}
			}
			results <- nil // Success
		}(workerID)
	}

	wg.Wait()
	close(results)

	// Check results
	successCount := 0
	for result := range results {
		if result != nil {
			t.Errorf("Concurrent test failed: %v", result)
		} else {
			successCount++
		}
	}

	if successCount != workerCount {
		t.Errorf("Expected %d successful workers, got %d", workerCount, successCount)
	}
}

func TestIsGooglePhotosSidecar(t *testing.T) {
	tmpDir := t.TempDir()

	// Valid Google Photos sidecar
	validSidecar := filepath.Join(tmpDir, "valid.json")
	validContent := `{"title": "photo.jpg", "photoTakenTime": {"timestamp": "1672531200"}}`
	os.WriteFile(validSidecar, []byte(validContent), 0644)

	// Invalid JSON file (not a Google Photos sidecar)
	invalidSidecar := filepath.Join(tmpDir, "invalid.json")
	invalidContent := `{"some": "other", "json": "structure"}`
	os.WriteFile(invalidSidecar, []byte(invalidContent), 0644)

	// Test valid sidecar
	if !isGooglePhotosSidecar(validSidecar) {
		t.Error("Expected valid Google Photos sidecar to be recognized")
	}

	// Test invalid sidecar
	if isGooglePhotosSidecar(invalidSidecar) {
		t.Error("Expected invalid sidecar to be rejected")
	}

	// Test non-existent file
	if isGooglePhotosSidecar(filepath.Join(tmpDir, "nonexistent.json")) {
		t.Error("Expected non-existent file to be rejected")
	}
}

func TestSidecarRegexMatching(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a media file and various potential sidecar files
	mediaFile := "Some Very Long Photo Name That Gets Truncated.jpg"
	os.WriteFile(filepath.Join(tmpDir, mediaFile), []byte("test image"), 0644)

	// Create valid Google Photos sidecar with truncated name
	sidecarContent := `{"title": "Some Very Long Photo Name That Gets Truncated.jpg", "photoTakenTime": {"timestamp": "1672531200"}}`
	truncatedSidecar := "Some Very Long Photo Name That Gets Truncated.jpg.supplement.json"
	os.WriteFile(filepath.Join(tmpDir, truncatedSidecar), []byte(sidecarContent), 0644)

	// Create invalid JSON file that starts with same name
	invalidJSON := "Some Very Long Photo Name That Gets Truncated.jpg.other.json"
	os.WriteFile(filepath.Join(tmpDir, invalidJSON), []byte(`{"other": "data"}`), 0644)

	file := MediaFile{
		Path:     filepath.Join(tmpDir, mediaFile),
		BaseName: mediaFile,
		Dir:      tmpDir,
	}

	result := findSidecarFile(file)
	expected := filepath.Join(tmpDir, truncatedSidecar)

	if result != expected {
		t.Errorf("Expected regex match to find %s, got %s", expected, result)
	}
}

func BenchmarkSidecarMatchingRegex(b *testing.B) {
	tmpDir := b.TempDir()

	// Create test files with various truncation patterns including trailing underscore edge case
	testCases := []struct {
		media   string
		sidecar string
	}{
		{"IMG_123.jpg", "IMG_123.jpg.supplemental-metadata.json"},
		{"Photo on 11-1-15 at 6.24 PM #3.jpg", "Photo on 11-1-15 at 6.24 PM #3.jpg.supplementa.json"},
		{"Bonanno1979BryanAndGrandpa45yrsOld_1.jpg", "Bonanno1979BryanAndGrandpa45yrsOld_1.jpg.suppl.json"},
		{"VeryLongFileName.jpg", "VeryLongFileName.jpg.s.json"},
		{"NumberedFile(1).jpg", "NumberedFile.jpg.supplemental-metadata(1).json"},
		{"47931_1530376731723_1003886514_1589589_6447425_.jpg", "47931_1530376731723_1003886514_1589589_6447425.json"},
		{"TrailingUnderscore_.jpg", "TrailingUnderscore.jpg.supplemental-metadata.json"},
		{"BonannoJohn1959VacavilleCalifWithEvaAndDelgadoK(1).jpg", "BonannoJohn1959VacavilleCalifWithEvaAndDelgado(1).json"},
		{"BonannoJohn1959VacavilleCalifWithEvaAndDelgadoK.jpg", "BonannoJohn1959VacavilleCalifWithEvaAndDelgado.json"},
		{"25159_1382026223053_1003886514_1164687_6240_n.jpg", "25159_1382026223053_1003886514_1164687_6240_n..json"},
	}

	sidecarContent := `{"title": "test.jpg", "photoTakenTime": {"timestamp": "1672531200"}}`

	for _, tc := range testCases {
		os.WriteFile(filepath.Join(tmpDir, tc.media), []byte("test image"), 0644)
		os.WriteFile(filepath.Join(tmpDir, tc.sidecar), []byte(sidecarContent), 0644)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			file := MediaFile{
				Path:     filepath.Join(tmpDir, tc.media),
				BaseName: tc.media,
				Dir:      tmpDir,
			}
			findSidecarFile(file)
		}
	}
}

func BenchmarkParseExifDate(b *testing.B) {
	dateStr := "2023:01:01 12:30:45"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseExifDate(dateStr)
	}
}

func TestTrailingUnderscoreEdgeCase(t *testing.T) {
	tmpDir := t.TempDir()

	// Create the real-world example files
	mediaFile := "47931_1530376731723_1003886514_1589589_6447425_.jpg"
	sidecarFile := "47931_1530376731723_1003886514_1589589_6447425.json"

	sidecarContent := `{"title": "47931_1530376731723_1003886514_1589589_6447425_.jpg", "photoTakenTime": {"timestamp": "1672531200"}}`

	// Create files
	err := os.WriteFile(filepath.Join(tmpDir, mediaFile), []byte("test image content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, sidecarFile), []byte(sidecarContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	file := MediaFile{
		Path:     filepath.Join(tmpDir, mediaFile),
		BaseName: mediaFile,
		Dir:      tmpDir,
	}

	result := findSidecarFile(file)
	expected := filepath.Join(tmpDir, sidecarFile)

	if result != expected {
		t.Errorf("Expected trailing underscore edge case to find %s, got %s", expected, result)
	}

	// Test another variant
	mediaFile2 := "AnotherExample_.jpg"
	sidecarFile2 := "AnotherExample.jpg.su.json"

	sidecarContent2 := `{"title": "AnotherExample_.jpg", "photoTakenTime": {"timestamp": "1672531300"}}`

	err = os.WriteFile(filepath.Join(tmpDir, mediaFile2), []byte("test image content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, sidecarFile2), []byte(sidecarContent2), 0644)
	if err != nil {
		t.Fatal(err)
	}

	file2 := MediaFile{
		Path:     filepath.Join(tmpDir, mediaFile2),
		BaseName: mediaFile2,
		Dir:      tmpDir,
	}

	result2 := findSidecarFile(file2)
	expected2 := filepath.Join(tmpDir, sidecarFile2)

	if result2 != expected2 {
		t.Errorf("Expected second trailing underscore case to find %s, got %s", expected2, result2)
	}
}

func TestArbitraryTruncationEdgeCase(t *testing.T) {
	// Create the real-world truncation examples
	testCases := []struct {
		mediaFile   string
		sidecarFile string
	}{
		{
			"BonannoJohn1959VacavilleCalifWithEvaAndDelgadoK(1).jpg",
			"BonannoJohn1959VacavilleCalifWithEvaAndDelgado(1).json",
		},
		{
			"BonannoJohn1959VacavilleCalifWithEvaAndDelgadoK.jpg",
			"BonannoJohn1959VacavilleCalifWithEvaAndDelgado.json",
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Case%d", i+1), func(t *testing.T) {
			// Create separate directory for each test case to avoid cross-matching
			tmpDir := t.TempDir()

			sidecarContent := fmt.Sprintf(`{"title": "%s", "photoTakenTime": {"timestamp": "%d"}}`, tc.mediaFile, 1672531200+i*3600)

			// Create files
			err := os.WriteFile(filepath.Join(tmpDir, tc.mediaFile), []byte("test image content"), 0644)
			if err != nil {
				t.Fatal(err)
			}

			err = os.WriteFile(filepath.Join(tmpDir, tc.sidecarFile), []byte(sidecarContent), 0644)
			if err != nil {
				t.Fatal(err)
			}

			file := MediaFile{
				Path:     filepath.Join(tmpDir, tc.mediaFile),
				BaseName: tc.mediaFile,
				Dir:      tmpDir,
			}

			result := findSidecarFile(file)
			expected := filepath.Join(tmpDir, tc.sidecarFile)

			if result != expected {
				t.Errorf("Expected arbitrary truncation case to find %s, got %s", expected, result)
			}
		})
	}
}

func TestDoubleDotEdgeCase(t *testing.T) {
	tmpDir := t.TempDir()

	// Create the double-dot edge case example
	mediaFile := "25159_1382026223053_1003886514_1164687_6240_n.jpg"
	sidecarFile := "25159_1382026223053_1003886514_1164687_6240_n..json"

	sidecarContent := `{"title": "25159_1382026223053_1003886514_1164687_6240_n.jpg", "photoTakenTime": {"timestamp": "1672531200"}}`

	// Create files
	err := os.WriteFile(filepath.Join(tmpDir, mediaFile), []byte("test image content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, sidecarFile), []byte(sidecarContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	file := MediaFile{
		Path:     filepath.Join(tmpDir, mediaFile),
		BaseName: mediaFile,
		Dir:      tmpDir,
	}

	result := findSidecarFile(file)
	expected := filepath.Join(tmpDir, sidecarFile)

	if result != expected {
		t.Errorf("Expected double-dot edge case to find %s, got %s", expected, result)
	}
}
