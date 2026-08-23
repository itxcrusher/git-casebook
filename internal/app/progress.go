package app

import (
	"fmt"
	"io/fs"
	"path/filepath"
)

// largeMirrorThreshold is the point past which a preserved mirror is worth
// explaining rather than merely reporting. GitHub serves forks from shared
// object storage, so mirroring a repository inside a large fork network can
// transfer far more than the project's own history.
const largeMirrorThreshold = 512 * 1024 * 1024

// Progress receives human-facing acquisition activity. A nil Progress keeps the
// application silent, which is the correct behaviour under --json and in tests.
type Progress func(line string)

func (a App) report(line string) {
	if a.Progress != nil {
		a.Progress(line)
	}
}

func acquisitionStartLine(sourceID, locator string) string {
	return fmt.Sprintf("Acquiring %s from %s ...", sourceID, locator)
}

func acquisitionDoneLine(sourceID string, size int64) string {
	line := fmt.Sprintf("Preserved %s (%s)", sourceID, humanBytes(size))
	if size >= largeMirrorThreshold {
		line += "; larger than the project's own history suggests." +
			" A repository inside a large fork network shares object storage with its forks."
	}
	return line
}

// directorySize sums regular file sizes below root. It is presentation-only and
// never fails the acquisition: an unreadable entry simply does not contribute.
func directorySize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

func humanBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	for _, name := range units {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, name)
		}
	}
	return fmt.Sprintf("%.1f PiB", value/unit)
}
