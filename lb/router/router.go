// Package router is the LB component in charge of routing the requests
// to the dest servers.
package router

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"log"

	"github.com/mhef/statera/lb/evaluator"
	"github.com/mhef/statera/lb/server"
)

// NodeKey define the key of a node. It is composed by the tuple host and port
// of the node.
type NodeKey struct {
	Host string
	Port uint16
}

// Node define a node in the context of the router.
type Node struct {
	NodeKey

	// Weight define the weight of the node. The weight may be used by some balancing
	// algorithms that demands it.
	Weight int

	healthCheckerCancel context.CancelFunc
	healthy             bool
	healthMu            sync.Mutex // guards healthCheckerCancel and healthy
}

// Balancer is an interface representing the implementation of a load balancing
// algorithm.
//
// The interface implementation must be safe for concurrent use by multiple
// goroutines.
type Balancer interface {
	// AddNode adds a node on the node balancing pool.
	AddNode(*Node)

	// DeleteNode removes a node from the node balancing pool.
	DeleteNode(NodeKey)

	// Balance return the node for wich the passed request should be sent.
	//
	// Balance should not modify the request.
	//
	// Balance must return nil if there isn't any node available.
	Balance(*http.Request) *Node
}

// HealthCheckConfig define the health check configuration of a node group.
type HealthCheckConfig struct {
	// Path define the path to wich the health check requests should be sent.
	//
	// The default Path is "/"
	Path string

	// Interval define the interval in seconds between each health check
	// request.
	//
	// The default Interval is 5 seconds.
	Interval int

	// Timeout define the time in seconds to a health check request be considered
	// failed.
	//
	// The default Timeout is 3 seconds.
	Timeout int
}

// NodeGroup is a group of node servers that will be balanced.
type NodeGroup struct {
	// Name specifies the name of the group and must be unique.
	Name string

	// HTTPS define if the connections to this group uses HTTPS.
	HTTPS bool

	// HealthCheck define the group configuration for the health check operations.
	HealthCheck HealthCheckConfig

	// Balancer define the load balancing algorithm that will be used to route route
	// requests to this group.
	Balancer Balancer

	nodes   map[NodeKey]*Node
	nodesMu sync.RWMutex

	transport http.RoundTripper
}

// AddNode takes a node and add it to the group, enabling the node to be scheduled
// by the load balancing algorithm to receive requests on the group behalf.
//
// After being added, the node will remain unreachable until it's health be validated
// by the health checking. The health checker will be the responsible for adding the
// node on the balancer.
func (ng *NodeGroup) AddNode(n *Node) {
	if n == nil {
		return
	}

	ng.nodesMu.Lock()
	defer ng.nodesMu.Unlock()
	if ng.nodes == nil {
		ng.nodes = make(map[NodeKey]*Node)
	}
	nk := NodeKey{
		Host: n.Host,
		Port: n.Port,
	}
	ng.nodes[nk] = n

	ng.startNodeHealthChecker(n)
}

// DeleteNode remove the node from the group, disabling the node from receiveing
// new requests.
//
// On-fly requests to the node are not canceled when this func is called.
func (ng *NodeGroup) DeleteNode(nk NodeKey) {
	ng.nodesMu.Lock()
	defer ng.nodesMu.Unlock()
	ng.stopNodeHealthChecker(ng.nodes[nk])
	ng.Balancer.DeleteNode(nk)
	delete(ng.nodes, nk)
}

// startNodeHealthChecker will start the health checker service for the passed
// node. A goroutine will be created and will do periodically health checks, based
// on the group health check configuration.
//
// Also this func is responsable for adding or removing the node from the Balancer,
// depending on the node health. Other funcs should not add or remove the node from
// the balancer during the execution of the health checker.
func (ng *NodeGroup) startNodeHealthChecker(n *Node) {
	n.healthMu.Lock()
	defer n.healthMu.Unlock()
	if n.healthCheckerCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	n.healthCheckerCancel = cancel
	go func() {
		t := time.NewTicker(time.Duration(ng.HealthCheck.Interval) * time.Second)
		for {
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
				ng.checkNodeHealth(ctx, n)
			}
		}
	}()
}

// stopNodeHealthChecker will stop the node health checker service. It will cancel
// the node health checker goroutine context
func (ng *NodeGroup) stopNodeHealthChecker(n *Node) {
	n.healthMu.Lock()
	defer n.healthMu.Unlock()
	if n.healthCheckerCancel == nil {
		return
	}
	n.healthCheckerCancel()
}

// checkNodeHealth will do a HTTP request, based on the group health check
// configuration, to verify the node healthness. If the node is currently unhealthy,
// and the check determines that the node is healthy again, it will be added back
// on the Balancer. The opposite will also happen: healthy node becoming unhealthy
// will be removed from the Balancer.
func (ng *NodeGroup) checkNodeHealth(ctx context.Context, n *Node) {
	scheme := "http"
	if ng.HTTPS {
		scheme = "https"
	}
	ctxT, cancel := context.WithTimeout(ctx, time.Duration(ng.HealthCheck.Timeout)*time.Second)
	defer cancel()
	url := fmt.Sprintf("%s://%s:%d/%s", scheme, n.Host, n.Port, ng.HealthCheck.Path)
	req, err := http.NewRequestWithContext(ctxT, "GET", url, nil)
	if err != nil {
		// We panic here because NewRequestWithContext only return errors on
		// malformed params.
		panic("lb/router: failed to create health check request")
	}

	res, err := ng.transport.RoundTrip(req)
	if res != nil && res.Body != nil {
		defer res.Body.Close()
	}

	// After the roundtrip we verify if the node still is on the group node
	// list. We do this because the roundtrip takes a lot of time (ms scale) and
	// the node can be removed when roundtrip is running.
	//
	// Also, we mantain the lock until the func return, to avoid the node be
	// deleted when the func is still executing.
	ng.nodesMu.Lock()
	defer ng.nodesMu.Unlock()
	if _, ok := ng.nodes[n.NodeKey]; !ok {
		return
	}

	n.healthMu.Lock()
	defer n.healthMu.Unlock()
	if n.healthy && (err != nil || res.StatusCode != 200) {
		n.healthy = false
		ng.Balancer.DeleteNode(n.NodeKey)
		log.Println(n.NodeKey, "is unhealthy")
		return
	}
	if !n.healthy && err == nil && res.StatusCode == 200 {
		n.healthy = true
		ng.Balancer.AddNode(n)
		log.Println(n.NodeKey, "is healthy")
		return
	}
}

var errNoNodeAvailable = errors.New("lb/router: there is no node available on the group")

// roundTrip executes a single HTTP request to a node. The node for wich the
// request will be sent is selected at runtime by the group Balancer.
//
// roundTrip will modify the request URL to adjust the scheme, host and port.
func (ng *NodeGroup) roundTrip(r *http.Request) (*http.Response, error) {
	n := ng.Balancer.Balance(r)
	if n == nil {
		return nil, errNoNodeAvailable
	}

	scheme := "http"
	if ng.HTTPS {
		scheme = "https"
	}

	r.URL.Scheme = scheme
	r.URL.Host = fmt.Sprintf("%s:%d", n.Host, n.Port)

	res, err := ng.transport.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	return res, nil
}

const (
	// maxIdleConns should be at least the maximum number of server nodes
	routerMaxIdleConns          = 30000
	routerMaxIdleConnsPerHost   = 1000
	routerMaxConnsPerHost       = 0
	routerIdleConnTimeout       = 60
	routerDialTimeout           = 15
	routerTLSHandshakeTimeout   = 15
	routerExpectContinueTimeout = 1
	routerRequestTimeout        = 30
)

// Router define the router component of the load balancer. This struct holds
// the node groups and handle the request balancing process.
type Router struct {
	ng map[string]*NodeGroup
}

// New returns an initialized instance of Router.
func New(ng []*NodeGroup) *Router {
	r := &Router{
		ng: make(map[string]*NodeGroup),
	}

	for _, n := range ng {
		n.transport = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: time.Second * routerDialTimeout,
			}).DialContext,
			MaxIdleConns:          routerMaxIdleConns,
			MaxIdleConnsPerHost:   routerMaxIdleConnsPerHost,
			MaxConnsPerHost:       routerMaxConnsPerHost,
			IdleConnTimeout:       time.Second * routerIdleConnTimeout,
			TLSHandshakeTimeout:   time.Second * routerTLSHandshakeTimeout,
			ExpectContinueTimeout: time.Second * routerExpectContinueTimeout,
		}

		r.ng[n.Name] = n
	}
	return r
}

var (
	errNoNodeGroupFromEvaluation = errors.New("lb/router: there is no node group on the evaluation context")
	errNodeGroupNotFound         = errors.New("lb/router: node group from the evaluation context not found on router")
)

// Handler handle the requests that arrives on the router. It will verify the
// evaluation result and then foward the request to the selected node group, using
// the group chosen balancing algorithm.
func (rtr *Router) Handler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		e, ok := evaluator.EvaluationResultFromRequest(r)
		if !ok {
			log.Println(errNoNodeGroupFromEvaluation)
			server.WriteError(w, http.StatusInternalServerError, "")
			return
		}
		if _, ok := rtr.ng[e.NodeGroup]; !ok {
			log.Println(errNodeGroupNotFound)
			server.WriteError(w, http.StatusInternalServerError, "")
			return
		}

		reqOut := r.Clone(r.Context())
		reqOut.Close = false
		if reqOut.Body != nil {
			defer reqOut.Body.Close()
		}

		res, err := rtr.ng[e.NodeGroup].roundTrip(reqOut)
		if err != nil {
			log.Println(err)
			server.WriteError(w, http.StatusBadGateway, "bad gateway")
			return
		}
		defer res.Body.Close()

		// copy headers
		for k, vv := range res.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}

		// write status code
		w.WriteHeader(res.StatusCode)

		// copy body
		io.Copy(w, res.Body)

		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
