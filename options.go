/*
 * COPYRIGHT 2020 Brightgate Inc.  All rights reserved.
 *
 * This copyright notice is Copyright Management Information under 17 USC 1202
 * and is included to protect this work and deter copyright infringement.
 * Removal or alteration of this Copyright Management Information without the
 * express written permission of Brightgate Inc is prohibited, and any
 * such unauthorized removal or alteration will be a violation of federal law.
 */

package briefpg

//
// This pattern was cribbed from zap's Option interface
//

// Option describes a generic interface for options.
type Option interface {
	apply(*BriefPG) error
}

// optionFunc wraps a func so it satisfies the Option interface.
type optionFunc func(*BriefPG) error

func (f optionFunc) apply(bpg *BriefPG) error {
	return f(bpg)
}

// OptPostgresPath returns an Option which sets the location to look for Postgres.
// If a satisfactory Postgres is not found at that location, returns an error.
// This option can only be set before calling Start().
func OptPostgresPath(dir string) Option {
	return optionFunc(func(bpg *BriefPG) error {
		return bpg.setPostgresPath(dir)
	})
}

// OptLogFunc returns an Option which sets the logging function for BriefPG.
// A typical usage is err := bpg.SetOption(briefpg.OptLogFunc(t.Logf)) to
// connect BriefPG to the test's logging.
func OptLogFunc(logf LogFunction) Option {
	return optionFunc(func(bpg *BriefPG) error {
		bpg.logf = logf
		return nil
	})
}

// OptTmpDir returns an Option which sets the temporary directory where the
// postgres database will put its files.  If no user-specified OptTmpDir is
// set, ioutil.TempDir is used to create one automatically.  The caller is
// responsible for making the TempDir.  This option can only be set before
// calling Start().  If the TmpDir is specified, and user-created, it will not
// be cleaned up when calling Fini() or MustFini().  If automatically created,
// it will be automatically cleaned up.
func OptTmpDir(dir string) Option {
	return optionFunc(func(bpg *BriefPG) error {
		return bpg.setTmpDir(dir)
	})
}

// OptPostgresEncoding returns an Option which sets the -E argument to the
// postgres 'initdb' command; this sets the default encoding for new databases.
// If not set, the default is UNICODE.  This option can only be set before
// calling Start().
func OptPostgresEncoding(enc string) Option {
	return optionFunc(func(bpg *BriefPG) error {
		return bpg.setPostgresEncoding(enc)
	})
}
