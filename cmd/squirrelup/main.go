package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"filippo.io/age"
	"github.com/breezerider/squirrel-up/pkg/common"
	"github.com/mholt/archiver/v4"
)

const (
	appname = "squirrelup"

	usage = `Usage: %s backup_dir output_uri
   create an (optionally) encrypted gzip-compressed TAR file and upload it to storage backend.
   At the moment only BackBlaze B2 clous storage is implemented.
`

	defaultConfigFilepath = appname + ".yml"
)

var (
	version string
	commit  string
	date    string
)

// return the usage string.
func usageString() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, usage, appname)
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
	if len(args) >= 2 {
		if slices.ContainsFunc(args, func(arg string) bool { return (arg == "--help") || (arg == "-h") }) {
			fmt.Fprintf(stdout, "%s\n", usageString())
			return nil
		}
		if slices.ContainsFunc(args, func(arg string) bool { return (arg == "--version") || (arg == "-V") }) {
			fmt.Printf("%s v%s %s (%s)\n", appname, version, commit, date)
			return nil
		}
	}

	if len(args) != 3 {
		fmt.Fprintf(stdout, "%s\n", usageString())
		return fmt.Errorf("wrong number of arguments, expecting exactly 2 arguments")
	}

	// process first input argument
	var inputDirectory string = args[1]
	if isDir, err := isDirectory(inputDirectory); !isDir {
		if err != nil {
			return fmt.Errorf("first argument must be a valid directory path: %s", err.Error())
		} else {
			return fmt.Errorf("first argument must be a valid directory path")
		}
	}

	// process second input argument
	outputUri, err := url.ParseRequestURI(args[2])
	if err != nil {
		return fmt.Errorf("could not parse output URI: %s", err.Error())
	}

	/* load configuration */
	var cfg common.Config

	err = initConfig(&cfg)
	if err != nil {
		return fmt.Errorf("%s", err.Error())
	}

	/* initialize the backend */
	backend, err := common.CreateStorageBackend(outputUri, &cfg)
	if err != nil {
		return fmt.Errorf("failed to create backend: %s", err.Error())
	}

	fileinfo, err := backend.GetFileInfo(outputUri)
	if err != nil {
		if err.Error() == common.ErrFileNotFound {
			log.Printf("File %q not found\n", outputUri)
		} else {
			return fmt.Errorf("backend operation failed: %s", err.Error())
		}
	} else {
		log.Printf("File info: %+v\n", *fileinfo)
	}

	/* initialize encryption */
	var recipients []age.Recipient
	recipients, err = initEncryption(&cfg)
	if err != nil {
		return fmt.Errorf("%s", err.Error())
	}

	/* create an archive from the input directory */
	var outputArchivePath string
	outputArchivePath, err = archiveDirectory(inputDirectory)
	if err != nil {
		return fmt.Errorf("%s", err.Error())
	}

	/* encrypt the output file */
	var outputEncryptedPath string
	if len(recipients) > 0 {
		log.Printf("recipients: %+v\n", recipients)
		outputEncryptedPath, err = encryptFile(outputArchivePath, recipients)
		if err != nil {
			return fmt.Errorf("%s", err.Error())
		}
	} else {
		log.Printf("no pubkey found, encryption disabled\n")
		outputEncryptedPath = outputArchivePath
	}

	/* store output file */
	outputFile, err := os.Open(filepath.Clean(outputEncryptedPath))
	if err == nil {
		err = backend.StoreFile(io.ReadSeekCloser(outputFile), outputUri)
		if err != nil {
			// Print the error and exit.
			log.Fatalf("Unable to write %q to %q, %v", inputDirectory, outputUri, err)
		} else {
			// Print the error and exit.
			log.Printf("Wrote %q to %q", inputDirectory, outputUri)
		}
	} else {
		return fmt.Errorf("could not open output file: %s", err.Error())
	}
	_ = outputFile.Close()
	_ = os.Remove(outputArchivePath)
	_ = os.Remove(outputEncryptedPath)

	return nil
}

func initConfig(cfg *common.Config) error {
	configFile, err := os.Open(defaultConfigFilepath)
	if err == nil {
		err = common.LoadConfigFromFile(cfg, configFile)
	}
	_ = configFile.Close()

	if err != nil && err != io.EOF {
		if os.IsNotExist(err) {
			log.Printf("configuration file %s does not exists", defaultConfigFilepath)
		} else {
			return fmt.Errorf("could not load configuration from %s: %s", defaultConfigFilepath, err.Error())
		}
	}
	err = common.LoadConfigFromEnv(cfg)
	if err != nil {
		return fmt.Errorf("could not load configuration from environment: %s", err.Error())
	}

	return nil
}

func initEncryption(cfg *common.Config) ([]age.Recipient, error) {
	var recipients []age.Recipient

	if len(cfg.Encryption.Pubkey) > 0 {
		if r, err := age.ParseX25519Recipient(cfg.Encryption.Pubkey); err == nil {
			recipients = append(recipients, r)
		} else {
			log.Printf("pubkey parsing failed, assuming it is path to file")

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

func encryptFile(filePath string, recipients []age.Recipient) (string, error) {
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
	} else {
		log.Printf("%d bytes written\n", numWritten)
	}
	_ = encryptedWriter.Close()

	return tmp.Name(), nil
}
