package evidence

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/itxcrusher/git-casebook/internal/model"
)

type Store struct {
	CaseRoot string
}

func (s Store) PutBytes(extension string, data []byte) (string, string, error) {
	if extension == "" || strings.ContainsAny(extension, `/\\`) {
		return "", "", fmt.Errorf("invalid artifact extension %q", extension)
	}
	digest := model.SHA256(data)
	rel := filepath.ToSlash(filepath.Join("artifacts", "sha256", digest+"."+extension))
	path, err := s.resolve(rel)
	if err != nil {
		return "", "", err
	}
	if existing, err := os.ReadFile(path); err == nil {
		if model.SHA256(existing) != digest {
			return "", "", fmt.Errorf("content-addressed artifact mismatch at %s", rel)
		}
		return rel, digest, nil
	} else if !os.IsNotExist(err) {
		return "", "", err
	}
	if err := writeOnce(path, data); err != nil {
		return "", "", err
	}
	return rel, digest, nil
}

func (s Store) PutJSON(extension string, value any) (string, string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", "", fmt.Errorf("marshal artifact: %w", err)
	}
	b = append(b, '\n')
	return s.PutBytes(extension, b)
}

func (s Store) PutLines(extension string, values []string) (string, string, error) {
	copyValues := append([]string(nil), values...)
	sort.Strings(copyValues)
	data := []byte(strings.Join(copyValues, "\n"))
	if len(data) > 0 {
		data = append(data, '\n')
	}
	return s.PutBytes(extension, data)
}

func (s Store) ReadLines(rel string) ([]string, error) {
	path, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var values []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	for scanner.Scan() {
		if scanner.Text() != "" {
			values = append(values, scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Strings(values)
	return values, nil
}

func (s Store) ReadJSON(rel string, value any) error {
	path, err := s.resolve(rel)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, value); err != nil {
		return fmt.Errorf("decode artifact %s: %w", rel, err)
	}
	return nil
}

func (s Store) Verify(rel string) error {
	path, err := s.resolve(rel)
	if err != nil {
		return err
	}
	base := filepath.Base(path)
	dot := strings.IndexByte(base, '.')
	if dot != 64 {
		return fmt.Errorf("artifact %s does not have a SHA-256 filename", rel)
	}
	want := base[:dot]
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf("artifact %s digest mismatch", rel)
	}
	return nil
}

func (s Store) resolve(rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("artifact path must be relative")
	}
	root, err := filepath.Abs(s.CaseRoot)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	check, err := filepath.Rel(root, path)
	if err != nil || check == ".." || strings.HasPrefix(check, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("artifact path escapes case directory")
	}
	if err := rejectSymlinkComponents(root, path); err != nil {
		return "", err
	}
	return path, nil
}

func rejectSymlinkComponents(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	current := root
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("artifact path traverses symlink %s", current)
		}
	}
	return nil
}

func TreeFingerprint(root string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		return "", fmt.Errorf("walk preserved source: %w", err)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, rel := range paths {
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%s\x00%d\x00", rel, info.Mode().Type().String(), info.Size())
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return "", err
			}
			_, _ = io.WriteString(h, target)
		case info.Mode().IsRegular():
			f, err := os.Open(path)
			if err != nil {
				return "", err
			}
			_, copyErr := io.Copy(h, f)
			closeErr := f.Close()
			if copyErr != nil {
				return "", copyErr
			}
			if closeErr != nil {
				return "", closeErr
			}
		}
		_, _ = h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeOnce(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".artifact-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	keep := false
	defer func() {
		_ = tmp.Close()
		if !keep {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		keep = true
		return nil
	}
	if err := os.Rename(tmpPath, path); err != nil {
		if runtime.GOOS == "windows" {
			if _, statErr := os.Stat(path); statErr == nil {
				keep = true
				return nil
			}
		}
		return err
	}
	keep = true
	return nil
}
