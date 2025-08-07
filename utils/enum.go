package utils

func CycleEnumPtr[T ~int](current *T, direction int, max T) {
	*current = (*current + T(direction) + max + 1) % (max + 1)
}

func GetNextEnum[T ~int](current T, max T) T {
	next := current + 1
	if next > max {
		return 0
	}
	return next
}

func GetPrevEnum[T ~int](current T, max T) T {
	prev := current - 1
	if prev < 0 {
		return max
	}
	return prev
}
