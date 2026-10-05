package queue

type GenericList[T any] struct {
	Items    []T
	LessFunc func(a, b T) bool
}

func (l GenericList[T]) Len() int {
	return len(l.Items)
}

func (l GenericList[T]) Less(i, j int) bool {
	return l.LessFunc(l.Items[i], l.Items[j])
}

func (l GenericList[T]) Swap(i, j int) {
	l.Items[i], l.Items[j] = l.Items[j], l.Items[i]
}

func (l *GenericList[T]) Push(x any) {
	l.Items = append(l.Items, x.(T))
}

func (l *GenericList[T]) Pop() any {
	old := l.Items
	n := len(old)

	item := old[n-1]
	l.Items = old[:n-1]

	return item
}
