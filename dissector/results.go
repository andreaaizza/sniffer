package dissector

import "fmt"

func (r *Result) PrettyString() string {
	return fmt.Sprintf("%s => %s", r.GetRequest().PrettyString(), r.GetReponse().PrettyString())
}
