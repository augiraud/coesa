package coesa

import "sync"

// A solution retains the physical quantities before any output-unit conversion
type physicalSolution struct {
	state   State
	numbers NumberDensities
}

func solveUpper(altitude float64) physicalSolution {
	numbers := upperAtmosphere(altitude)
	return physicalSolution{state: upperStateSI(baseStateSI(altitude), numbers), numbers: numbers}
}

type cacheEntry struct {
	altitude       float64
	solution       physicalSolution
	previous, next *cacheEntry
}

type cacheFlight struct {
	done     chan struct{}
	solution physicalSolution
}

// All mutable fields are protected by mu. Capacity and solve are immutable after
// construction. The production solver is total on validated upper altitudes.
type solutionCache struct {
	mu             sync.Mutex
	capacity       int
	solve          func(float64) physicalSolution
	entries        map[float64]*cacheEntry
	flights        map[float64]*cacheFlight
	newest, oldest *cacheEntry
	stats          CacheStats
}

// load accepts finite geometric metres above upperBoundary. Public methods
// validate all inputs and output units before reaching it. No rounding occurs.
func (c *solutionCache) load(altitude float64) physicalSolution {
	c.mu.Lock()
	if entry := c.entries[altitude]; entry != nil {
		c.stats.Hits++
		c.touch(entry)
		result := entry.solution
		c.mu.Unlock()
		return result
	}
	c.stats.Misses++
	if flight := c.flights[altitude]; flight != nil {
		c.stats.Coalesced++
		c.mu.Unlock()
		<-flight.done
		return flight.solution
	}
	if c.entries == nil {
		c.entries = make(map[float64]*cacheEntry)
		c.flights = make(map[float64]*cacheFlight)
	}
	flight := &cacheFlight{done: make(chan struct{})}
	c.flights[altitude] = flight
	c.mu.Unlock()

	result := c.solve(altitude)
	c.mu.Lock()
	c.store(altitude, result)
	flight.solution = result
	delete(c.flights, altitude)
	close(flight.done)
	c.mu.Unlock()
	return result
}

func (c *solutionCache) store(altitude float64, solution physicalSolution) {
	var entry *cacheEntry
	if len(c.entries) == c.capacity {
		// Recycle the evicted node to avoid an allocation on steady-state misses
		entry = c.oldest
		c.unlink(entry)
		delete(c.entries, entry.altitude)
		c.stats.Evictions++
	} else {
		entry = &cacheEntry{}
	}
	entry.altitude, entry.solution = altitude, solution
	c.pushFront(entry)
	c.entries[altitude] = entry
}

func (c *solutionCache) touch(entry *cacheEntry) {
	if c.newest == entry {
		return
	}
	c.unlink(entry)
	c.pushFront(entry)
}

func (c *solutionCache) unlink(entry *cacheEntry) {
	if entry.previous != nil {
		entry.previous.next = entry.next
	} else {
		c.newest = entry.next
	}
	if entry.next != nil {
		entry.next.previous = entry.previous
	} else {
		c.oldest = entry.previous
	}
}

func (c *solutionCache) pushFront(entry *cacheEntry) {
	entry.previous, entry.next = nil, c.newest
	if c.newest != nil {
		c.newest.previous = entry
	} else {
		c.oldest = entry
	}
	c.newest = entry
}
