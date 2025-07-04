package tools

func ZeroIfNil[T any, PT *T](value PT) T {
	if value != nil {
		return *value
	}

	var result T
	return result
}
