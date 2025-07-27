package main

import "sync"

/*

Methods:
   Enqueue(item) // add to end of queue
   Dequeue(item) // remove from head
   Peek() item // return head without removing it
   Size() int // number of elements
   IsEmpty() bool //

Requirements:
   - methods from above
   - thread safe is matter
   - generics if you like
   - O(1)
*/

type Queue[T any] struct {
	mu   sync.RWMutex
	size int

	head *item[T]
	tail *item[T]
}

type item[T any] struct {
	value T
	next  *item[T]
}

func (q *Queue[T]) IsEmpty() bool { return q.Size() == 0 }
func (q *Queue[T]) Size() int     { q.mu.RLock(); defer q.mu.RUnlock(); return q.size }

func (q *Queue[T]) Peek() (T, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.head != nil {
		return q.head.value, true
	}

	var out T
	return out, false
}

func (q *Queue[T]) Dequeue() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.head == nil {
		var out T
		return out, false
	}
	q.size--

	out := q.head.value
	q.head = q.head.next

	if q.head.next == nil {
		q.tail = nil
	}

	return out, true
}

func (q *Queue[T]) Enqueue(val T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	it := &item[T]{value: val}
	q.size++

	if q.head == nil {
		q.head = it
		q.tail = it
	} else {
		q.tail.next = it
		q.tail = it
	}
}

func main() {
}
