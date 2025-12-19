package status

import "slices"

// TaskTransitions defines valid state transitions for tasks
// Key is the current state, value is a list of valid next states
var TaskTransitions = map[Task][]Task{
	TaskPending:    {TaskProcessing, TaskCancelled, TaskFailed},
	TaskProcessing: {TaskCompleted, TaskFailed},
	TaskCompleted:  {},                // terminal state
	TaskFailed:     {TaskPending},     // can retry
	TaskCancelled:  {},                // terminal state
}

// SiteTransitions defines valid state transitions for sites
var SiteTransitions = map[Site][]Site{
	SitePending: {SiteActive, SiteFrozen, SiteMoved},
	SiteActive:  {SiteDown, SiteFrozen, SitePending, SiteMoved},
	SiteDown:    {SiteActive, SiteDead, SiteFrozen, SiteMoved},
	SiteDead:    {SitePending}, // can only reset to pending for re-detection
	SiteFrozen:  {SiteActive, SitePending},
	SiteMoved:   {}, // terminal state - domain redirected to another domain
}

// URLTransitions defines valid state transitions for sitemap URLs
var URLTransitions = map[URL][]URL{
	URLPending: {URLIndexed, URLError, URLSkipped},
	URLIndexed: {},            // terminal state
	URLError:   {URLPending},  // can retry after fixing
	URLSkipped: {},            // terminal state
}

// CanTaskTransition checks if a task status transition is valid
func CanTaskTransition(from, to Task) bool {
	allowed, ok := TaskTransitions[from]
	if !ok {
		return false
	}
	return slices.Contains(allowed, to)
}

// CanSiteTransition checks if a site status transition is valid
func CanSiteTransition(from, to Site) bool {
	allowed, ok := SiteTransitions[from]
	if !ok {
		return false
	}
	return slices.Contains(allowed, to)
}

// CanURLTransition checks if a URL status transition is valid
func CanURLTransition(from, to URL) bool {
	allowed, ok := URLTransitions[from]
	if !ok {
		return false
	}
	return slices.Contains(allowed, to)
}

// ActiveTaskStatuses returns statuses that indicate an active task
func ActiveTaskStatuses() []Task {
	return []Task{TaskPending, TaskProcessing}
}

// ScannableSiteStatuses returns statuses that allow scanning
func ScannableSiteStatuses() []Site {
	return []Site{SiteActive, SiteDown}
}
