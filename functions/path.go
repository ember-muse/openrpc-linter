package functions

func resultPath(path string) []string {
	if path == "" {
		return []string{}
	}
	return []string{path}
}
