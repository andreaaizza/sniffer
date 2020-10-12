package dissector

import "fmt"

func (r *Result) PrettyString() string {
	return fmt.Sprintf("%s", r.GetAdu().PrettyString())
}
