package update

import (
	"strconv"
	"strings"
)

func parseVersion(value string) ([3]uint64, bool) {
	var version [3]uint64
	parts := strings.Split(strings.TrimPrefix(value, "v"), ".")
	if len(parts) != len(version) {
		return version, false
	}
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return version, false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return version, false
			}
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return version, false
		}
		version[i] = n
	}
	return version, true
}

func compareVersions(a, b [3]uint64) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
