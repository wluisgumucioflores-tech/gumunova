// Package migrations embeds the versioned SQL migrations.
// Applied migrations are never edited: add a new numbered file instead.
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
