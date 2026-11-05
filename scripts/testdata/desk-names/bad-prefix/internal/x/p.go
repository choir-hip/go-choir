package x

import "strings"

func isTask(id string) bool { return strings.HasPrefix(id, "management-task:") }
