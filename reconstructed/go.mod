// RECONSTRUCTION — not the original gateway source.
//
// Package/function layout mirrors the `updater` module recovered from the
// gateway binary's pclntab (see ../REVERSE-ENGINEERING.md). Bodies are a
// compatibility reimplementation of the OBSERVED contract, written from the
// outside; they are not decompiled original code.
//
// The original is a go-zero service. This skeleton is stdlib-only so it builds
// and tests offline with no external dependencies.
module ppanel-gateway/reconstructed

go 1.24
