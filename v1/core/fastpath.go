package core

import "os"

// Fastpath selects optimised code paths (direct Pix access, append-based
// formatting). Every fast path must produce output identical to its simple
// path; the simple path is the reference that changes first. It is read once
// from CATI_FASTPATH; "0" selects the simple paths. Tests may toggle it.
var Fastpath = os.Getenv("CATI_FASTPATH") != "0"
