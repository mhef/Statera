// Package server is responsible for managing the frontend part of the load balancer.
// It will manage the listeners that are in charge of receiving the requests and
// will answer that requests when the response is made available by the other statera
// processes.
package server

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const shutdownTimeout = 30

// Certificate define a type that hold the certificate and key files for use on
// TLS.
type Certificate struct {
	CertFile string
	KeyFile  string
}

// TLS define specific configurations for TLS.
type TLS struct {
	// Certs hold the certificates of the listener.
	Certs []Certificate

	// MinTLSVersion define the minimum TLS version supported by the listener.
	// If zero, TLS 1.0 is the default.
	MinTLSVersion uint16

	// MaxTLSVersion define the maximum TLS version supported by the listener.
	// If zero, TLS 1.3 is the default.
	MaxTLSVersion uint16
}

// Listener is, essentially, a opened port on the server that will wait for
// connections and requests.
type Listener struct {
	// Addr specifies the TCP address for the listener to listen on, in the form
	// "host:port". If blank, :http will be used for non-TLS listeners and :https
	// for TLS listeners.
	Addr string

	// Handler define the handler to invoke when responding to HTTP requests.
	Handler http.Handler

	// HTTP2 define if the support for HTTP2 should be enabled for this listener.
	// HTTPS needed.
	HTTP2 bool

	// TLS specifies the TLS configurations of the listener.
	//
	// If TLS.Certificates has at least one certificate, the listener will use HTTPS.
	//
	// If no certificate is supplied, HTTP/2 will not be enabled.
	TLS *TLS

	server *http.Server
}

// handler wraps Listener.Handler to add the Listener addr on the request context.
func (l *Listener) handler() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = context.WithValue(ctx, listenerKey, l.Addr)
		l.Handler.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// ListenAndServe will setup and start a HTTP server for the listener and will
// begin to serve to requests.
//
// This func blocks until a shutdown signal is received by the application.
func (l *Listener) ListenAndServe() error {
	// setup TLS config
	tCfg := &tls.Config{}
	if l.TLS != nil && l.TLS.Certs != nil {
		tCfg.MinVersion = l.TLS.MinTLSVersion
		tCfg.MaxVersion = l.TLS.MaxTLSVersion

		for _, c := range l.TLS.Certs {
			cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
			if err != nil {
				return err
			}
			tCfg.Certificates = append(tCfg.Certificates, cert)
		}
	}

	l.server = &http.Server{
		Addr:      l.Addr,
		Handler:   l.handler(),
		TLSConfig: tCfg,
	}

	if !l.HTTP2 {
		// Disable the HTTP2 support for the server.
		l.server.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler))
	}

	go func() {
		if len(tCfg.Certificates) > 0 {
			if err := l.server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
				panic(err)
			}
			return
		}

		if err := l.server.ListenAndServe(); err != http.ErrServerClosed {
			panic(err)
		}
	}()

	if err := l.waitForShutdown(); err != nil {
		return err
	}
	return nil
}

// waitForShutdown waits for an interrupt signal and gracefully shutdown
// the HTTP server of the listener when one occurs.
//
// The func blocks and only return when the HTTP server has been completely shut
// down.
func (l *Listener) waitForShutdown() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout*time.Second)
	defer cancel()

	l.server.SetKeepAlivesEnabled(false)

	if err := l.server.Shutdown(ctx); err != nil {
		panic(err)
	}
	return nil
}
