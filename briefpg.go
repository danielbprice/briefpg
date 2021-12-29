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

// LogFunction describes a basic printf-style function.
type LogFunction func(string, ...interface{})

// NullLogFunction can be used to suppress output from briefpg.  (It is the
// default).
func NullLogFunction(format string, a ...interface{}) {
}

type cmdMap map[string]string

// BriefPG represents a managed instance of the Postgres database server; the
// instance and all associated data is disposed when Fini() is called.
type BriefPG struct {
	tmpDir         string      // Set with OptTmpDir
	madeTmpDir     bool        // Set when the TmpDir was created automatically
	encoding       string      // Defaults to "UNICODE", set with OptPostgresEncoding
	pgConfTemplate string      // Postgres Config File template, set with OptPgConfTemplate
	logf           LogFunction // Verbose output, set with OptLogFunc
	state          bpState
	pgCmds         cmdMap
	pgVer          string // Detected Postgres version corresponding to pgCmds
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
		return fmt.Errorf("%s; command: %s; stderr: %s: %w",
			msg, args, xerr.Stderr, xerr)
	}
	return fmt.Errorf("%s", msg)
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
		return nil, fmt.Errorf("couldn't find Postgres; tried %s",
			strings.Join(allPaths, ":"))
	}
	return pgCmds, nil
}

// PostgresInstalled returns an error if the module is unable to operate due to
// a failure to locate PostgreSQL.
func PostgresInstalled(path string) error {
	_, err := findPostgres(path)
	return err
}

// New returns an instance of BriefPG; if no PostgresPath option is present,
// attempt to automatically locate an instance of Postgres by first scanning
// the user's $PATH environment variable.  If that fails, try a series of
// well-known installation locations.  See the documentation for specific
// Options to understand what they do.
func New(options ...Option) (*BriefPG, error) {
	bpg := &BriefPG{
		state:          stateUninitialized,
		encoding:       "UNICODE",
		logf:           NullLogFunction,
		pgCmds:         nil,
		pgConfTemplate: DefaultPgConfTemplate,
	}

	for _, o := range options {
		err := o.apply(bpg)
		if err != nil {
			return nil, fmt.Errorf("Failed applying option: %w", err)
		}
	}

	// We've applied options-- if there isn't a specified postgres dir, go
	// look for it.
	if bpg.pgCmds == nil {
		err := bpg.setPostgresPath("")
		if err != nil {
			return nil, fmt.Errorf("Unable to find Postgres")
		}
	}

	return bpg, nil
}

// SetOption applies an Option to the BriefPG.  The option may fail to apply if
// invalid, or if the the BriefPG is in a state where applying the option is
// impossible.  Passing Options to New() is preferred.
func (bp *BriefPG) SetOption(o Option) error {
	return o.apply(bp)
}

// setPostgresPath looks for postgres at the indicated location.  If not
// present, it fails.  This is also when we harvest the postgres version.
func (bp *BriefPG) setPostgresPath(pgPath string) error {
	var err error

	if bp.state >= stateInitialized {
		return fmt.Errorf("postgres path cannot be set after db has been initialized")
	}
	bp.pgCmds, err = findPostgres(pgPath)
	if err != nil {
		return err
	}

	outb, err := exec.Command(bp.pgCmds["pg_ctl"], "-V").Output()
	if err != nil {
		return fmt.Errorf("Failed running pg_ctl -V: %w", err)
	}
	out := strings.TrimSpace(string(outb))
	sl := strings.Split(out, " ")
	bp.pgVer = sl[len(sl)-1]
	return nil
}

func (bp *BriefPG) setTmpDir(tmpDir string) error {
	if bp.madeTmpDir {
		return fmt.Errorf("tmpdir cannot be set after tmpdir has been created")
	}
	bp.tmpDir = tmpDir
	return nil
}

func (bp *BriefPG) setPostgresEncoding(enc string) error {
	bp.encoding = enc
	return nil
}

func (bp *BriefPG) mkTemp() error {
	var err error

	dirPrefix := "briefpg."
	// Refine the prefix if we can
	user, err := user.Current()
	if err == nil && user.Username != "" {
		dirPrefix = fmt.Sprintf("briefpg.%s.", user.Username)
	}

	bp.tmpDir, err = ioutil.TempDir("", dirPrefix)
	if err != nil {
		return fmt.Errorf("Failed to make tmpdir: %w", err)
	}
	bp.madeTmpDir = true
	return nil
}

// PgVer returns the detected version of Postgres
func (bp *BriefPG) PgVer() string {
	return bp.pgVer
}

// DbDir returns the installation directory of the Postgres database.  In
// general, this should not be needed when writing tests, but it is provided
// for completeness.
func (bp *BriefPG) DbDir() string {
	return filepath.Join(bp.tmpDir, bp.pgVer)
}

func (bp *BriefPG) initDB(ctx context.Context) error {
	if bp.tmpDir == "" {
		if err := bp.mkTemp(); err != nil {
			return err
		}
		bp.state = statePresent
	} else if _, err := os.Stat(bp.tmpDir); err != nil {
		bp.state = stateNotPresent
		return fmt.Errorf("Tmpdir %s not present or not readable: %w", bp.tmpDir, err)
	}

	if _, err := os.Stat(bp.DbDir()); err != nil {
		cmd := exec.Command(bp.pgCmds["initdb"], "--nosync", "-U", "postgres",
			"-D", bp.DbDir(), "-E", bp.encoding, "-A", "trust")
		bp.logf("briefpg: %s\n", strings.Join(cmd.Args, " "))
		cmdOut, err := cmd.CombinedOutput()
		if err != nil {
			bp.logf("briefpg: FAILED: %s\n", string(cmdOut))
			return wrapExecErr("initDB failed", cmd, err)
		}
	}
	confFile := filepath.Join(bp.DbDir(), "postgresql.conf")
	bp.logf("briefpg: generating %s\n", confFile)
	tmpl, err := template.New("postgresql.conf").Parse(bp.pgConfTemplate)
	if err != nil {
		return fmt.Errorf("initDB failed to parse postgresql.conf template: %w", err)
	}
	conf, err := os.OpenFile(confFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("initDB failed to open config: %w", err)
	}
	defer conf.Close()

	bpConf := struct {
		TmpDir string
	}{
		TmpDir: bp.tmpDir,
	}
	err = tmpl.Execute(conf, bpConf)
	if err != nil {
		return fmt.Errorf("initDB failed to execute template: %w", err)
	}
	bp.state = stateInitialized
	return nil
}

// Start the postgres server, performing necessary initialization along the way
func (bp *BriefPG) Start(ctx context.Context) error {
	var err error
	if bp.state == stateDefunct {
		return fmt.Errorf("briefpg instance is defunct")
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
	cmd := exec.Command(bp.pgCmds["pg_ctl"], "-w", "-o", postgresOpts, "-s",
		"-D", bp.DbDir(), "-l", logFile, "start")
	bp.logf("briefpg: %s\n", strings.Join(cmd.Args, " "))
	cmdOut, err := cmd.CombinedOutput()
	if err != nil {
		bp.logf("briefpg: %s\n", string(cmdOut))
		return wrapExecErr("Start failed", cmd, err)
	}
	bp.state = stateServerStarted
	return nil
}

// CreateDB is a convenience function to create a named database; you can do
// this using your database driver instead, at lower cost.  This routine uses
// 'psql' to do the job.  The primary use case is to rapidly set up an empty
// database for test purposes.  The URI to access the database is returned.
func (bp *BriefPG) CreateDB(ctx context.Context, dbName, createArgs string) (string, error) {
	if bp.state < stateServerStarted {
		return "", fmt.Errorf("Server not started; cannot create database")
	}
	scmd := fmt.Sprintf("CREATE DATABASE \"%s\" %s", dbName, createArgs)
	cmd := exec.Command(bp.pgCmds["psql"], "-c", scmd, bp.DBUri("postgres"))
	bp.logf("briefpg: %s\n", strings.Join(cmd.Args, " "))
	cmdOut, err := cmd.CombinedOutput()
	for _, line := range strings.Split(strings.TrimSpace(string(cmdOut)), "\n") {
		bp.logf("briefpg: %s\n", line)
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
		return fmt.Errorf("Server not started; cannot dump database")
	}
	cmd := exec.Command(bp.pgCmds["pg_dump"], bp.DBUri(dbName))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	bp.logf("briefpg: starting dump: %s\n", strings.Join(cmd.Args, " "))
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
	return fmt.Sprintf("postgresql:///%s?host=%s&user=postgres", dbName, bp.tmpDir)
}

// Fini stops the database server, if running, and cleans it up
func (bp *BriefPG) Fini(ctx context.Context) error {
	if bp.state >= stateServerStarted {
		cmd := exec.Command(bp.pgCmds["pg_ctl"], "-m", "immediate", "-w",
			"-D", bp.DbDir(), "stop")
		bp.logf("briefpg: %s\n", strings.Join(cmd.Args, " "))
		cmdOut, err := cmd.CombinedOutput()
		if err != nil {
			bp.logf("briefpg: %s\n", string(cmdOut))
			return wrapExecErr("Fini failed", cmd, err)
		}
	}

	if bp.state >= statePresent {
		if bp.madeTmpDir {
			bp.logf("briefpg: cleaning up %s\n", bp.tmpDir)
			os.RemoveAll(bp.tmpDir)
		}
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
