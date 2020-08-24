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
	"fmt"
	"log"
)

func ExamplePostgresInstalled() {
	err := PostgresInstalled("")
	if err != nil {
		log.Fatalf("Can't continue; did not find an installed PostgreSQL instance: %v", err)
	}
	fmt.Printf("Found Postgres")

	// Output: Found Postgres
}

func ExampleBriefPG_Start() {
	ctx := context.Background()
	bpg, err := New()
	if err != nil {
		log.Fatalf("New failed: %v", err)
	}
	err = bpg.Start(ctx)
	if err != nil {
		log.Fatalf("Start failed: %v", err)
	}
	fmt.Printf("Started")
	defer bpg.MustFini(ctx)

	// At this point, you might use the pq SQL driver to
	// open the database using sql.Open(); you can obtain
	// the database URI with bpg.DBUri().

	// Output: Started
}
