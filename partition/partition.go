// Package partition splits batch parse results into successes and failures.
//
// Parsers that can fail per row return a ParseResult; Results runs the parser
// over a slice and gives back two slices — values and failures — so query
// callers can shape their own response without per-row error juggling.
package partition

type ParseResult[V any, E error] struct {
	Value V
	Err   E
	OK    bool
}

func Ok[V any, E error](v V) ParseResult[V, E] {
	return ParseResult[V, E]{Value: v, OK: true}
}

func Fail[V any, E error](e E) ParseResult[V, E] {
	return ParseResult[V, E]{Err: e, OK: false}
}

// Results returns (values, failures) preserving input order within each slice.
func Results[R any, V any, E error](
	rows []R,
	parse func(R) ParseResult[V, E],
) (values []V, failures []E) {
	values = make([]V, 0, len(rows))
	failures = make([]E, 0)
	for _, r := range rows {
		res := parse(r)
		if res.OK {
			values = append(values, res.Value)
		} else {
			failures = append(failures, res.Err)
		}
	}
	return values, failures
}
