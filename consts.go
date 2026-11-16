package uno

import "time"

var defaultDecoderTimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	time.DateOnly,
	time.TimeOnly}

var defaultEncoderTimeLayout = time.RFC3339
