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

import (
	"bufio"
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestPostgresInstalled(t *testing.T) {
	err := PostgresInstalled("")
	if err != nil {
		t.Fatalf("PostgresInstalled failed: %v", err)
	}

	err = PostgresInstalled("/bogus/path")
	if err == nil {
		t.Fatalf("PostgresInstalled expected to fail: %v", err)
	}

	_, err = New(OptPostgresPath("/bogus/path"))
	if err == nil {
		t.Fatalf("New with bogus path expected to fail: %v", err)
	}
}

func TestBrief(t *testing.T) {
	ctx := context.Background()
	bpg, err := New(OptLogFunc(t.Logf))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	ver := bpg.PgVer()
	matched, err := regexp.Match("[0-9]+.", []byte(ver))
	if err != nil || !matched {
		t.Fatalf("Unexpected version: %v", ver)
	}
	err = bpg.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	bpg.MustFini(ctx)

	// Now it is "defunct" so, can't start it again.
	err = bpg.Start(ctx)
	if err == nil {
		bpg.MustFini(ctx)
		t.Fatalf("Expected Start to fail")
	}
}

func TestTmpDir(t *testing.T) {
	ctx := context.Background()

	bpg, err := New(OptLogFunc(t.Logf), OptTmpDir("/some/bogus/path"))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Start should fail because tmpdir is set, and bogus
	err = bpg.Start(ctx)
	if err == nil {
		bpg.MustFini(ctx)
		t.Fatalf("Expected start to fail")
	}

	// Now make tmpdir and apply it as an option
	tmpDir, err := ioutil.TempDir("", "test.")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := bpg.SetOption(OptTmpDir(tmpDir)); err != nil {
		t.Fatalf("SetOption failed: %v", err)
	}

	err = bpg.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	bpg.MustFini(ctx)

	// Manually created tmpDir should survive the Fini()
	_, err = os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}

	pgVer := bpg.PgVer()
	_, err = os.Stat(filepath.Join(tmpDir, pgVer))
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
}

func TestBadEncoding(t *testing.T) {
	ctx := context.Background()
	bpg, err := New(OptPostgresEncoding("GARBAGE"))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer bpg.MustFini(ctx)

	err = bpg.Start(ctx)
	if err == nil {
		t.Fatalf("Expected start to fail")
	}
	t.Logf("err: %v", err)
}

func TestBadPgPath(t *testing.T) {
	ctx := context.Background()
	bpg, err := New(OptPostgresPath("/garbage/path"))
	if err == nil || bpg != nil {
		t.Fatalf("expecting New to fail")
	}
	t.Logf("New failed as expected: %v", err)

	bpg, err = New()
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer bpg.MustFini(ctx)

	err = bpg.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	err = bpg.SetOption(OptPostgresPath(""))
	if err == nil {
		t.Fatalf("Expected SetOption to fail")
	}
}

func TestCreateDB(t *testing.T) {
	ctx := context.Background()
	bpg, err := New(OptLogFunc(t.Logf))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	_, err = bpg.CreateDB(ctx, "test_db", "")
	if err == nil {
		t.Fatalf("Expected CreatedDB to fail")
	}

	err = bpg.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer bpg.MustFini(ctx)

	_, err = bpg.CreateDB(ctx, "test_db", "")
	if err != nil {
		t.Fatalf("CreatedDB failed: %v", err)
	}
}

func TestDumpDB(t *testing.T) {
	ctx := context.Background()
	bpg, err := New(OptLogFunc(t.Logf))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	outBuf := new(bytes.Buffer)
	outBufW := bufio.NewWriter(outBuf)

	err = bpg.DumpDB(ctx, "test_db", outBufW)
	if err == nil {
		t.Fatalf("Expected DumpDB to fail: %s", outBuf.String())
	}

	err = bpg.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	_, err = bpg.CreateDB(ctx, "test_db", "")
	if err != nil {
		t.Fatalf("CreatedDB failed: %v", err)
	}

	err = bpg.DumpDB(ctx, "garbagedb", outBufW)
	if err == nil {
		t.Fatalf("Expected DumpDB to fail: %s", outBuf.String())
	}

	err = bpg.DumpDB(ctx, "test_db", outBufW)
	if err != nil {
		t.Fatalf("Expected DumpDB to work: %v", err)
	}
	t.Logf("dumpdb output: %s", outBuf.String())

	bpg.MustFini(ctx)

	err = bpg.DumpDB(ctx, "test_db", outBufW)
	if err == nil {
		t.Fatalf("Expected DumpDB to fail: %s", outBuf.String())
	}
}
