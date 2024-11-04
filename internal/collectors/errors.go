package collectors

import "errors"

// ErrCollectorSkipped is returned when a collector is skipped due to unsupported OS
var ErrCollectorSkipped = errors.New("collector skipped due to unsupported OS")
