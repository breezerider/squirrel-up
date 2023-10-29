# SquirrelUp

[![codecov](https://codecov.io/gh/breezerider/squirrel-up/branch/main/graph/badge.svg)](https://codecov.io/gh/breezerider/squirrel-up)

## Description

SquirrelUp was designed to backup individual data snapshots provided in a directory, like [rsnapshot](https://rsnapshot.org/) backups.
SquirrelUp can put the snapshot into a TAR file, compress it with GZip using [archiver](https://github.com/mholt/archiver).
Optionally, it can encrypt the file with asymmetric encription using [age](https://github.com/FiloSottile/age).
Then it will store the output file to a storage backen (currently only BackBlaze B2 storage is available).

## Usage

```shell
$ squirrelup
Usage: squirrelup <backup_dir> <output_prefix_uri>
    Create an (optionally) encrypted gzip-compressed TAR file and upload it to storage backend.
    At the moment only BackBlaze B2 cloud storage is implemented.

Required arguments:
    <backup_dir>                  Path to local directory that serves as backup root.
    <output_prefix_uri>           Remote URI prefix.

Optional arguments:
    --config, -c <config_file>    Path to local config file.
    --verbose, -v                 Verbose output.

BackBlaze B2 Backend:
    <output_prefix_uri> must follow the pattern 'b2://<bucket>/<path>/<to>/<prefix>/'.

Default configuration is stored under <config_path>.
```

## Requirements

* Docker
* Make

## Build

Build squirrelup by running:

```shell
make build
```

## Test

Run the test suite for squirrelup by running:

```shell
make test
```

## Acknowledgements

Project template generated using [inizio](https://github.com/insidieux/inizio)
