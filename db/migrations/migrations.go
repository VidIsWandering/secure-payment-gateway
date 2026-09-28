// Package migrations embeds the versioned SQL migrations so the binary can
// apply them at startup without shipping the files separately.
package migrations

import "embed"

// FS holds every *.up.sql / *.down.sql migration in this directory.
//
//go:embed *.sql
var FS embed.FS
