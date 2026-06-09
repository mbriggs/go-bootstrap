package webtest

import (
	"fmt"
	"net/url"
	"strings"
)

type PathState struct {
	URI    string
	Params map[string]string
}

// Path builds request path state from fragments: Path("v1", "things", id).
func Path(fragments ...string) *PathState {
	path := PathState{
		URI:    "/" + strings.Join(fragments, "/"),
		Params: make(map[string]string),
	}

	return &path
}

func (ps *PathState) PathParam(key, value string) *PathState {
	ps.Params[key] = value
	return ps
}

func (ps *PathState) RoutedState() ([]string, []string) {
	names := make([]string, 0, len(ps.Params))
	values := make([]string, 0, len(ps.Params))

	for k, v := range ps.Params {
		names = append(names, k)
		values = append(values, v)
	}

	return names, values
}

func (ps *PathState) Path() string {
	return ps.URI
}

func (ps *PathState) SetQuery(query url.Values) *PathState {
	ps.URI = fmt.Sprintf("%s?%s", ps.URI, query.Encode())
	return ps
}
