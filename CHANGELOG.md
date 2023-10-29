# Change Log

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
