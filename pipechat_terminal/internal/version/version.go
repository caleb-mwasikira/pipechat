// Package version compares program versions such as 1.2.3 or 1.3.0-beta.1. The server uses it to find the
// newest client, and the client to decide whether that one is newer than itself.
package version

import (
	"strconv"
	"strings"
)

// Valid reports whether text looks like 1.2.3 or 1.2.3-beta.1 (an optional leading v is allowed).
func Valid(text string) bool {
	core, _, _ := strings.Cut(strings.TrimPrefix(text, "v"), "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if _, err := strconv.ParseUint(part, 10, 32); err != nil {
			return false
		}
	}
	return true
}

// Compare returns -1, 0 or 1 as a is older than, the same as, or newer than b. A version with a
// pre-release suffix (1.2.3-beta.1) is older than the same version without one. Text that is not a
// version is older than any that is.
func Compare(a, b string) int {
	va, vb := Valid(a), Valid(b)
	switch {
	case !va && !vb:
		return 0
	case !va:
		return -1
	case !vb:
		return 1
	}
	coreA, preA, _ := strings.Cut(strings.TrimPrefix(a, "v"), "-")
	coreB, preB, _ := strings.Cut(strings.TrimPrefix(b, "v"), "-")
	partsA, partsB := strings.Split(coreA, "."), strings.Split(coreB, ".")
	for i := range 3 {
		x, _ := strconv.Atoi(partsA[i])
		y, _ := strconv.Atoi(partsB[i])
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	switch {
	case preA == preB:
		return 0
	case preA == "": // a is a release, b a pre-release
		return 1
	case preB == "":
		return -1
	}
	return comparePre(preA, preB)
}

// comparePre orders pre-release names like beta.2 and beta.10: numbers as numbers, the rest as text.
func comparePre(a, b string) int {
	partsA, partsB := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(partsA) && i < len(partsB); i++ {
		x, errX := strconv.Atoi(partsA[i])
		y, errY := strconv.Atoi(partsB[i])
		switch {
		case errX == nil && errY == nil && x != y:
			if x < y {
				return -1
			}
			return 1
		case errX == nil && errY == nil:
		case partsA[i] != partsB[i]:
			if partsA[i] < partsB[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(partsA) < len(partsB):
		return -1
	case len(partsA) > len(partsB):
		return 1
	}
	return 0
}
