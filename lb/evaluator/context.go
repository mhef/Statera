package evaluator

import (
	"net/http"
)

// ctxEvaluationResultKey is the type used to define the evaluation key.
type ctxEvaluationResultKey struct{}

// evaluationResultKey is the key that holds the evaluation result of the request.
var evaluationResultKey ctxEvaluationResultKey

// EvaluationResultFromRequest returns the evaluation result present in the request
// context, if one.
//
// Returns a bool indicating if the evaluation result was found or not.
//
// The ok bool must be checked before using the evaluation.
func EvaluationResultFromRequest(r *http.Request) (e EvaluationResult, ok bool) {
	e, ok = r.Context().Value(evaluationResultKey).(EvaluationResult)
	return
}
