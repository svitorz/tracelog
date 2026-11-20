// Package migrations embeds the SQL migrations so that `tracelog migrate` works from the
// binary alone (container, CI), without the files on disk.
package migrations

import "embed"

// Files holds every *.sql migration of this directory.
//
//go:embed *.sql
var Files embed.FS
