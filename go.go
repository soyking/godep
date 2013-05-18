package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const spool = "/var/tmp/godep"

var cmdGo = &Command{
	Usage: "go command [args...]",
	Short: "run the go tool in a sandbox",
	Long: `
Go runs the go tool in a temporary GOPATH sandbox
with the dependencies listed in file Godeps.
`,
	Run: runGo,
}

// Set up a sandbox and run the go tool. The sandbox is built
// out of specific checked-out revisions of repos. We keep repos
// and revs materialized on disk under the assumption that disk
// space is cheap and plentiful, and writing files is slow.
// Everything is kept in the spool directory.
func runGo(cmd *Command, args []string) {
	g, err := ReadGodeps("Godeps")
	if err != nil {
		log.Fatalln(err)
	}
	gopath, err := sandboxAll(g.Deps)
	if err != nil {
		log.Fatalln(err)
	}

	// make empty dir for first entry in gopath
	target := filepath.Join(spool, "target", rands(10))
	defer os.RemoveAll(target)
	err = os.MkdirAll(target, 0777)
	if err != nil {
		log.Fatalln(err)
	}

	gopath = target + ":" + gopath
	c := exec.Command("go", args...)
	c.Env = []string{"GOPATH=" + gopath + ":" + os.Getenv("GOPATH")}
	c.Env = append(c.Env, os.Environ()...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err = c.Run()
	if err != nil {
		log.Fatalln("go", err)
	}

	// copy binaries out
}

// sandboxAll ensures that the commits in deps are available
// on disk, and returns a GOPATH string that will cause them
// to be used.
func sandboxAll(a []Dependency) (gopath string, err error) {
	var path []string
	for _, dep := range a {
		dir, err := sandbox(dep)
		if err != nil {
			return "", err
		}
		path = append(path, dir)
	}
	return strings.Join(path, ":"), nil
}

// sandbox ensures that the commit in d is available on disk,
// and returns a GOPATH string that will cause it to be used.
func sandbox(d Dependency) (gopath string, err error) {
	if !exists(d.RepoPath()) {
		err = d.createRepo()
		if err != nil {
			return "", fmt.Errorf("can't clone %s: %s", d.Remote(), err)
		}
	}
	err = d.fetch()
	if err != nil {
		return "", err
	}
	err = vcsCheckout(d.WorkdirRoot(), d.Rev, d.RepoPath())
	if err != nil {
		return "", fmt.Errorf("checkout %s rev %s: %s", d.ImportPath, d.Rev, err)
	}
	return d.Gopath(), nil
}

func rands(n int) string {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		log.Fatal("rands", err)
	}
	return hex.EncodeToString(b)[:n]
}
