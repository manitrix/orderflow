// Package migrations contains the Phase 1 bootstrap migration.
package migrations

import _ "embed"

//go:embed 001_orders.sql
var Initial string
