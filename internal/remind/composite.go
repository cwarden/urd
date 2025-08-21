package remind

import (
	"sync"
	"time"
)

// CompositeSource combines multiple ReminderSources
type CompositeSource struct {
	sources   []ReminderSource
	mu        sync.RWMutex
	eventChan chan FileChangeEvent
	stopChans []chan struct{}
}

// NewCompositeSource creates a new composite reminder source
func NewCompositeSource(sources ...ReminderSource) *CompositeSource {
	return &CompositeSource{
		sources:   sources,
		eventChan: make(chan FileChangeEvent, 10),
	}
}

// AddSource adds a new source to the composite
func (c *CompositeSource) AddSource(source ReminderSource) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sources = append(c.sources, source)
}

// SetFiles implements ReminderSource - sets files on all sources that support them
func (c *CompositeSource) SetFiles(files []string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, source := range c.sources {
		source.SetFiles(files)
	}
}

// GetEvents implements ReminderSource - combines events from all sources
func (c *CompositeSource) GetEvents(start, end time.Time) ([]Event, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var allEvents []Event
	eventMap := make(map[string]Event) // Deduplicate by ID

	for _, source := range c.sources {
		events, err := source.GetEvents(start, end)
		if err != nil {
			// Log error but continue with other sources
			continue
		}

		for _, event := range events {
			// Use event ID for deduplication
			if _, exists := eventMap[event.ID]; !exists {
				eventMap[event.ID] = event
			}
		}
	}

	// Convert map back to slice
	for _, event := range eventMap {
		allEvents = append(allEvents, event)
	}

	return allEvents, nil
}

// WatchFiles implements ReminderSource - watches all sources
func (c *CompositeSource) WatchFiles() (<-chan FileChangeEvent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Start watching all sources
	for _, source := range c.sources {
		stopChan := make(chan struct{})
		c.stopChans = append(c.stopChans, stopChan)

		sourceChan, err := source.WatchFiles()
		if err != nil || sourceChan == nil {
			continue // Skip sources that don't support watching
		}

		// Forward events from this source to our composite channel
		go func(src <-chan FileChangeEvent, stop chan struct{}) {
			for {
				select {
				case event, ok := <-src:
					if !ok {
						return
					}
					select {
					case c.eventChan <- event:
					default:
						// Channel full, drop event
					}
				case <-stop:
					return
				}
			}
		}(sourceChan, stopChan)
	}

	return c.eventChan, nil
}

// StopWatching implements ReminderSource - stops watching all sources
func (c *CompositeSource) StopWatching() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Stop all watchers
	for _, stopChan := range c.stopChans {
		close(stopChan)
	}
	c.stopChans = nil

	// Stop watching in all sources
	for _, source := range c.sources {
		source.StopWatching()
	}

	if c.eventChan != nil {
		close(c.eventChan)
		c.eventChan = nil
	}

	return nil
}
