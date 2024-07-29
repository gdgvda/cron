package cron

type entryHeap []*Entry

func (h entryHeap) Len() int      { return len(h) }
func (h entryHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h entryHeap) Less(i, j int) bool {
	// Two zero times should return false.
	// Otherwise, zero is "greater" than any other time.
	if h[i].Next.IsZero() {
		return false
	}
	if h[j].Next.IsZero() {
		return true
	}
	return h[i].Next.Before(h[j].Next)
}

func (h *entryHeap) Push(x any) {
	*h = append(*h, x.(*Entry))
}

func (h *entryHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
