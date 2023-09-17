package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"
)

func assertEquals(t *testing.T, expected any, actual any, description string) {
	if actual != expected {
		t.Fatalf("assertion %s failed:\nexpected: %+v\nactual: %+v\n", description, expected, actual)
	}
}

/* test cases for main */
func TestMainVersion(t *testing.T) {
	fmt.Println("Running main...")
	args := []string{appname, "--version"}
	var stdout, stderr bytes.Buffer

	err := run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err != nil {
		t.Fatalf(err.Error())
	}
}

func TestMainHelp(t *testing.T) {
	fmt.Println("Running main...")
	args := []string{appname, "--help"}
	var stdout, stderr bytes.Buffer

	err := run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err != nil {
		t.Fatalf(err.Error())
	}
}

func TestMainWrongNumArgs(t *testing.T) {
	fmt.Println("Running main...")
	args := []string{appname, "."}
	var stdout, stderr bytes.Buffer

	err := run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err == nil {
		t.Fatalf("%s was supposed to fail", appname)
	}
	assertEquals(t, "wrong number of arguments, expecting exactly 2 arguments", err.Error(), "err.Error")

	args = []string{appname, ".", "dummy://path/", "another-arg"}

	err = run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err == nil {
		t.Fatalf("%s was supposed to fail", appname)
	}
	assertEquals(t, "wrong number of arguments, expecting exactly 2 arguments", err.Error(), "err.Error")
}

func TestMainNotADir(t *testing.T) {
	fmt.Println("Running main...")
	// create a temporary file
	tmp, err := os.CreateTemp("", appname+"-testing-")
	if err != nil {
		t.Fatalf("could not create temporary file: %s", err.Error())
	}
	defer os.Remove(tmp.Name())

	args := []string{appname, tmp.Name(), "dummy://path/"}
	var stdout, stderr bytes.Buffer

	err = run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err == nil {
		t.Fatalf("%s was supposed to fail", appname)
	}
	assertEquals(t, "first argument must be a valid directory path", err.Error(), "err.Error")
}

func TestMainInvalidURI(t *testing.T) {
	fmt.Println("Running main...")
	args := []string{appname, ".", "dummy"}
	var stdout, stderr bytes.Buffer

	err := run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err == nil {
		t.Fatalf("%s was supposed to fail", appname)
	}
	assertEquals(t, "could not parse output URI: parse \"dummy\": invalid URI for request", err.Error(), "err.Error")
}

func TestMainRun(t *testing.T) {
	const pubkey = "age1xmwwc06ly3ee5rytxm9mflaz2u56jjj36s0mypdrwsvlul66mv4q47ryef"
	fmt.Println("Running main...")
	args := []string{appname, ".", "dummy://path/"}
	var stdout, stderr bytes.Buffer

	// without encryption
	os.Setenv("SQUIRRELUP_PUBKEY", "")
	err := run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err != nil {
		t.Fatalf(err.Error())
	}

	// with pubkey from env
	os.Setenv("SQUIRRELUP_PUBKEY", pubkey)
	err = run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err != nil {
		t.Fatalf(err.Error())
	}

	// with pubkey from file
	tmp, err := os.CreateTemp("", appname+"-testing-")
	if err != nil {
		t.Fatalf("could not create temporary file: %s", err.Error())
	}
	defer os.Remove(tmp.Name())
	if err = os.WriteFile(tmp.Name(), []byte(pubkey), 0666); err != nil {
		t.Fatalf("could not write to temporary file: %s", err.Error())
	}

	os.Setenv("SQUIRRELUP_PUBKEY", tmp.Name())
	err = run(args, nil, io.Writer(&stdout), io.Writer(&stderr))
	if err != nil {
		t.Fatalf(err.Error())
	}
}
