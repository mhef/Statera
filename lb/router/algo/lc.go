package algo

import (
	"container/heap"
	"net/http"
	"sync"

	"github.com/mhef/statera/lb/router"
)

// nodeWR is a type that hold a node along with it's current number of on-fly
// requests and it's index on the heap.
type nodeWR struct {
	node *router.Node

	// reqs hold the number of requests currently on-fly to the node
	reqs int

	// index hold the index of the node item in the heap.
	index int
}

// nodeHeap implements the heap.Interface. It is a heap binary tree that holds
// the nodes ordered by the number of on-fly requests.
type nodeHeap []*nodeWR

// Len return the current len of the heap.
func (nh nodeHeap) Len() int { return len(nh) }

// Less takes two node indexes from the heap and return if the first node has
// less on-fly requests than the other.
func (nh nodeHeap) Less(i, j int) bool {
	return nh[i].reqs < nh[j].reqs
}

// Swap takes two node indexes from the heap and swap them on the heap array.
func (nh nodeHeap) Swap(i, j int) {
	nh[i], nh[j] = nh[j], nh[i]
	nh[i].index = i
	nh[j].index = j
}

// Push takes a nodeWR and pushes it to the end of the heap array.
func (nh *nodeHeap) Push(x any) {
	n := len(*nh)
	item := x.(*nodeWR)
	item.index = n
	*nh = append(*nh, item)
}

// Pop return and remove the last element of the heap array.
func (nh *nodeHeap) Pop() any {
	old := *nh
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*nh = old[0 : n-1]
	return item
}

// LC define the least-connections load balancing algorithm implementation.
type LC struct {
	nodes nodeHeap
	mu    sync.Mutex
}

// NewLC return an initialized least-connections balancer.
func NewLC() *LC {
	return &LC{
		nodes: make(nodeHeap, 0),
	}
}

// AddNode takes a node and adds it in the balancing list.
func (l *LC) AddNode(n *router.Node) {
	l.mu.Lock()
	defer l.mu.Unlock()
	nwr := &nodeWR{
		node:  n,
		reqs:  0,
		index: -1,
	}
	heap.Push(&l.nodes, nwr)
}

// DeleteNode removes the node from the balance list.
func (l *LC) DeleteNode(k router.NodeKey) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, v := range l.nodes {
		if k != v.node.NodeKey {
			continue
		}
		heap.Remove(&l.nodes, v.index)
		return
	}
}

// Balance return the node for wich the next request should be sent.
func (l *LC) Balance(r *http.Request) *router.Node {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.nodes.Len() == 0 {
		return nil
	}

	selected := l.nodes[0]
	selected.reqs++
	heap.Fix(&l.nodes, 0)
	go l.monitorRequestFinish(r, selected)
	return selected.node
}

func (l *LC) monitorRequestFinish(r *http.Request, n *nodeWR) {
	done := r.Context().Done()
	if done != nil {
		<-done
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	n.reqs--
	heap.Fix(&l.nodes, n.index)
}
