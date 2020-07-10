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
	"context"
	"os"
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
}

func TestBrief(t *testing.T) {
	ctx := context.Background()
	bpg, err := NewWithOptions("", t.Logf)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	err = bpg.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer bpg.MustFini(ctx)
}

func TestCreateDB(t *testing.T) {
	ctx := context.Background()
	bpg, err := NewWithOptions("", t.Logf)
	if err != nil {
		t.Fatalf("New failed: %v", err)
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
	bpg, err := NewWithOptions("", t.Logf)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	err = bpg.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer bpg.MustFini(ctx)

	err = bpg.DumpDB(ctx, "test_db", os.Stdout)
	if err == nil {
		t.Fatalf("Expected DumpDB failed: %v", err)
	}
}
