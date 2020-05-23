package shared

type MyThing struct {
	Message string
}

type ServerRegistration struct {
	ID string
}

type TestResult struct {
	StatusCode int16
	ResponseTime int
	Target string
}

type ResultData struct {
	Results []AggregateTestResult
}

type AggregateTestResult struct {
	Target string
	Requests int
	StatusCodes map[string]int
	P50 int
	P90 int
	P99 int
	RequestsPerSecond float64
}

type TestTarget struct {
	URI string
	Method string
	Headers map[string][]string
	Body string
}

type Test struct {
	State string
	StateReason string
	Lease string
	Length int16
	VirtualUsers int16
	Servers []string
	Targets []TestTarget
}