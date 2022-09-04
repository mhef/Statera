// Package evaluator is the LB component in charge of evaluating the user rules
// over each request.
package evaluator

import (
	"context"
	"log"
	"net/http"
	"sort"
	"sync"

	"github.com/mhef/statera/lb/server"
)

// Condition define a condition for a rule.
type Condition struct {
	// Not negates the condition result.
	Not bool

	// Type define wich type of data will be compared.
	Type CondType

	// Key define the key that will be compared, on types that have keys.
	Key string

	// Operation define the comparison operation that will be made.
	Operation CondOp

	// Value define the value waited to the condition be satisfied.
	Value string
}

// Action define the behaviour that should be taken if a rule is satisfied.
type Action struct {
	// NodeGroup indicate that the request should be fowarded to this group.
	NodeGroup string

	// Reject indicate that the request should be negated.
	Reject struct {
		StatusCode int
		Message    string
	}

	// Redirect indicate that the client will be redirect to this address.
	Redirect string
}

// Rule define a rule that will be evaluated by the evaluator.
type Rule struct {
	Priority   int
	Listener   string
	Conditions []Condition
	Action     Action
	Dynamic    string
}

// Evaluator is the component in charge of evaluating each request, using the
// rules defined before by the LB admin.
type Evaluator struct {
	r  []*Rule
	mu sync.RWMutex
}

// New return a new instance of Evaluator.
func New() *Evaluator {
	return &Evaluator{}
}

// AddRule adds the provided rule to the Evaluator.
func (e *Evaluator) AddRule(r *Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.r = append(e.r, r)
	sort.SliceStable(e.r, func(i, j int) bool {
		return e.r[i].Priority < e.r[j].Priority
	})
}

// DeleteRule deletes the provided rule from the Evaluator.
func (e *Evaluator) DeleteRule(r *Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, v := range e.r {
		if v == r {
			e.r = append(e.r[:i], e.r[i+1:]...)
			return
		}
	}
}

// evaluateRequest takes a request and then evaluate all rules present on the
// Evaluator until a match, then return the Action of the matched rule. A rule
// is considered satisfied, if all of it's conditions are satisfied.
func (e *Evaluator) evaluateRequest(r *http.Request) (Action, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, rule := range e.r {
		lnr, ok := server.ListenerFromRequest(r)
		if !ok || lnr != rule.Listener {
			continue
		}
		allCondsTrue := true
		for _, cnd := range rule.Conditions {
			ret, err := evaluateCondition(r, cnd)
			if err != nil {
				return Action{}, err
			}
			if !ret {
				allCondsTrue = false
				break
			}
		}
		if allCondsTrue {
			return rule.Action, nil
		}
	}

	// if the code execution reach this point, it means that no rule was satisfied.
	return Action{
		Reject: struct {
			StatusCode int
			Message    string
		}{
			StatusCode: 500,
			Message:    "no rule was satisfied",
		},
	}, nil
}

// EvaluationResult hold the evaluation result of a request that evaluated to be
// fowarded.
type EvaluationResult struct {
	NodeGroup string
}

// Handler will evaluate each request with the Evaluator rules and then will take
// the action of the matched rule.
func (e *Evaluator) Handler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		a, err := e.evaluateRequest(r)
		if err != nil {
			log.Println(err)
			server.WriteError(w, http.StatusBadGateway, "rule evaluation failed")
			return
		}

		if a.NodeGroup != "" {
			ctx := r.Context()
			ctx = context.WithValue(ctx, evaluationResultKey, EvaluationResult{
				NodeGroup: a.NodeGroup,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		if a.Reject.StatusCode != 0 {
			server.WriteError(w, a.Reject.StatusCode, a.Reject.Message)
			return
		}

		if a.Redirect != "" {
			http.Redirect(w, r, a.Redirect, http.StatusFound)
			return
		}

		server.WriteError(w, http.StatusBadGateway, "rule matched has no action")
	}
	return http.HandlerFunc(fn)
}
