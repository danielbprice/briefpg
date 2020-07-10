/*
 * COPYRIGHT 2020 Brightgate Inc.  All rights reserved.
 *
 * This copyright notice is Copyright Management Information under 17 USC 1202
 * and is included to protect this work and deter copyright infringement.
 * Removal or alteration of this Copyright Management Information without the
 * express written permission of Brightgate Inc is prohibited, and any
 * such unauthorized removal or alteration will be a violation of federal law.
 */

//
// Package briefpg provides an easy way to start and control temporary
// instances of PostgreSQL servers.  The package is designed primarily as an
// aid to writing test cases.  The package does not select or mandate any
// particular PostgreSQL driver, as it invokes Postgres commands to do its
// work.  The package has been designed to have no dependencies outside of
// core go packages.
//
// Concepts are drawn from Eric Radmon's EphemeralPG and Python's
// testing.postgresql.
//
package briefpg

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/xerrors"
)

const (
	// DefaultPgConfTemplate is used by briefpg to configure the transient
	// postgres instance.  This is cribbed from
	// https://github.com/eradman/ephemeralpg
	// It is provided here for reference.
	DefaultPgConfTemplate = `
unix_socket_directories = '{{.TmpDir}}'
listen_addresses = ''
shared_buffers = 12MB
fsync = off
synchronous_commit = off
full_page_writes = off
log_min_duration_statement = 0
log_connections = on
log_disconnections = on
max_worker_processes = 4
`
)

type bpState int

const (
	stateNotPresent bpState = iota
	statePresent
	stateUninitialized
	stateInitialized
	stateServerStarted
	stateDefunct
)

// LogFunc describes a basic printf-style function.
type LogFunc func(string, ...interface{})

// NullLogFunc can be used to suppress output from briefpg.  (It is the
// default).
func NullLogFunc(format string, a ...interface{}) {
}

type cmdMap map[string]string

// BriefPG represents a managed instance of the Postgres database server; the
// instance and all associated data is disposed when Fini() is called.
type BriefPG struct {
	TmpDir         string  // Can be user-set if desired
	Encoding       string  // Defaults to "UNICODE"
	Logf           LogFunc // Verbose output
	DirPrefix      string  // Directory component prefix (e.g. "briefpg.jane.")
	PgVer          string  // Detected postgres version
	PgConfTemplate string  // Postgres Config File template
	state          bpState
	pgCmds         cmdMap
}

var utilities = []string{"psql", "initdb", "pg_ctl", "pg_dump"}

var tryGlobs = []string{
	"/usr/lib/postgresql/*/bin", // Debian
	"/usr/pgsql-*/bin",          // Centos
	"/usr/local/pgsql/bin",
	"/usr/local/pgsql-*/bin",
	"/usr/local/bin", // MacOS Homebrew, and others
}

func wrapExecErr(msg string, cmd *exec.Cmd, err error) error {
	args := strings.Join(cmd.Args, " ")
	if xerr, ok := err.(*exec.ExitError); ok {
		return xerrors.Errorf("%s; command: %s; stderr: %s: %w",
			msg, args, xerr.Stderr, xerr)
	}
	return xerrors.Errorf("%s", msg)
}

// findPostgres will look for a valid Postgres instance in path.  If path is
// "", then it will search the user's $PATH for a valid instance.  If that
// fails, it will search a set of well-known postgres directories.
func findPostgres(path string) (cmdMap, error) {
	var allPaths = make([]string, 0)
	var pgCmds = make(cmdMap)

	if path != "" {
		allPaths = append(allPaths, path)
	} else {
		// Start with the directories in the user's path
		pathSplit := strings.Split(os.Getenv("PATH"), ":")
		allPaths = append(allPaths, pathSplit...)

		// Append the tryGlobs directories if they match anything
		for _, glob := range tryGlobs {
			if paths, err := filepath.Glob(glob); err == nil {
				allPaths = append(allPaths, paths...)
			}
		}
	}

	// For each path element, see if ALL of the commands in utilities
	// (psql, initdb, ...) are present on that path element.
	// If yes, that's the version we'll use.
pathLoop:
	for _, path := range allPaths {
		newCmds := make(map[string]string)
		for _, cName := range utilities {
			p := filepath.Join(path, cName)
			if _, err := os.Stat(p); err != nil {
				continue pathLoop
			}
			newCmds[cName] = p
		}
		pgCmds = newCmds
		break
	}

	if len(pgCmds) == 0 {
		return nil, xerrors.Errorf("couldn't find Postgres; tried %s", strings.Join(allPaths, ":"))
	}
	return pgCmds, nil
}

// PostgresInstalled returns an error if the module is unable to operate due to
// a failure to locate PostgreSQL.
func PostgresInstalled(path string) error {
	_, err := findPostgres(path)
	return err
}

// NewWithOptions returns an instance of BriefPG; if path is "", then attempt
// to automatically locate an instance of Postgres by first scanning the users
// $PATH environment variable.  If that fails, try a series of well-known
// installation locations.
func NewWithOptions(path string, logf LogFunc) (*BriefPG, error) {
	if logf == nil {
		logf = NullLogFunc
	}
	bpg := &BriefPG{
		state:          stateUninitialized,
		Encoding:       "UNICODE",
		Logf:           logf,
		DirPrefix:      "briefpg.",
		pgCmds:         nil,
		PgConfTemplate: DefaultPgConfTemplate,
	}

	// Refine the prefix if we can
	user, err := user.Current()
	if err == nil && user.Username != "" {
		bpg.DirPrefix = fmt.Sprintf("briefpg.%s.", user.Username)
	}

	pgCmds, err := findPostgres(path)
	if err != nil {
		return nil, err
	}
	bpg.pgCmds = pgCmds

	outb, err := exec.Command(pgCmds["psql"], "-V").Output()
	if err != nil {
		return nil, xerrors.Errorf("Failed running psql -V: %w", err)
	}
	out := strings.TrimSpace(string(outb))
	sl := strings.Split(out, " ")
	bpg.PgVer = sl[len(sl)-1]
	return bpg, nil
}

// New returns an instance of BriefPG with default options.  It will
// automatically locate an instance of Postgres by first scanning the user's
// $PATH environment variable.  If that fails, it tries a series of well-known
// installation locations.
func New() (*BriefPG, error) {
	return NewWithOptions("", nil)
}

func (bp *BriefPG) mkTemp() error {
	var err error
	bp.TmpDir, err = ioutil.TempDir("", bp.DirPrefix)
	if err != nil {
		return xerrors.Errorf("Failed to make tmpdir: %w", err)
	}
	return nil
}

func (bp *BriefPG) DbDir() string {
	return filepath.Join(bp.TmpDir, bp.PgVer)
}

func (bp *BriefPG) initDB(ctx context.Context) error {
	if bp.TmpDir == "" {
		if err := bp.mkTemp(); err != nil {
			return err
		}
		bp.state = statePresent
	} else if _, err := os.Stat(bp.TmpDir); err != nil {
		bp.state = stateNotPresent
		return xerrors.Errorf("Tmpdir %s not present or not readable: %w", bp.TmpDir, err)
	}

	if _, err := os.Stat(bp.DbDir()); err != nil {
		cmd := exec.Command(bp.pgCmds["initdb"], "--nosync", "-U", "postgres", "-D", bp.DbDir(), "-E", bp.Encoding, "-A", "trust")
		bp.Logf("briefpg: %s\n", strings.Join(cmd.Args, " "))
		cmdOut, err := cmd.CombinedOutput()
		bp.Logf("briefpg: %s\n", string(cmdOut))
		if err != nil {
			return wrapExecErr("initDB failed", cmd, err)
		}
	}
	confFile := filepath.Join(bp.DbDir(), "postgresql.conf")
	bp.Logf("briefpg: generating %s\n", confFile)
	tmpl, err := template.New("postgresql.conf").Parse(bp.PgConfTemplate)
	if err != nil {
		return xerrors.Errorf("initDB failed to parse postgresql.conf template: %w", err)
	}
	conf, err := os.OpenFile(confFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return xerrors.Errorf("initDB failed to open config: %w", err)
	}
	defer conf.Close()

	bpConf := struct {
		TmpDir string
	}{
		TmpDir: bp.TmpDir,
	}
	err = tmpl.Execute(conf, bpConf)
	if err != nil {
		return xerrors.Errorf("initDB failed to execute template: %w", err)
	}
	bp.state = stateInitialized
	return nil
}

// Start the postgres server, performing necessary initialization along the way
func (bp *BriefPG) Start(ctx context.Context) error {
	var err error
	if bp.state == stateDefunct {
		return xerrors.Errorf("briefpg instance is defunct")
	}

	if bp.state < stateInitialized {
		err = bp.initDB(ctx)
		if err != nil {
			return err
		}
	}

	userOpts := "" // XXX
	postgresOpts := fmt.Sprintf("-c listen_addresses='' %s", userOpts)
	logFile := filepath.Join(bp.DbDir(), "postgres.log")
	cmd := exec.Command(bp.pgCmds["pg_ctl"], "-w", "-o", postgresOpts, "-s", "-D", bp.DbDir(), "-l", logFile, "start")
	bp.Logf("briefpg: %s\n", strings.Join(cmd.Args, " "))
	cmdOut, err := cmd.CombinedOutput()
	bp.Logf("briefpg: %s\n", string(cmdOut))
	if err != nil {
		return wrapExecErr("Start failed", cmd, err)
	}
	bp.state = stateServerStarted
	return nil
}

// CreateDB is a convenience function to create a named database; you can do this
// using your database driver instead, at lower cost.  This routine uses 'psql' to
// do the job.  The primary use case is to rapidly set up an empty database for
// test purposes.  The URI to access the database is returned.
func (bp *BriefPG) CreateDB(ctx context.Context, dbName, createArgs string) (string, error) {
	if bp.state < stateServerStarted {
		return "", xerrors.Errorf("Server not started; cannot create database")
	}
	scmd := fmt.Sprintf("CREATE DATABASE \"%s\" %s", dbName, createArgs)
	cmd := exec.Command(bp.pgCmds["psql"], "-c", scmd, bp.DBUri("postgres"))
	bp.Logf("briefpg: %s\n", strings.Join(cmd.Args, " "))
	cmdOut, err := cmd.CombinedOutput()
	for _, line := range strings.Split(strings.TrimSpace(string(cmdOut)), "\n") {
		bp.Logf("briefpg: %s\n", line)
	}
	if err != nil {
		return "", wrapExecErr("CreateDB failed", cmd, err)
	}
	return bp.DBUri(dbName), nil
}

// DumpDB writes the named database contents to w using pg_dump.  In a test
// case, this can be used to dump the database in the event of a failure.
func (bp *BriefPG) DumpDB(ctx context.Context, dbName string, w io.Writer) error {
	if bp.state < stateServerStarted {
		return xerrors.Errorf("Server not started; cannot dump database")
	}
	cmd := exec.Command(bp.pgCmds["pg_dump"], bp.DBUri(dbName))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	bp.Logf("briefpg: starting dump: %s\n", strings.Join(cmd.Args, " "))
	err = cmd.Start()
	if err != nil {
		return err
	}
	_, err = io.Copy(w, stdout)
	if err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return wrapExecErr("DumpDB failed", cmd, err)
	}
	return nil
}

// DBUri returns the connection URI for a named database
func (bp *BriefPG) DBUri(dbName string) string {
	return fmt.Sprintf("postgresql:///%s?host=%s&user=postgres", dbName, bp.TmpDir)
}

// Fini stops the database server, if running, and cleans it up
func (bp *BriefPG) Fini(ctx context.Context) error {
	if bp.state >= stateServerStarted {
		cmd := exec.Command(bp.pgCmds["pg_ctl"], "-m", "immediate", "-w", "-D", bp.DbDir(), "stop")
		bp.Logf("briefpg: %s\n", strings.Join(cmd.Args, " "))
		cmdOut, err := cmd.CombinedOutput()
		bp.Logf("briefpg: %s\n", string(cmdOut))
		if err != nil {
			return wrapExecErr("Fini failed", cmd, err)
		}
	}

	if bp.state >= statePresent {
		bp.Logf("briefpg: cleaning up %s\n", bp.TmpDir)
		os.RemoveAll(bp.TmpDir)
	}

	bp.state = stateDefunct
	return nil
}

// MustFini stops the database server, if running, and cleans it up; this
// routine wraps Fini() and will panic if an error is raised.
func (bp *BriefPG) MustFini(ctx context.Context) {
	if err := bp.Fini(ctx); err != nil {
		panic(fmt.Sprintf("MustFini: %v", err))
	}
}
