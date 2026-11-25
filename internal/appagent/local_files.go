package appagent

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxEditableFile = 16 << 20

func withinDirectory(base, filename string) (string, bool) {
	if base == "" {
		return "", false
	}
	relative, err := filepath.Rel(base, filename)
	return relative, err == nil && filepath.IsLocal(relative)
}

func openLocalRoot(directory string) (*os.Root, error) {
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return nil, errors.New("DirectoryUnavailable")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, errors.New("DirectoryUnavailable")
	}
	actual, err := root.Stat(".")
	if err != nil || !os.SameFile(info, actual) {
		root.Close()
		return nil, errors.New("DirectoryUnavailable")
	}
	return root, nil
}

// Each component is opened relative to its checked parent. An exchanged link
// cannot redirect the default output directory into another conversation.
func openOutputRoot(home, output string, create bool) (*os.Root, error) {
	relative, ok := withinDirectory(home, output)
	if !ok || relative == "." {
		return nil, errors.New("PathNotAllowed")
	}
	root, err := openLocalRoot(home)
	if err != nil {
		return nil, err
	}
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		info, err := root.Lstat(part)
		if errors.Is(err, os.ErrNotExist) && create {
			if createErr := root.Mkdir(part, 0700); createErr != nil && !errors.Is(createErr, os.ErrExist) {
				root.Close()
				return nil, errors.New("StorageError")
			}
			info, err = root.Lstat(part)
		}
		if err != nil || !info.IsDir() {
			root.Close()
			return nil, errors.New("DirectoryUnavailable")
		}
		next, err := root.OpenRoot(part)
		root.Close()
		if err != nil {
			return nil, errors.New("DirectoryUnavailable")
		}
		actual, err := next.Stat(".")
		if err != nil || !os.SameFile(info, actual) {
			next.Close()
			return nil, errors.New("PathNotAllowed")
		}
		root = next
	}
	return root, nil
}

func regularLocalFile(root *os.Root, name string) (*os.File, error) {
	info, err := root.Stat(name)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("FileUnavailable")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, errors.New("FileUnavailable")
	}
	actual, err := file.Stat()
	if err != nil || !actual.Mode().IsRegular() || !os.SameFile(info, actual) {
		file.Close()
		return nil, errors.New("FileUnavailable")
	}
	return file, nil
}

func editLocalText(ctx context.Context, source io.Reader, old, replacement string, all bool) ([]byte, int, error) {
	text, encoding, err := openText(io.LimitReader(source, maxEditableFile+1))
	if err != nil {
		return nil, 0, err
	}
	data, err := io.ReadAll(io.LimitReader(skillContextReader{ctx: ctx, Reader: text}, maxEditableFile+1))
	if err != nil {
		return nil, 0, err
	}
	if len(data) > maxEditableFile {
		return nil, 0, errors.New("FileTooLarge")
	}
	body := string(data)
	runes := bufio.NewReader(strings.NewReader(body))
	for {
		r, n, err := runes.ReadRune()
		if err != nil {
			break
		}
		if !textRune(r, n) {
			return nil, 0, binaryFile
		}
	}
	count := strings.Count(body, old)
	if count == 0 {
		return nil, 0, errors.New("TextNotFound")
	}
	if !all && count != 1 {
		return nil, 0, errors.New("TextNotUnique")
	}
	if int64(len(body))+int64(count)*int64(len(replacement)-len(old)) > maxEditableFile {
		return nil, 0, errors.New("FileTooLarge")
	}
	result, err := encoding.encode(strings.Replace(body, old, replacement, count))
	if len(result) > maxEditableFile {
		return nil, 0, errors.New("FileTooLarge")
	}
	return result, count, err
}

func inspectWriteTarget(root *os.Root, path string) (os.FileInfo, error) {
	info, err := root.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, nil
	case err != nil || !info.Mode().IsRegular():
		return nil, errors.New("FileUnavailable")
	default:
		return info, nil
	}
}

func sameWriteTarget(previous, current os.FileInfo) bool {
	if previous == nil || current == nil {
		return previous == nil && current == nil
	}
	return os.SameFile(previous, current) && previous.Size() == current.Size() && previous.ModTime().Equal(current.ModTime())
}

func replaceLocalFile(ctx context.Context, root *os.Root, relative string, previous os.FileInfo, content []byte) error {
	parent := filepath.Dir(relative)
	if err := root.MkdirAll(parent, 0700); err != nil {
		return errors.New("StorageError")
	}
	mode := os.FileMode(0600)
	if previous != nil {
		mode = previous.Mode().Perm()
	}
	pending := filepath.Join(parent, ".fealdhord-"+rand.Text())
	file, err := root.OpenFile(pending, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return errors.New("StorageError")
	}
	defer root.Remove(pending)
	_, writeErr := file.Write(content)
	if errors.Join(writeErr, file.Close()) != nil {
		return errors.New("StorageError")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := inspectWriteTarget(root, relative)
	if err != nil || !sameWriteTarget(previous, current) {
		return errors.New("FileChanged")
	}
	if previous == nil {
		// A link publishes a new file only if the destination is still absent.
		if err := root.Link(pending, relative); err != nil {
			if errors.Is(err, os.ErrExist) {
				return errors.New("FileChanged")
			}
			return errors.New("StorageError")
		}
	} else if err := root.Rename(pending, relative); err != nil {
		return errors.New("StorageError")
	}
	return nil
}
