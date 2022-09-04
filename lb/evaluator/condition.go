package evaluator

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
)

// CondType is a type used to define condition types.
type CondType int

// Currently implemented conditions types.
const (
	Path CondType = iota
	Query
	BodyString
	BodyForm
	Header
	IP
)

// CondOp is a type used to define condition operations.
type CondOp int

// Currently implemented conditions operations.
const (
	Equal CondOp = iota
	BeginWith
	Regex
	Range
)

// doStrCondOp is a generic help function that do comparison operations between two
// strings. The func is case insensitive.
//
// It compare the following values of CondOp: Equal, BeginWith and Regex.
//
// On the BeginWith operation, it will verify if the a string begins with b.
// On the Regex operation, it will verify if the regex pattern b matches the
// string a.
func doStrCondOp(op CondOp, a string, b string) (bool, error) {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	switch op {
	case Equal:
		return a == b, nil
	case BeginWith:
		return strings.HasPrefix(a, b), nil
	case Regex:
		return regexp.MatchString(b, a)
	}
	return false, errors.New("evaluator/condition: invalid operation for string type")
}

// evaluateCondPath takes a request and a condition and uses the request path
// to evaluate the condition.
func evaluateCondPath(r *http.Request, c Condition) (bool, error) {
	p := r.URL.EscapedPath()
	return doStrCondOp(c.Operation, p, c.Value)
}

// evaluateCondQuery takes a request and a condition and uses the request query
// to evaluate the condition.
func evaluateCondQuery(r *http.Request, c Condition) (bool, error) {
	q := r.URL.Query()
	if _, ok := q[c.Key]; !ok {
		return false, nil
	}
	return doStrCondOp(c.Operation, q[c.Key][0], c.Value)
}

// evaluateCondQuery takes a request and a condition and uses the request body
// as a string to evaluate the condition.
func evaluateCondBodyString(r *http.Request, c Condition) (bool, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return false, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body)) // reset request body buffer
	fmt.Println(string(body))
	return doStrCondOp(c.Operation, string(body), c.Value)
}

// evaluateCondBodyForm takes a request and a condition and uses the request body
// as a form to evaluate the condition.
func evaluateCondBodyForm(r *http.Request, c Condition) (bool, error) {
	if err := r.ParseForm(); err != nil {
		return false, err
	}
	if _, ok := r.PostForm[c.Key]; !ok {
		return false, nil
	}
	return doStrCondOp(c.Operation, r.PostForm[c.Key][0], c.Value)
}

// evaluateCondBodyForm takes a request and a condition and uses the request header
// to evaluate the condition.
func evaluateCondHeader(r *http.Request, c Condition) (bool, error) {
	if _, ok := r.Header[c.Key]; !ok {
		return false, nil
	}
	return doStrCondOp(c.Operation, r.Header[c.Key][0], c.Value)
}

// evaluateCondIP takes a request and a condition and uses the request client IP to
// evaluate the condition.
func evaluateCondIP(r *http.Request, c Condition) (bool, error) {
	if c.Operation != Range {
		return false, errors.New("evaluator/condition: invalid operation for IP type")
	}
	_, ipNet, err := net.ParseCIDR(c.Value)
	if err != nil {
		return false, err
	}
	rIP := strings.Split(r.RemoteAddr, ":") // remove the port
	ip := net.ParseIP(rIP[0])
	return ipNet.Contains(ip), nil
}

// evaluateCondition takes a Request and a Condition and then evaluate the
// condition over the Request.
func evaluateCondition(r *http.Request, c Condition) (ret bool, err error) {
	switch c.Type {
	case Path:
		ret, err = evaluateCondPath(r, c)
	case Query:
		ret, err = evaluateCondQuery(r, c)
	case BodyString:
		ret, err = evaluateCondBodyString(r, c)
	case BodyForm:
		ret, err = evaluateCondBodyForm(r, c)
	case Header:
		ret, err = evaluateCondHeader(r, c)
	case IP:
		ret, err = evaluateCondIP(r, c)
	}
	ret = ret != c.Not // ret != c.Not  ==  ret XOR c.Not
	return
}
