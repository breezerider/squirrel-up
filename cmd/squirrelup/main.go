package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/breezerider/squirrel-up/pkg/common"
	"github.com/mholt/archiver/v4"
)

const (
	appname = "SquirrelUp"

	usage = `Usage: %s <backup_dir> <output_prefix_uri>
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

Default configuration is stored under %s.
`
)

type CLI struct {
	Verbose        bool
	ConfigFilepath string
	PositionalArgs []string
}

var (
	version               string
	commit                string
	date                  string
	defaultConfigFilepath string
)

// return the usage string.
func usageString(name string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, usage, name, defaultConfigFilepath)
	return builder.String()
}

// isDirectory determines if a file represented
// by `path` is a directory or not.
func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("stat call failed on %q: %s", path, err.Error())
	}

	return fileInfo.IsDir(), nil
}

// see https://pace.dev/blog/2020/02/12/why-you-shouldnt-use-func-main-in-golang-by-mat-ryer.html
func main() {
	if err := run(os.Args, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	/* handle input arguments */
	var cli_args CLI

	if len(args) >= 2 {
		// config
		var storeConfig bool = false

		for _, arg := range args[1:] {
			if storeConfig {
				cli_args.ConfigFilepath = arg
				storeConfig = false
				continue
			}

			switch arg {
			case "--help", "-h":
				fmt.Fprintf(stdout, "%s\n", usageString(args[0]))
				return nil
			case "--version", "-V":
				fmt.Fprintf(stdout, "%s v%s (commit hash:%s | date:%s)\n", appname, version, commit, date)
				return nil
			case "--verbose", "-v":
				cli_args.Verbose = true
			case "--config", "-c":
				storeConfig = true
			default:
				if strings.HasPrefix(arg, "-") {
					return fmt.Errorf("unrecognize command line option '%s'", arg)
				}

				if len(cli_args.PositionalArgs) >= 2 {
					cli_args.PositionalArgs = append(cli_args.PositionalArgs, "")
					break
				}

				cli_args.PositionalArgs = append(cli_args.PositionalArgs, arg)
			}
		}

		if storeConfig {
			return fmt.Errorf("invalid use of the configuration switch, must provide a value")
		}
	}

	if len(cli_args.PositionalArgs) != 2 {
		fmt.Fprintf(stderr, "%s\n", usageString(args[0]))
		return fmt.Errorf("wrong number of arguments, expecting exactly 2 positional arguments")
	}

	// process first input argument
	var inputDirectory string = cli_args.PositionalArgs[0]
	if isDir, err := isDirectory(inputDirectory); !isDir {
		if err != nil {
			return fmt.Errorf("first argument must be a valid directory path: %s", err.Error())
		} else {
			return fmt.Errorf("first argument must be a valid directory path")
		}
	}

	// process second input argument
	outputPrefixUri, err := url.ParseRequestURI(cli_args.PositionalArgs[1])
	if err != nil {
		return fmt.Errorf("could not parse output URI: %s", err.Error())
	}

	/* load configuration */
	var cfg common.Config

	if len(cli_args.ConfigFilepath) == 0 {
		cli_args.ConfigFilepath = defaultConfigFilepath
	}
	err = initConfig(&cfg, cli_args.ConfigFilepath, stdout, stderr)
	if err != nil {
		return fmt.Errorf("%s", err.Error())
	}

	/* initialize the backend */
	backend, err := common.CreateStorageBackend(outputPrefixUri, &cfg)
	if err != nil {
		return fmt.Errorf("failed to create backend: %s", err.Error())
	}

	/* validate output URI */
	// fileinfo, err := backend.GetFileInfo(outputPrefixUri)
	// if err != nil {
	// 	if err.Error() == common.ErrFileNotFound {
	// 		fmt.Fprintf(stderr, "file %q not found\n", outputPrefixUri)
	// 	} else {
	// 		return fmt.Errorf("backend operation failed: %s", err.Error())
	// 	}
	// } else {
	// 	fmt.Fprintf(stdout, "file info: %+v\n", *fileinfo)
	// }

	/* list prefix contents */
	filelist, err := backend.ListFiles(outputPrefixUri)
	if err != nil {
		return fmt.Errorf("could not list remote files: %s", err.Error())
	}

	/* remove old files */
	if cfg.Backup.Hours > 0.0 {
		timeNow := time.Now()
		for _, fileinfo := range filelist {
			diff := timeNow.Sub(fileinfo.Modified())
			fmt.Fprintf(stderr, "file %s, time diff = %.0f h\n", fileinfo.Name(), diff.Hours())
			if diff.Hours() >= cfg.Backup.Hours {
				relativeUri, err := outputPrefixUri.Parse(fileinfo.Name())
				if err == nil {
					fmt.Fprintf(stdout, "removing file %q\n", relativeUri)
					err = backend.RemoveFile(relativeUri)
				}
				if err != nil {
					fmt.Fprintf(stderr, "could not remove remote files: %s\n", err.Error())
				}
			}
		}
	}

	/* initialize encryption */
	var recipients []age.Recipient
	recipients, err = initEncryption(&cfg, stdout, stderr)
	if err != nil {
		return fmt.Errorf("%s", err.Error())
	}

	/* create an archive from the input directory */
	var outputArchivePath string
	outputArchivePath, err = archiveDirectory(inputDirectory)
	if err != nil {
		_ = os.Remove(outputArchivePath)
		return fmt.Errorf("%s", err.Error())
	}

	/* encrypt the output file */
	var outputEncryptedPath string
	if len(recipients) > 0 {
		fmt.Fprintf(stderr, "recipients: %+v\n", recipients)
		outputEncryptedPath, err = encryptFile(outputArchivePath, recipients, stdout, stderr)
		if err != nil {
			_ = os.Remove(outputArchivePath)
			_ = os.Remove(outputEncryptedPath)
			return fmt.Errorf("%s", err.Error())
		}
	} else {
		// report no pubkey
		fmt.Fprintf(stderr, "no pubkey found, encryption disabled\n")
		outputEncryptedPath = outputArchivePath
	}

	/* store output file */
	var errorMessage string
	outputFile, err := os.Open(filepath.Clean(outputEncryptedPath))
	if err == nil {
		relativeUri, err := outputPrefixUri.Parse(time.Now().Format(cfg.Backup.Name) + ".tar.gz")
		if err == nil {
			err = backend.StoreFile(io.ReadSeekCloser(outputFile), relativeUri)
			if err == nil {
				// report progress
				fmt.Fprintf(stdout, "wrote %q to %q\n", inputDirectory, relativeUri)
			}
		}
		if err != nil {
			errorMessage = fmt.Sprintf("unable to write %q to %q: %s", inputDirectory, relativeUri, err.Error())
		}
	} else {
		errorMessage = fmt.Sprintf("could not open output file: %s", err.Error())
	}

	/* clean up */
	_ = outputFile.Close()
	_ = os.Remove(outputArchivePath)
	_ = os.Remove(outputEncryptedPath)

	if err != nil {
		return fmt.Errorf(errorMessage)
	}

	return nil
}

func initConfig(cfg *common.Config, cfgFilepath string, stdout, stderr io.Writer) error {
	var err error

	err = cfg.SetDefaultValues()
	if err != nil {
		return fmt.Errorf("could set default configuration values: %s", err.Error())
	}

	if len(cfgFilepath) > 0 {
		var configFile *os.File
		configFile, err = os.Open(filepath.Clean(cfgFilepath))
		if err == nil {
			fmt.Fprintf(stderr, "loading configuration from %s\n", filepath.Clean(cfgFilepath))
			err = cfg.LoadConfigFromFile(configFile)
			_ = configFile.Close()
		}
	} else {
		fmt.Fprintf(stderr, "default configuration path is empty\n")
		err = nil
	}

	if err != nil && err != io.EOF {
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(stderr, "configuration file %s does not exist\n", cfgFilepath)
		} else {
			return fmt.Errorf("could not load configuration from %s: %s", cfgFilepath, err.Error())
		}
	}

	err = cfg.LoadConfigFromEnv()
	if err != nil {
		return fmt.Errorf("could not load configuration from environment: %s", err.Error())
	}

	return nil
}

func initEncryption(cfg *common.Config, stdout, stderr io.Writer) ([]age.Recipient, error) {
	var recipients []age.Recipient

	if len(cfg.Encryption.Pubkey) > 0 {
		if r, err := age.ParseX25519Recipient(cfg.Encryption.Pubkey); err == nil {
			recipients = append(recipients, r)
		} else {
			fmt.Fprintf(stderr, "pubkey parsing failed, assuming it is path to file\n")

			pubkeyFile, err := os.Open(cfg.Encryption.Pubkey)
			if err != nil {
				return nil, fmt.Errorf("could not open pubkey file: %s", err.Error())
			}

			recipients, err = age.ParseRecipients(pubkeyFile)
			if err != nil {
				return nil, fmt.Errorf("parsing pubkey file failed: %s", err.Error())
			}
		}
	}

	return recipients, nil
}

func archiveDirectory(dirPath string) (string, error) {
	// map files on disk to their paths in the archive
	files, err := archiver.FilesFromDisk(nil, map[string]string{
		dirPath: "",
	})
	if err != nil {
		return "", fmt.Errorf("could not initialize archive files structure: %s", err.Error())
	}

	// create the output file we'll write to
	tmp, err := os.CreateTemp("", appname+"-backup-")
	if err != nil {
		return "", fmt.Errorf("could not create temporary file: %s", err.Error())
	}

	// we can use the CompressedArchive type to gzip a tarball
	// (compression is not required; you could use Tar directly)
	format := archiver.CompressedArchive{
		Compression: archiver.Gz{},
		Archival:    archiver.Tar{NumericUIDGID: true},
	}

	// create the archive
	err = format.Archive(context.Background(), tmp, files)
	if err != nil {
		return "", fmt.Errorf("failed to generate archive: %s", err.Error())
	}

	// close the file
	_ = tmp.Close()

	return tmp.Name(), nil
}

func encryptFile(filePath string, recipients []age.Recipient, stdout, stderr io.Writer) (string, error) {
	// open input file
	input, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return "", fmt.Errorf("could not open input file %s: %s", filePath, err.Error())
	}

	// create the output file we'll write to
	tmp, err := os.CreateTemp("", appname+"-encrypted-")
	if err != nil {
		return "", fmt.Errorf("could not create temporary file: %s", err.Error())
	}

	// create the encrypted writer
	encryptedWriter, err := age.Encrypt(tmp, recipients...)
	if err != nil {
		return "", fmt.Errorf("could not initlize encryption for file '%s': %s", tmp.Name(), err.Error())
	}

	// encrypt the file
	numWritten, err := io.Copy(encryptedWriter, input)
	if err != nil {
		return "", fmt.Errorf("could not write file '%s' to encrypted file '%s': %s", input.Name(), tmp.Name(), err.Error())
	} else if numWritten == 0 {
		return "", fmt.Errorf("zero bytes written to encrypted archive")
	}
	_ = encryptedWriter.Close()

	return tmp.Name(), nil
}
