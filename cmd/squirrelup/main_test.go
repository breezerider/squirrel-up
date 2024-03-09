package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/breezerider/squirrel-up/pkg/common"
)

const expected_usage string = `Usage: SquirrelUp <backup_dir> <output_prefix_uri>
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

Default configuration is stored under .

`

func assertEquals(t *testing.T, expected any, actual any, description string) {
	if actual != expected {
		t.Fatalf("assertion %s failed:\nexpected: %+v\nactual: %+v\n", description, expected, actual)
	}
}

/* test cases for main */
func TestMainVersion(t *testing.T) {
	fmt.Println("Running TestMainVersion...")
	args := []string{appname, "--version"}
	var stdout, stderr bytes.Buffer
	var expected_stdout = `SquirrelUp v0.0.0 (commit hash:abcdef | date:1970-01-01 00:00:00 +0000 UTC)
`

	version = "0.0.0"
	commit = "abcdef"
	date = time.Unix(0, 0).UTC().String()

	err := run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err != nil {
		t.Fatalf(err.Error())
	}
	assertEquals(t, expected_stdout, stdout.String(), "TestMainVersion.stdout")
	assertEquals(t, 0, len(stderr.String()), "TestMainVersion.stderr")

	// clean up
	version = ""
	commit = ""
	date = ""
}

func TestMainHelp(t *testing.T) {
	fmt.Println("Running TestMainHelp...")
	args := []string{appname, "--help"}
	var stdout, stderr bytes.Buffer
	var expected_stdout = expected_usage

	err := run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err != nil {
		t.Fatalf(err.Error())
	}

	assertEquals(t, expected_stdout, stdout.String(), "TestMainHelp.stdout")
	assertEquals(t, 0, len(stderr.String()), "TestMainHelp.stderr")
}

func TestMainWrongCliArgs(t *testing.T) {
	fmt.Println("Running TestMainWrongCliArgs...")
	args := []string{appname, "."}
	var stdout, stderr bytes.Buffer
	var expected_stderr = expected_usage

	/* test with too few positional arguments */
	err := run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err == nil {
		t.Fatalf("%s was supposed to fail", appname)
	}
	assertEquals(t, "wrong number of arguments, expecting exactly 2 positional arguments", err.Error(), "TestMainWrongCliArgs.Error")
	assertEquals(t, expected_stderr, stderr.String(), "TestMainWrongCliArgs.stderr")
	assertEquals(t, 0, len(stdout.String()), "TestMainWrongCliArgs.stdout")

	// clean up
	stderr.Reset()

	/* test with too many positional arguments */
	args = []string{appname, ".", "dummy://path/", "another-arg"}

	err = run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err == nil {
		t.Fatalf("%s was supposed to fail\n", appname)
	}
	assertEquals(t, "wrong number of arguments, expecting exactly 2 positional arguments", err.Error(), "TestMainWrongCliArgs.Error")
	assertEquals(t, expected_stderr, stderr.String(), "TestMainWrongCliArgs.stderr")
	assertEquals(t, 0, len(stdout.String()), "TestMainWrongCliArgs.stdout")

	// clean up
	stderr.Reset()

	/* test configuration switch with no arguments */
	args = []string{appname, ".", "dummy://path/", "-c"}

	err = run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err == nil {
		t.Fatalf("%s was supposed to fail\n", appname)
	}
	assertEquals(t, "invalid use of the configuration switch, must provide a value", err.Error(), "TestMainWrongCliArgs.Error")
	assertEquals(t, 0, len(stderr.String()), "TestMainWrongCliArgs.stderr")
	assertEquals(t, 0, len(stdout.String()), "TestMainWrongCliArgs.stdout")

	/* test an unknown switch with no arguments */
	args = []string{appname, ".", "dummy://path/", "-u"}

	err = run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err == nil {
		t.Fatalf("%s was supposed to fail\n", appname)
	}
	assertEquals(t, "unrecognize command line option '-u'", err.Error(), "TestMainWrongCliArgs.Error")
	assertEquals(t, 0, len(stderr.String()), "TestMainWrongCliArgs.stderr")
	assertEquals(t, 0, len(stdout.String()), "TestMainWrongCliArgs.stdout")

	/* test an unknown switch with no arguments */
	args = []string{appname, ".", "dummy://path/", "--unknown"}

	err = run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err == nil {
		t.Fatalf("%s was supposed to fail\n", appname)
	}
	assertEquals(t, "unrecognize command line option '--unknown'", err.Error(), "TestMainWrongCliArgs.Error")
	assertEquals(t, 0, len(stderr.String()), "TestMainWrongCliArgs.stderr")
	assertEquals(t, 0, len(stdout.String()), "TestMainWrongCliArgs.stdout")
}

func TestMainInvalidDir(t *testing.T) {
	fmt.Println("Running TestMainInvalidDir...")

	var stdout, stderr bytes.Buffer

	/* test with a file instead of a directory */
	tmp, err := os.CreateTemp("", appname+"-testing-")
	if err != nil {
		t.Fatalf("could not create temporary file: %s", err.Error())
	}
	defer os.Remove(tmp.Name())

	// run main
	args := []string{appname, tmp.Name(), "dummy://path/"}

	err = run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err == nil {
		t.Fatalf("%s was supposed to fail\n", appname)
	}
	assertEquals(t, "first argument must be a valid directory path", err.Error(), "TestMainInvalidDir.Error")
	assertEquals(t, 0, len(stdout.String()), "TestMainInvalidDir.stdout")
	assertEquals(t, 0, len(stderr.String()), "TestMainInvalidDir.stderr")

	/* test with an inaccessible directory */
	tmpDir, err := os.MkdirTemp("", appname+"-testing-")
	if err != nil {
		t.Fatalf("could not create temporary directory: %s", err.Error())
	}
	defer os.Remove(tmpDir)

	err = os.Chmod(tmpDir, 0000)
	if err != nil {
		t.Fatalf("could not set directory permissions: %s", err.Error())
	}

	// run main
	args = []string{appname, tmpDir, "dummy://path/"}

	err = run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	_ = os.Chmod(tmpDir, 0777)
	if err == nil {
		t.Fatalf("%s was supposed to fail 2\n", appname)
	}
	assertEquals(t,
		fmt.Sprintf("could not initialize archive files structure: open %s: permission denied", tmpDir),
		err.Error(), "TestMainInvalidDir.Error")
	assertEquals(t, `file info: {name:path/ size:0 modified:{wall:0 ext:62135596800 loc:<nil>} isfile:false}
`, stdout.String(), "TestMainInvalidURI.stdout")
	assertEquals(t, `default configuration path is empty
`, stderr.String(), "TestMainInvalidURI.stderr")
}

func TestMainInvalidURI(t *testing.T) {
	fmt.Println("Running TestMainInvalidURI...")

	// invalid URI with known backend
	args := []string{appname, ".", "dummy"}
	var stdout, stderr bytes.Buffer

	err := run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err == nil {
		t.Fatalf("%s was supposed to fail", appname)
	}
	assertEquals(t, `could not parse output URI: parse "dummy": invalid URI for request`, err.Error(), "TestMainInvalidURI.Error")
	assertEquals(t, 0, len(stdout.String()), "TestMainInvalidURI.stdout")
	assertEquals(t, 0, len(stderr.String()), "TestMainInvalidURI.stderr")

	// valid URI with unknown backend
	args = []string{appname, ".", "s3://bucket/path/to/key"}

	err = run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err == nil {
		t.Fatalf("%s was supposed to fail\n", appname)
	}
	assertEquals(t, `failed to create backend: unknown URL scheme s3`, err.Error(), "TestMainInvalidURI.Error")
	assertEquals(t, 0, len(stdout.String()), "TestMainInvalidURI.stdout")
	assertEquals(t, `default configuration path is empty
`, stderr.String(), "TestMainInvalidURI.stderr")
}

func TestMainInvalidEncryptionConfig(t *testing.T) {
	fmt.Println("Running TestMainInvalidEncryptionConfig...")

	var stdout, stderr bytes.Buffer
	var cfg common.Config

	// invalid pubkey file: non exisiting file
	cfg.Encryption.Pubkey = "./non-existing"
	if recepients, err := initEncryption(&cfg, io.Writer(&stdout), io.Writer(&stderr)); err != nil {
		assertEquals(t, `could not open pubkey file: open ./non-existing: no such file or directory`, err.Error(), "TestMainInvalidEncryptionConfig.Error")
		assertEquals(t, 0, len(recepients), "TestMainInvalidEncryptionConfig.recepients")
		assertEquals(t, 0, len(stdout.String()), "TestMainInvalidEncryptionConfig.stdout")
		assertEquals(t, `pubkey parsing failed, assuming it is path to file
`, stderr.String(), "TestMainInvalidEncryptionConfig.stderr")

	} else {
		t.Fatalf("initEncryption was supposed to fail, return values: %v, %v\n", recepients, err)
	}

	// clean up
	stderr.Reset()

	// invalid pubkey file: /dev/null
	cfg.Encryption.Pubkey = "/dev/null"
	if recepients, err := initEncryption(&cfg, io.Writer(&stdout), io.Writer(&stderr)); err != nil {
		assertEquals(t, `parsing pubkey file failed: no recipients found`, err.Error(), "TestMainInvalidEncryptionConfig.Error")
		assertEquals(t, 0, len(recepients), "TestMainInvalidEncryptionConfig.recepients")
		assertEquals(t, 0, len(stdout.String()), "TestMainInvalidEncryptionConfig.stdout")
		assertEquals(t, `pubkey parsing failed, assuming it is path to file
`, stderr.String(), "TestMainInvalidEncryptionConfig.stderr")

	} else {
		t.Fatalf("initEncryption was supposed to fail, return values: %v, %v\n", recepients, err)
	}
}

func TestMainInvalidConfig(t *testing.T) {
	fmt.Println("Running TestMainInvalidConfig...")

	var stdout, stderr bytes.Buffer
	var cfg common.Config

	// invalid config file: non exisiting file
	if err := initConfig(&cfg, "./non-existing", io.Writer(&stdout), io.Writer(&stderr)); err == nil {
		assertEquals(t, 0, len(stdout.String()), "TestMainInvalidConfig.stdout")
		assertEquals(t, `configuration file ./non-existing does not exist
`, stderr.String(), "TestMainInvalidConfig.stderr")

	} else {
		t.Fatalf(err.Error())
	}

	// clean up
	stderr.Reset()

	// invalid config file: /dev/null
	if err := initConfig(&cfg, "/dev/null", io.Writer(&stdout), io.Writer(&stderr)); err != nil {
		assertEquals(t, `could not load configuration from /dev/null: LoadConfigFromFile failed: EOF`, err.Error(), "TestMainInvalidConfig.Error")
		assertEquals(t, 0, len(stdout.String()), "TestMainInvalidConfig.stdout")
		assertEquals(t, `loading configuration from /dev/null
`, stderr.String(), "TestMainInvalidConfig.stderr")

	} else {
		t.Fatalf("initConfig was supposed to fail\n")
	}

	// clean up
	stderr.Reset()

	// invalid config: invalid env value
	os.Setenv("SQUIRRELUP_BACKUP_HOURS", "invalid")
	if err := initConfig(&cfg, "", io.Writer(&stdout), io.Writer(&stderr)); err != nil {
		assertEquals(t, `could not load configuration from environment: LoadConfigFromEnv failed: Backup: Hours("invalid"): strconv.ParseFloat: parsing "invalid": invalid syntax`, err.Error(), "TestMainInvalidConfig.Error")
		assertEquals(t, 0, len(stdout.String()), "TestMainInvalidConfig.stdout")
		assertEquals(t, `default configuration path is empty
`, stderr.String(), "TestMainInvalidConfig.stderr")

	} else {
		t.Fatalf("initConfig was supposed to fail\n")
	}

	// clean up
	os.Setenv("SQUIRRELUP_BACKUP_HOURS", "")

	// fmt.Println("stdout")
	// fmt.Println(stdout.String())
	// fmt.Println("stderr")
	// fmt.Println(stderr.String())
	// t.Fatalf("TestMainInvalidConfigFile\n")
}

func TestMainRun(t *testing.T) {
	const pubkey = "age1xmwwc06ly3ee5rytxm9mflaz2u56jjj36s0mypdrwsvlul66mv4q47ryef"
	var yaml string = `encryption:
  pubkey: "%s"

`
	defaultConfigFilepath = ""

	fmt.Println("Running TestMainRun...")
	args := []string{appname, ".", "dummy://path/to/dir/"}
	var stdout, stderr bytes.Buffer
	_ = common.GenerateDummyFiles("to/dir/", 2)

	// without encryption
	os.Setenv("SQUIRRELUP_PUBKEY", "")
	err := run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err != nil {
		t.Fatalf(err.Error())
	} else {
		diff := time.Now().Sub(time.Unix(0, 0))

		assertEquals(t, fmt.Sprintf(`file info: {name:path/to/dir/ size:0 modified:{wall:0 ext:62135596800 loc:<nil>} isfile:false}
wrote "." to "dummy://path/to/dir/%s.tar.gz"
removing file "dummy://path/to/dir/A"
removing file "dummy://path/to/dir/B"
`, time.Now().Format("2006-01-02T15-0700")), stdout.String(), "TestMainRun.stdout")
		assertEquals(t, fmt.Sprintf(`default configuration path is empty
no pubkey found, encryption disabled
file to/dir/A, time diff = %.0f h
file to/dir/B, time diff = %.0f h
`, diff.Hours(), diff.Hours()), stderr.String(), "TestMainRun.stderr")
	}

	// clean up
	stdout.Reset()
	stderr.Reset()

	// with pubkey from env
	os.Setenv("SQUIRRELUP_PUBKEY", pubkey)
	err = run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err != nil {
		t.Fatalf(err.Error())
	} else {
		diff := time.Now().Sub(time.Unix(0, 0))

		assertEquals(t, fmt.Sprintf(`file info: {name:path/to/dir/ size:0 modified:{wall:0 ext:62135596800 loc:<nil>} isfile:false}
wrote "." to "dummy://path/to/dir/%s.tar.gz.age"
removing file "dummy://path/to/dir/A"
removing file "dummy://path/to/dir/B"
`, time.Now().Format("2006-01-02T15-0700")), stdout.String(), "TestMainRun.stdout")
		assertEquals(t, fmt.Sprintf(`default configuration path is empty
recipients: [age1xmwwc06ly3ee5rytxm9mflaz2u56jjj36s0mypdrwsvlul66mv4q47ryef]
file to/dir/A, time diff = %.0f h
file to/dir/B, time diff = %.0f h
`, diff.Hours(), diff.Hours()), stderr.String(), "TestMainRun.stderr")
	}

	// clean up
	stdout.Reset()
	stderr.Reset()

	// with pubkey from env (file path)
	tmpKey, err := os.CreateTemp("", appname+"-testing-")
	if err != nil {
		t.Fatalf("could not create temporary file: %s", err.Error())
	}
	defer os.Remove(tmpKey.Name())

	if err = os.WriteFile(tmpKey.Name(), []byte(pubkey), 0666); err != nil {
		t.Fatalf("could not write to temporary file: %s", err.Error())
	}

	os.Setenv("SQUIRRELUP_PUBKEY", tmpKey.Name())
	err = run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err != nil {
		t.Fatalf(err.Error())
	} else {
		diff := time.Now().Sub(time.Unix(0, 0))

		assertEquals(t, fmt.Sprintf(`file info: {name:path/to/dir/ size:0 modified:{wall:0 ext:62135596800 loc:<nil>} isfile:false}
wrote "." to "dummy://path/to/dir/%s.tar.gz.age"
removing file "dummy://path/to/dir/A"
removing file "dummy://path/to/dir/B"
`, time.Now().Format("2006-01-02T15-0700")), stdout.String(), "TestMainRun.stdout")
		assertEquals(t, fmt.Sprintf(`default configuration path is empty
pubkey parsing failed, assuming it is path to file
recipients: [age1xmwwc06ly3ee5rytxm9mflaz2u56jjj36s0mypdrwsvlul66mv4q47ryef]
file to/dir/A, time diff = %.0f h
file to/dir/B, time diff = %.0f h
`, diff.Hours(), diff.Hours()), stderr.String(), "TestMainRun.stderr")
	}

	// clean up
	os.Setenv("SQUIRRELUP_PUBKEY", "")
	stdout.Reset()
	stderr.Reset()

	// with pubkey from config file
	tmpCfg, err := os.CreateTemp("", appname+"-testing-")
	if err != nil {
		t.Fatalf("could not create temporary file: %s", err.Error())
	}
	defer os.Remove(tmpCfg.Name())

	if err = os.WriteFile(tmpCfg.Name(), []byte(fmt.Sprintf(yaml, pubkey)), 0600); err != nil {
		t.Fatalf("could not write to temporary file: %s", err.Error())
	}

	args = []string{appname, ".", "dummy://path/to/dir/", "--config", tmpCfg.Name()}

	err = run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err != nil {
		t.Fatalf(err.Error())
	} else {
		diff := time.Now().Sub(time.Unix(0, 0))

		assertEquals(t, fmt.Sprintf(`file info: {name:path/to/dir/ size:0 modified:{wall:0 ext:62135596800 loc:<nil>} isfile:false}
wrote "." to "dummy://path/to/dir/%s.tar.gz.age"
removing file "dummy://path/to/dir/A"
removing file "dummy://path/to/dir/B"
`, time.Now().Format("2006-01-02T15-0700")), stdout.String(), "TestMainRun.stdout")
		assertEquals(t, fmt.Sprintf(`loading configuration from %s
recipients: [age1xmwwc06ly3ee5rytxm9mflaz2u56jjj36s0mypdrwsvlul66mv4q47ryef]
file to/dir/A, time diff = %.0f h
file to/dir/B, time diff = %.0f h
`, tmpCfg.Name(), diff.Hours(), diff.Hours()), stderr.String(), "TestMainRun.stderr")
	}

	// clean up
	stdout.Reset()
	stderr.Reset()

	// with pubkey from config file (file path)
	if err = os.WriteFile(tmpCfg.Name(), []byte(fmt.Sprintf(yaml, tmpKey.Name())), 0600); err != nil {
		t.Fatalf("could not write to temporary file: %s", err.Error())
	}

	args = []string{appname, ".", "--config", tmpCfg.Name(), "dummy://path/to/dir/"}

	err = run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err != nil {
		t.Fatalf(err.Error())
	} else {
		diff := time.Now().Sub(time.Unix(0, 0))

		assertEquals(t, fmt.Sprintf(`file info: {name:path/to/dir/ size:0 modified:{wall:0 ext:62135596800 loc:<nil>} isfile:false}
wrote "." to "dummy://path/to/dir/%s.tar.gz.age"
removing file "dummy://path/to/dir/A"
removing file "dummy://path/to/dir/B"
`, time.Now().Format("2006-01-02T15-0700")), stdout.String(), "TestMainRun.stdout")
		assertEquals(t, fmt.Sprintf(`loading configuration from %s
pubkey parsing failed, assuming it is path to file
recipients: [age1xmwwc06ly3ee5rytxm9mflaz2u56jjj36s0mypdrwsvlul66mv4q47ryef]
file to/dir/A, time diff = %.0f h
file to/dir/B, time diff = %.0f h
`, tmpCfg.Name(), diff.Hours(), diff.Hours()), stderr.String(), "TestMainRun.stderr")
	}

	// clean up
	_ = common.GenerateDummyFiles("", 0)
}
