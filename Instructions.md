Create a Go command-line application to clean up a Google Photos Takeout export. The application should **recursively scan a source directory** and process all media files supported by ExifTool. It must use **goroutines** and a single **ExifTool instance in persistent mode** for efficiency.

The repository includes a test directory named test/src/ with a variety of test media files and their corresponding JSON sidecar files, which the application should use for testing.

### Core Logic

For each media file found, the application should perform the following steps:

1.  **Extract Exif Metadata**:
    * Use ExifTool to extract metadata.
    * Prioritize finding a valid creation date from the following Exif tags, in this order: `DateTimeOriginal`, `CreationDate`, `CreateDate`, `MediaCreateDate`, `DateTimeCreated`.

2.  **Handle Missing Dates**:
    * If no date is found in the specified Exif tags, check for an accompanying JSON sidecar file in the same directory.
    * The sidecar's filename will start with the media file's name.
    * Be able to handle these two naming conventions for the sidecar:
        * The sidecar may or may not include `.supplemental-metadata` before the `.json` extension, which can be truncated with long filenames.
        * If the media file name ends with `(number).extension` (e.g., `IMG_123(2).jpg`), the JSON sidecar will also have the same `(number)` suffix (e.g., `IMG_123.jpg.supplemental-metadata(2).json`).

3.  **Process and Update Files**:
    * If a JSON sidecar is found, parse it to get the creation date.
    * Use this date to update the media file's Exif tags (e.g., using `exiftool -overwrite_original -AllDates="%s"`) if they are missing or incorrect.
    * Implement an option to **move** files with a valid creation date to a new directory structure organized by year, month, and day (e.g., `output_dir/YYYY/MM/DD/original_filename.ext`).

***

### Efficiency and Error Handling

* Use a **worker pool pattern with goroutines** to process files concurrently.
* Initialize only a single ExifTool process in persistent mode to avoid the overhead of starting a new process for each file.
* Include robust error handling for file I/O, JSON parsing, and ExifTool operations.
* Provide clear progress indicators to the user during the scan and processing stages.

***

### Command-Line Interface

The application should have a command-line interface with the following flags:

* `-source`: Path to the Google Photos Takeout root directory.
* `-output`: Path to the output directory for cleaned files.
* `-move`: Optional flag to enable moving files to the new directory structure.
* `-dry-run`: **Optional flag to simulate the process without making any changes to files or directories.** The application should print what actions it *would* take (e.g., "Would update Exif tags for file: `IMG_123.jpg`", "Would move file `IMG_123.jpg` to `output/2023/04/15/`").

The final application should be a single binary, cross-platform compatible.
