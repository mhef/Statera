// Package lb is the backbone of statera. The package is in charge of start all
// services and of tie everything up.
package lb

import (
	"fmt"
	"sync"

	"github.com/mhef/statera/cfg"
	"github.com/mhef/statera/lb/evaluator"
	"github.com/mhef/statera/lb/router"
	"github.com/mhef/statera/lb/router/algo"
	"github.com/mhef/statera/lb/server"
)

// listenerControl takes a Mux and a slice of cfg.Listener and start each listener,
// attaching the Mux as the handler of the listeners.
//
// This func blocks, returning only when a kill signal is received by the program.
func listenerControl(m *Mux, cfgLnr []cfg.Listener) {
	// Create each listener
	listeners := make([]*server.Listener, 0)
	for _, l := range cfgLnr {
		serverLnr := &server.Listener{
			Addr:    l.Addr,
			Handler: m,
			HTTP2:   l.HTTP2,
		}
		if l.TLS != nil && len(l.TLS.Certs) > 0 {
			// If cfg.Listener has TLS config, import that config.
			serverLnr.TLS = &server.TLS{
				MinTLSVersion: l.TLS.MinTLSVersion,
				MaxTLSVersion: l.TLS.MaxTLSVersion,
			}
			serverLnr.TLS.Certs = make([]server.Certificate, 0)
			for _, cert := range l.TLS.Certs {
				serverLnr.TLS.Certs = append(serverLnr.TLS.Certs, server.Certificate{
					CertFile: cert.CertFile,
					KeyFile:  cert.KeyFile,
				})
			}
		}
		listeners = append(listeners, serverLnr)
	}

	// start each listener and wait indefinitely until all of them are shut down.
	var wg sync.WaitGroup
	wg.Add(len(listeners))
	for _, l := range listeners {
		go func(il *server.Listener) {
			defer wg.Done()
			if err := il.ListenAndServe(); err != nil {
				panic(err)
			}
		}(l)
	}
	wg.Wait()
}

// evaluatorControl takes a mux and a slice of cfg.Rule, then create the evaluator
// and attachs it's handler on the mux chain.
func evaluatorControl(m *Mux, cfgRules []cfg.Rule) {
	e := evaluator.New()
	for _, rCfg := range cfgRules {
		r := &evaluator.Rule{
			Priority: rCfg.Priority,
			Listener: rCfg.Listener,
			Action:   evaluator.Action(rCfg.Action),
			Dynamic:  rCfg.Dynamic,
		}
		r.Conditions = make([]evaluator.Condition, 0, len(rCfg.Conditions))
		for _, c := range rCfg.Conditions {
			r.Conditions = append(r.Conditions, evaluator.Condition{
				Not:       c.Not,
				Type:      evaluator.CondType(c.Type),
				Key:       c.Key,
				Operation: evaluator.CondOp(c.Operation),
				Value:     c.Value,
			})
		}
		e.AddRule(r)
	}
	m.Chain(e.Handler)
}

func routerControl(m *Mux, cfgNgs []cfg.NodeGroup) {
	rNgs := make([]*router.NodeGroup, 0, len(cfgNgs))
	for _, cfgNg := range cfgNgs {
		var balancer router.Balancer

		switch cfgNg.Algorithm {
		case "rr":
			balancer = algo.NewRR()
		case "wrr":
			balancer = algo.NewWRR()
		case "lc":
			balancer = algo.NewLC()
		default:
			panic(fmt.Sprintf("invalid load balancing algorithm %s", cfgNg.Algorithm))
		}

		rNg := &router.NodeGroup{
			Name:        cfgNg.Name,
			HTTPS:       cfgNg.HTTPS,
			Balancer:    balancer,
			HealthCheck: router.HealthCheckConfig(cfgNg.HealthCheck),
		}

		for _, n := range cfgNg.Nodes {
			rNg.AddNode(&router.Node{
				NodeKey: router.NodeKey{
					Host: n.Host,
					Port: n.Port,
				},
				Weight: n.Weight,
			})
		}

		rNgs = append(rNgs, rNg)
	}

	r := router.New(rNgs)
	m.Chain(r.Handler)
}

// Start the statera load balancer.
func Start(c *cfg.Config) {
	m := NewMux()
	evaluatorControl(m, c.Rules)
	routerControl(m, c.NodeGroups)

	// listenerControl blocks until server shutdown...
	listenerControl(m, c.Listeners)
}
