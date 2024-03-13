# Change Log

## [0.3.0] - 2024-03-13

### Added

- Implement multipart upload in the B2Backend for large files (with fixed 5 retry attemps & 5 second delay)
- Introduce ProgressReporterFacade interface for progress reporting settings
- Add isfile field to FileInfo and support FileInfo on prefixes in B2Backend

### Fixed

- Minor refactor of the test suite.
- Some more spelling mistakes.

### Changed

- More secure sequence of operations to prevent deleting existing archives in case backup fails
- Provide verbose output and progress tracking when corresponding flag is specified

## [0.2.0] - 2023-10-28

### Added

- Backup files rotation based on modified date.
- Command line switch to provide custom configuration.
- RemoveFile function to StorageBackend interface
- GenerateDummyFiles that generates dummy file info for DummyBackend.ListFiles
- Getters for FileInfo fields
- SetDefaultValues pointer receiver that resets fields of Config struct to default values
- Implement new handler for command line arguments

### Fixed

- Refactor & extend test suite.
- Various spelling mistakes.

### Changed

- Expect second command line argument to be path to a prefix on storage backend (no longer a path to file).
- Add '.age' to file extension only when encryption is enabled.
- LoadConfigFromFile and LoadConfigFromEnv are now pointer receivers of Config.
- Run as local user in docker container during build and test.
- Mount local go build cache into the docker container during build and test.

## [0.1.0] - 2023-09-17

_First release._

[0.1.0]: https://github.com/breezerider/squirrelup/releases/tag/v0.1.0
