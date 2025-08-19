package remind

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileWatcher struct {
	watcher  *fsnotify.Watcher
	files    map[string]time.Time
	onChange func(string)
	mu       sync.RWMutex
	done     chan struct{}
}

func NewFileWatcher(onChange func(string)) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	fw := &FileWatcher{
		watcher:  watcher,
		files:    make(map[string]time.Time),
		onChange: onChange,
		done:     make(chan struct{}),
	}

	go fw.watch()
	return fw, nil
}

func (fw *FileWatcher) AddFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()

	if _, exists := fw.files[absPath]; exists {
		return nil // Already watching
	}

	err = fw.watcher.Add(absPath)
	if err != nil {
		return err
	}

	fw.files[absPath] = time.Now()
	return nil
}

func (fw *FileWatcher) RemoveFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()

	if _, exists := fw.files[absPath]; !exists {
		return nil // Not watching
	}

	err = fw.watcher.Remove(absPath)
	if err != nil {
		return err
	}

	delete(fw.files, absPath)
	return nil
}

func (fw *FileWatcher) watch() {
	debounce := make(map[string]*time.Timer)

	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				// Debounce rapid events
				if timer, exists := debounce[event.Name]; exists {
					timer.Stop()
				}

				debounce[event.Name] = time.AfterFunc(100*time.Millisecond, func() {
					fw.mu.RLock()
					if _, watching := fw.files[event.Name]; watching {
						fw.mu.RUnlock()
						if fw.onChange != nil {
							fw.onChange(event.Name)
						}
					} else {
						fw.mu.RUnlock()
					}

					delete(debounce, event.Name)
				})
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			// Log error but continue watching
			_ = err

		case <-fw.done:
			return
		}
	}
}

func (fw *FileWatcher) Close() error {
	close(fw.done)
	return fw.watcher.Close()
}
