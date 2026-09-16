package remind

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher reports changes to a set of files. It watches the directory
// containing each file rather than the file itself: an inotify watch on a
// file is removed when the file is replaced by a rename, which is how many
// editors and tools (sed -i, rsync, sync clients) save, so a watch on the
// file would stop reporting after the first such save.
type FileWatcher struct {
	watcher *fsnotify.Watcher
	// files maps each watched path to the path passed to AddFile, which is
	// the path reported to onChange. A symlink is watched at both its own
	// path and its target's path.
	files map[string]string
	// dirs counts the watched paths in each watched directory.
	dirs     map[string]int
	onChange func(string)
	mu       sync.Mutex
	debounce map[string]*time.Timer
	done     chan struct{}
}

func NewFileWatcher(onChange func(string)) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	fw := &FileWatcher{
		watcher:  watcher,
		files:    make(map[string]string),
		dirs:     make(map[string]int),
		onChange: onChange,
		debounce: make(map[string]*time.Timer),
		done:     make(chan struct{}),
	}

	go fw.watch()
	return fw, nil
}

// watchPaths returns the paths to watch for path: its absolute path and, when
// it is a symlink, the absolute path of its target.
func watchPaths(path string) ([]string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	paths := []string{absPath}
	if target, err := filepath.EvalSymlinks(absPath); err == nil && target != absPath {
		paths = append(paths, target)
	}
	return paths, nil
}

func (fw *FileWatcher) AddFile(path string) error {
	paths, err := watchPaths(path)
	if err != nil {
		return err
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()

	for _, p := range paths {
		if _, exists := fw.files[p]; exists {
			continue
		}
		dir := filepath.Dir(p)
		if fw.dirs[dir] == 0 {
			if err := fw.watcher.Add(dir); err != nil {
				return err
			}
		}
		fw.dirs[dir]++
		fw.files[p] = path
	}
	return nil
}

func (fw *FileWatcher) RemoveFile(path string) error {
	paths, err := watchPaths(path)
	if err != nil {
		return err
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()

	for _, p := range paths {
		if _, exists := fw.files[p]; !exists {
			continue
		}
		delete(fw.files, p)
		dir := filepath.Dir(p)
		fw.dirs[dir]--
		if fw.dirs[dir] == 0 {
			delete(fw.dirs, dir)
			if err := fw.watcher.Remove(dir); err != nil {
				return err
			}
		}
	}
	return nil
}

func (fw *FileWatcher) watch() {
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}

			fw.mu.Lock()
			reported, watching := fw.files[event.Name]
			if !watching {
				fw.mu.Unlock()
				continue
			}
			// A save produces several events (truncate and write, or create
			// and rename), so report once after they stop arriving.
			if timer, exists := fw.debounce[reported]; exists {
				timer.Stop()
			}
			fw.debounce[reported] = time.AfterFunc(100*time.Millisecond, func() {
				fw.mu.Lock()
				delete(fw.debounce, reported)
				fw.mu.Unlock()
				select {
				case <-fw.done:
					return
				default:
				}
				if fw.onChange != nil {
					fw.onChange(reported)
				}
			})
			fw.mu.Unlock()

		case _, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}

		case <-fw.done:
			return
		}
	}
}

func (fw *FileWatcher) Close() error {
	close(fw.done)
	fw.mu.Lock()
	for _, timer := range fw.debounce {
		timer.Stop()
	}
	fw.mu.Unlock()
	return fw.watcher.Close()
}
