package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/lenovo/mesh/ipv6diag-server/internal/handler"
	"github.com/lenovo/mesh/ipv6diag-server/internal/listener"
	"github.com/lenovo/mesh/ipv6diag-server/internal/store"
)

var version = "dev"

func main() {
	httpAddr := flag.String("http-addr", "0.0.0.0:80", "IPv4 HTTP listen address")
	http6Addr := flag.String("http6-addr", "[::]:80", "IPv6 HTTP listen address")
	httpsAddr := flag.String("https-addr", "0.0.0.0:443", "IPv4 HTTPS listen address")
	https6Addr := flag.String("https6-addr", "[::]:443", "IPv6 HTTPS listen address")
	certFile := flag.String("cert", "", "Path to TLS certificate file (PEM)")
	keyFile := flag.String("key", "", "Path to TLS private key file (PEM)")
	dbPath := flag.String("db", "./reports.db", "Path to SQLite database file")
	certmagicDir := flag.String("certmagic-dir", "./certmagic-data", "Directory for CertMagic certificate storage (used when HTTPS_HOST is set)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	httpsHost := os.Getenv("HTTPS_HOST")

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	reportStore := store.NewReportStore(db)
	reportsHandler := &handler.ReportsHandler{Store: reportStore}

	// Build application muxes
	httpMux := http.NewServeMux()
	httpMux.Handle("/diag", &handler.DiagHandler{IsTLS: false})
	httpMux.Handle("/health", &handler.HealthHandler{})
	httpMux.Handle("/reports", reportsHandler)
	httpMux.Handle("/reports/", reportsHandler)
	httpMux.Handle("/dashboard", &handler.DashboardHandler{})

	tlsMux := http.NewServeMux()
	tlsMux.Handle("/diag", &handler.DiagHandler{IsTLS: true})
	tlsMux.Handle("/health", &handler.HealthHandler{})
	tlsMux.Handle("/reports", reportsHandler)
	tlsMux.Handle("/reports/", reportsHandler)
	tlsMux.Handle("/dashboard", &handler.DashboardHandler{})

	// Resolve TLS config: CertMagic (HTTPS_HOST) > manual cert/key > none
	var tlsCfg *tls.Config
	var httpHandler http.Handler = httpMux

	if httpsHost != "" {
		certmagic.DefaultACME.Agreed = true
		certmagic.Default.Storage = &certmagic.FileStorage{Path: *certmagicDir}
		magic := certmagic.NewDefault()

		// Wrap HTTP handler so the ACME issuer can serve HTTP-01 challenge tokens
		if am, ok := magic.Issuers[0].(*certmagic.ACMEIssuer); ok {
			httpHandler = am.HTTPChallengeHandler(httpMux)
		}

		ctx := context.Background()
		if err := magic.ManageSync(ctx, []string{httpsHost}); err != nil {
			log.Fatalf("CertMagic failed to obtain certificate for %s: %v", httpsHost, err)
		}
		tlsCfg = magic.TLSConfig()
		log.Printf("CertMagic: managing certificate for %s", httpsHost)
	} else if *certFile != "" && *keyFile != "" {
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("Loading TLS certificate: %v", err)
		}
		tlsCfg = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	}

	// Create plain HTTP listeners (always)
	listeners, err := listener.Create(*httpAddr, *http6Addr, *httpsAddr, *https6Addr, "", "")
	if err != nil {
		log.Fatalf("Failed to create listeners: %v", err)
	}
	defer listeners.CloseAll()

	// Create HTTPS listeners if we have a TLS config (either CertMagic or manual)
	var ipv4HTTPS, ipv6HTTPS net.Listener
	if tlsCfg != nil {
		ipv4HTTPS, err = tls.Listen("tcp4", *httpsAddr, tlsCfg)
		if err != nil {
			log.Fatalf("IPv4 HTTPS listener on %s: %v", *httpsAddr, err)
		}
		defer ipv4HTTPS.Close()

		ipv6HTTPS, err = tls.Listen("tcp6", *https6Addr, tlsCfg)
		if err != nil {
			log.Fatalf("IPv6 HTTPS listener on %s: %v", *https6Addr, err)
		}
		defer ipv6HTTPS.Close()
	}

	errCh := make(chan error, 4)

	go func() {
		log.Printf("IPv4 HTTP listening on %s", listeners.IPv4HTTP.Addr())
		errCh <- (&http.Server{Handler: httpHandler}).Serve(listeners.IPv4HTTP)
	}()

	go func() {
		log.Printf("IPv6 HTTP listening on %s", listeners.IPv6HTTP.Addr())
		errCh <- (&http.Server{Handler: httpHandler}).Serve(listeners.IPv6HTTP)
	}()

	if ipv4HTTPS != nil {
		go func() {
			log.Printf("IPv4 HTTPS listening on %s", ipv4HTTPS.Addr())
			errCh <- (&http.Server{Handler: tlsMux}).Serve(ipv4HTTPS)
		}()
	}

	if ipv6HTTPS != nil {
		go func() {
			log.Printf("IPv6 HTTPS listening on %s", ipv6HTTPS.Addr())
			errCh <- (&http.Server{Handler: tlsMux}).Serve(ipv6HTTPS)
		}()
	}

	if httpsHost == "" && (*certFile == "" || *keyFile == "") {
		log.Println("No TLS configured — HTTPS disabled. Set HTTPS_HOST env var for automatic certs.")
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("Received signal %s, shutting down...", sig)
	case err := <-errCh:
		log.Printf("Listener error: %v", err)
	}

	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	listeners.CloseAll()
	log.Println("Server stopped.")
}
