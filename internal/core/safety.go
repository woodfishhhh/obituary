package core

import (
	"io/fs"

	"github.com/woodfishhhh/obituary/internal/core/guard"
	"github.com/woodfishhhh/obituary/internal/core/internal/pathcheck"
)

type Guard = guard.Guard

type RefusedError = guard.RefusedError

func DefaultGuard(home string, keep ...string) Guard { return guard.DefaultGuard(home, keep...) }

func samePath(a, b string) bool { return pathcheck.SamePath(a, b) }

func isInside(child, parent string) bool { return pathcheck.IsInside(child, parent) }

func isLink(info fs.FileInfo) bool { return pathcheck.IsLink(info) }
