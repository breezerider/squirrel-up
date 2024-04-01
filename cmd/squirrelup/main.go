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

type (
	cliArgs struct {
		Verbose        bool
		ConfigFilepath string
		PositionalArgs []string
	}

	progressWriter struct {
		common.ProgressReporter
		Index int
	}
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

var (
	version               string
	commit                string
	date                  string
	defaultConfigFilepath string
)

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	err = pw.AdvanceTask(pw.Index, int64(n))
	return
}

// return the usage string.
func usageString(name string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, usage, name, defaultConfigFilepath)
	return builder.String()
}

// isDirectory returns true if a path points to a directory.
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
	var cli_args cliArgs
	var err error

	if terminate, err := parseArgs(args, &cli_args, stdout, stderr); err != nil {
		return fmt.Errorf("%s", err.Error())
	} else if terminate {
		return nil
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

	if cli_args.Verbose {
		fmt.Fprintf(stderr, "loading configuration...\n")
	}
	if len(cli_args.ConfigFilepath) == 0 {
		cli_args.ConfigFilepath = defaultConfigFilepath
	}
	err = initConfig(&cfg, cli_args.ConfigFilepath, stdout, stderr)
	if err != nil {
		return fmt.Errorf("%s", err.Error())
	}
	if cli_args.Verbose {
		cfg.Internal.Reporter = common.NewMultiProgressbarReporter(stdout)
	}

	/* initialize the backend */
	if cli_args.Verbose {
		fmt.Fprintf(stderr, "intializing backend & verifying settings...\n")
	}
	backend, err := common.CreateStorageBackend(outputPrefixUri, &cfg)
	if err != nil {
		return fmt.Errorf("failed to create backend: %s", err.Error())
	}

	/* validate output URI */
	fileinfo, err := backend.GetFileInfo(outputPrefixUri)
	if err != nil {
		if err.Error() == common.ErrFileNotFound {
			fmt.Fprintf(stderr, "file %q not found\n", outputPrefixUri)
		} else {
			return fmt.Errorf("backend operation failed: %s", err.Error())
		}
	} else {
		fmt.Fprintf(stdout, "file info: %+v\n", *fileinfo)
		if fileinfo.IsFile() {
			return fmt.Errorf("output URI must be a directory prefix, but a file path was specified: %q", outputPrefixUri)
		}
	}

	/* initialize encryption */
	if cli_args.Verbose {
		fmt.Fprintf(stderr, "initializing encryption...\n")
	}
	var recipients []age.Recipient
	recipients, err = initEncryption(&cfg, stdout, stderr)
	if err != nil {
		return fmt.Errorf("%s", err.Error())
	}

	/* create an archive from the input directory */
	if cli_args.Verbose {
		fmt.Fprintf(stderr, "generating backup archive...\n")
	}
	var outputArchivePath string
	outputArchivePath, err = archiveDirectory(inputDirectory, &cfg)
	if err != nil {
		_ = os.Remove(outputArchivePath)
		return fmt.Errorf("%s", err.Error())
	}

	/* encrypt the output file */
	var outputEncryptedPath, outputFileExtension string
	if len(recipients) > 0 {
		if cli_args.Verbose {
			fmt.Fprintf(stderr, "encrypting backup archive for recipients: %+v\n", recipients)
		}
		outputEncryptedPath, err = encryptFile(outputArchivePath, recipients, &cfg)
		if err != nil {
			_ = os.Remove(outputArchivePath)
			_ = os.Remove(outputEncryptedPath)
			return fmt.Errorf("%s", err.Error())
		}
		outputFileExtension = ".tar.gz.age"
	} else {
		// report no pubkey
		if cli_args.Verbose {
			fmt.Fprintf(stderr, "no pubkey found, encryption disabled\n")
		}
		outputEncryptedPath = outputArchivePath
		outputFileExtension = ".tar.gz"
	}

	/* store output file */
	if cli_args.Verbose {
		fmt.Fprintf(stderr, "uploading backup archive...\n")
	}
	var errorMessage string
	var outputFile *os.File
	outputFile, err = os.Open(filepath.Clean(outputEncryptedPath))
	if err == nil {
		var relativeUri *url.URL
		relativeUri, err = outputPrefixUri.Parse(time.Now().Format(cfg.Backup.Name) + outputFileExtension)
		if err == nil {
			if cli_args.Verbose {
				fmt.Fprintf(stderr, "uploading backup archive of %q to %q\n", inputDirectory, relativeUri)
			}

			var fileInfo os.FileInfo
			fileInfo, err = outputFile.Stat()
			if err == nil {
				err = backend.StoreFile(io.ReaderAt(outputFile), fileInfo.Size(), relativeUri)
			}
		}
		if err != nil {
			errorMessage = fmt.Sprintf("unable to write backup archive of %q to %q: %s", inputDirectory, relativeUri, err.Error())
		} else {
			fmt.Fprintf(stdout, "uploaded backup archive of %q to %q\n", inputDirectory, relativeUri)
		}
	} else {
		errorMessage = fmt.Sprintf("could not open output file: %s", err.Error())
	}

	/* clean up */
	_ = outputFile.Close()
	_ = os.Remove(outputArchivePath)
	_ = os.Remove(outputEncryptedPath)

	/* clean up remote backup prefix */
	if err == nil && cfg.Backup.Hours > 0.0 {
		err = cleanupBackupPrefix(backend, cfg.Backup.Hours, outputPrefixUri, stdout, stderr)
		if err != nil {
			errorMessage = fmt.Sprintf("failed to clean up backup prefix: %s", err.Error())
		}
	}

	if err != nil {
		return fmt.Errorf(errorMessage)
	}

	return nil
}

func parseArgs(args []string, cli_args *cliArgs, stdout, stderr io.Writer) (bool, error) {
	var storeConfig bool = false
	var positionalArgs []string = []string{}

argsLoop:
	for _, arg := range args[1:] {
		if storeConfig {
			cli_args.ConfigFilepath = arg
			storeConfig = false
			continue
		}

		if strings.HasPrefix(arg, "-") {
			switch arg {
			case "--help", "-h":
				fmt.Fprintf(stdout, "%s\n", usageString(args[0]))
				return true, nil
			case "--version", "-V":
				fmt.Fprintf(stdout, "%s v%s (commit hash:%s | date:%s)\n", appname, version, commit, date)
				return true, nil
			case "--verbose", "-v":
				cli_args.Verbose = true
			case "--config", "-c":
				storeConfig = true
			default:
				return true, fmt.Errorf("unrecognize command line option '%s'", arg)
			}
		} else {
			positionalArgs = append(positionalArgs, arg)

			if len(positionalArgs) > 2 {
				break argsLoop
			}
		}
	}

	if storeConfig {
		return true, fmt.Errorf("invalid use of the configuration switch, must provide a value")
	}

	if len(positionalArgs) != 2 {
		fmt.Fprintf(stderr, "%s\n", usageString(args[0]))
		return true, fmt.Errorf("wrong number of arguments, expecting exactly 2 positional arguments")
	} else {
		cli_args.PositionalArgs = positionalArgs
	}

	return false, nil
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

func archiveDirectory(dirPath string, cfg *common.Config) (string, error) {
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
	var index int = 0
	var archiveOutput io.Writer
	if cfg.Internal.Reporter != nil {
		index, _ = cfg.Internal.Reporter.CreateFileTask(-1)
		_ = cfg.Internal.Reporter.DescribeTask(index, "archiving")
		archiveOutput = io.MultiWriter(
			io.Writer(tmp),
			&progressWriter{
				cfg.Internal.Reporter,
				index,
			},
		)
	} else {
		archiveOutput = io.Writer(tmp)
	}
	err = format.Archive(context.Background(), archiveOutput, files)
	if err != nil {
		return "", fmt.Errorf("failed to generate archive: %s", err.Error())
	}
	if index > 0 {
		cfg.Internal.Reporter.FinishTask(index)
	}

	// close the file
	_ = tmp.Close()

	return tmp.Name(), nil
}

func encryptFile(filePath string, recipients []age.Recipient, cfg *common.Config) (string, error) {
	// get input file size
	fileInfo, err := os.Stat(filepath.Clean(filePath))
	if err != nil {
		return "", fmt.Errorf("could not stat input file %s: %s", filePath, err.Error())
	}

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
	var encryptedOutput io.Writer
	if cfg.Internal.Reporter != nil {
		var index int
		index, _ = cfg.Internal.Reporter.CreateFileTask(fileInfo.Size())
		_ = cfg.Internal.Reporter.DescribeTask(index, "encrypting")
		encryptedOutput = io.MultiWriter(
			encryptedWriter,
			&progressWriter{
				cfg.Internal.Reporter,
				index,
			},
		)
	} else {
		encryptedOutput = encryptedWriter
	}
	numWritten, err := io.Copy(encryptedOutput, input)
	if err != nil {
		return "", fmt.Errorf("could not write file '%s' to encrypted file '%s': %s", input.Name(), tmp.Name(), err.Error())
	} else if numWritten == 0 {
		return "", fmt.Errorf("zero bytes written to encrypted archive")
	}
	_ = encryptedWriter.Close()

	return tmp.Name(), nil
}

func cleanupBackupPrefix(backend common.StorageBackend, hours float64, outputPrefixUri *url.URL, stdout, stderr io.Writer) error {
	/* list prefix contents */
	filelist, err := backend.ListFiles(outputPrefixUri)
	if err != nil {
		return fmt.Errorf("could not list remote files: %s", err.Error())
	}

	/* remove old files */
	timeNow := time.Now()
	for _, fileinfo := range filelist {
		diff := timeNow.Sub(fileinfo.Modified())
		fmt.Fprintf(stderr, "file %s, time diff = %.0f h\n", fileinfo.Name(), diff.Hours())
		if diff.Hours() >= hours {
			relativeUri, err := outputPrefixUri.Parse("/" + fileinfo.Name())
			if err == nil {
				fmt.Fprintf(stdout, "removing file %q\n", relativeUri)
				err = backend.RemoveFile(relativeUri)
			}
			if err != nil {
				fmt.Fprintf(stderr, "could not remove remote file %q: %s\n", relativeUri, err.Error())
			}
		}
	}

	return nil
}
