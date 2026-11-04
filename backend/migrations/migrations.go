package migrations

import "embed"

// Files contains the versioned SQL migrations shipped with the API.
//
//go:embed *.up.sql
var Files embed.FS
