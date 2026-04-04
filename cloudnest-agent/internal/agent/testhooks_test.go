package agent

func swapProcessAliveFunc(fn func(pid int) bool) func() {
	prev := processAliveFunc
	processAliveFunc = fn
	return func() {
		processAliveFunc = prev
	}
}

func swapStartDataPlaneFunc(fn func(addr string, rateLimit int64) error) func() {
	prev := startDataPlaneFunc
	startDataPlaneFunc = fn
	return func() {
		startDataPlaneFunc = prev
	}
}

func swapProbeDataPlaneFunc(fn func(addr string) error) func() {
	prev := probeDataPlaneFunc
	probeDataPlaneFunc = fn
	return func() {
		probeDataPlaneFunc = prev
	}
}
