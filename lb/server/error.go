package server

import (
	"net/http"
	"text/template"
)

const errorHTML = `<html>
	<body>
		<h1>There was a problem</h1>
		<h3>{{.Msg}}</h3>
	</body>
</html>`

var errorTmpl = template.Must(template.New("error").Parse(errorHTML))

type errorData struct {
	Msg string
}

// WriteError is a standardization func. It should be used by all the application
// to write errors to the client. It will format the message as HTML and write it
// to the ResponseWriter.
func WriteError(w http.ResponseWriter, statusCode int, message string) {
	w.WriteHeader(statusCode)

	if err := errorTmpl.Execute(w, errorData{message}); err != nil {
		panic(err)
	}
}
