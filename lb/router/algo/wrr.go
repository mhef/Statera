package algo

import (
	"container/list"
	"net/http"
	"sync"

	"github.com/mhef/statera/lb/router"
)

// WRR define the weighted round-robin load balancing algorithm implementation.
type WRR struct {
	// nodes is a linked list that hold the nodes being currently balanced by this
	// algorithm.
	nodes *list.List

	// cur is a cursor that hold the next node that will be returned by the algorithm.
	cur *list.Element

	// reqCredit hold the current number of requests available for the node
	// appointed to by cur.
	reqCredit int

	mu sync.Mutex
}

// NewWRR return an initialized weighted round-robin balancer.
func NewWRR() *WRR {
	return &WRR{
		nodes: list.New(),
	}
}

// AddNode takes a node and adds it to the balancing list.
func (r *WRR) AddNode(n *router.Node) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes.PushBack(n)
}

// DeleteNode removes a node from the balancing list.
func (r *WRR) DeleteNode(k router.NodeKey) {
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
					r.reqCredit = r.cur.Value.(*router.Node).Weight
				} else {
					r.cur = r.cur.Next()
					r.reqCredit = r.cur.Value.(*router.Node).Weight
				}
			}
		}

		r.nodes.Remove(e)
		return
	}
}

// Balance return the node for wich the next request should be sent.
func (r *WRR) Balance(*http.Request) *router.Node {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.nodes.Len() == 0 {
		return nil
	}

	if r.cur == nil {
		if r.nodes.Front().Value.(*router.Node).Weight <= 1 {
			r.cur = r.nodes.Front().Next()
			if r.cur != nil {
				r.reqCredit = r.cur.Value.(*router.Node).Weight
			} else {
				r.reqCredit = 0
			}
		} else {
			r.cur = r.nodes.Front()
			r.reqCredit = r.nodes.Front().Value.(*router.Node).Weight - 1
		}

		return r.nodes.Front().Value.(*router.Node)
	}

	v := r.cur
	r.reqCredit--
	if r.reqCredit < 1 {
		r.cur = r.cur.Next()
		if r.cur != nil {
			r.reqCredit = r.cur.Value.(*router.Node).Weight
		} else {
			r.reqCredit = 0
		}
	}
	return v.Value.(*router.Node)
}
