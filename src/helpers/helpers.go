// Contains few helpers functions which are used througout the project
package helpers

import (
	"errors"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// Returns the root directory. This is the place where the app is installed
// or the place where the source is stored if in development or installed
// with go get
func ProjectRoot() (string, error) {

	// first trying the gopath
	gopath := os.ExpandEnv("$GOPATH")
	relPath := filepath.FromSlash("src/github.com/ironsmile/httpms")
	for _, path := range strings.Split(gopath, ":") {
		tmplPath := filepath.Join(path, relPath)
		entry, err := os.Stat(tmplPath)
		if err != nil {
			continue
		}

		if entry.IsDir() {
			return tmplPath, nil
		}
	}

	// now we try the directory of the binary
	if len(os.Args) < 1 {
		// highly unlikely but still!
		return "", errors.New("os.Args is empty. " +
			"Cannot find the project root directory.")
	}

	abs, err := filepath.Abs(filepath.Dir(os.Args[0]))

	if err != nil {
		return "", err
	}

	return abs, nil
}

// Sets the logfile of the server
func SetLogsFile(logFilePath string) {
	logFile, err := os.Create(logFilePath)
	if err != nil {
		log.Println("Could not open logfile")
		os.Exit(1)
	}
	log.SetOutput(logFile)
}

// Copies a file from src to dst
func Copy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	cerr := out.Close()
	if err != nil {
		return err
	}
	return cerr
}

// Returns absolute path. If path is already absolute leave it be. If not join it with
// relativeRoot
func AbsolutePath(path, relativeRoot string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(relativeRoot, path)
}

// Returns the directory in which user files should be stored. Creates it is missing.
// User files are thing such as sqlite files, logfiles and user configs
func ProjectUserPath() (string, error) {
	user, err := user.Current()

	if err != nil {
		return "", err
	}

	path := filepath.Join(user.HomeDir, HttpmsDir)

	_, err = os.Stat(path)

	if err == nil {
		return path, nil
	}

	err = os.MkdirAll(path, os.ModeDir|0750)

	if err != nil {
		return "", err
	}

	return path, nil
}