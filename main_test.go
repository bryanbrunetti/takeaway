package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	// Create test files
	testFiles := []string{
		"IMG_123.jpg",
		"IMG_123.jpg.json",
		"IMG_456(1).jpg",
		"IMG_456.jpg.supplemental-metadata(1).json",
		"blank.jpg",
		"blank.jpg.su.json",
	}

	for _, filename := range testFiles {
		path := filepath.Join(tmpDir, filename)
		err := os.WriteFile(path, []byte("test"), 0644)
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

	etm, err := NewExifToolManager()
	if err != nil {
		b.Fatalf("Failed to create ExifTool manager: %v", err)
	}
	defer etm.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		etm.GetMetadata(testFile)
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

	etm, err := NewExifToolManager()
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

	// Test multiple calls to ensure persistent mode is working
	for i := 0; i < 3; i++ {
		metadata, err := etm.GetMetadata(testFile)
		if err != nil {
			t.Fatalf("Failed to get metadata on iteration %d: %v", i, err)
		}

		// Should at least have FileName
		if filename, exists := metadata["FileName"]; !exists || filename != "test.txt" {
			t.Errorf("Expected FileName 'test.txt', got '%s'", filename)
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
