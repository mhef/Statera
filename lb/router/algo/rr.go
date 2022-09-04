package algo

import (
	"container/list"
	"net/http"
	"sync"

	"github.com/mhef/statera/lb/router"
)

// RR define the round-robin load balancing algorithm implementation.
type RR struct {
	// nodes is a linked list that hold the nodes
	nodes *list.List
	// cur is a cursor that hold the next node that will be returned by the algorithm.
	cur *list.Element

	mu sync.Mutex
}

// NewRR return an initialized round-robin balancer.
func NewRR() *RR {
	return &RR{
		nodes: list.New(),
	}
}

// AddNode takes a node and adds it to the balance list.
func (r *RR) AddNode(n *router.Node) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes.PushBack(n)
}

// DeleteNode removes the node from the balance list.
func (r *RR) DeleteNode(k router.NodeKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for e := r.nodes.Front(); e != nil; e = e.Next() {
		v := e.Value.(*router.Node)
		if k != v.NodeKey {
			continue
		}

		// if the element to be deleted is the element to wich the cursor is
		// appointing, the cursor need to be updated.
		if e == r.cur {
			if r.nodes.Len() == 1 {
				r.cur = nil
			} else {
				if e.Next() == nil {
					r.cur = r.nodes.Front()
				} else {
					r.cur = e.Next()
				}
			}
		}

		r.nodes.Remove(e)
		return
	}
}

// Balance return the node for wich the next request should be sent.
func (r *RR) Balance(*http.Request) *router.Node {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.nodes.Len() == 0 {
		return nil
	}

	if r.cur == nil {
		r.cur = r.nodes.Front()
	}

	v := r.cur
	r.cur = r.cur.Next()
	return v.Value.(*router.Node)
}
