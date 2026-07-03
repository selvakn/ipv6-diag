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
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/selvakn/ipv6diag-server/internal/handler"
	"github.com/selvakn/ipv6diag-server/internal/listener"
	"github.com/selvakn/ipv6diag-server/internal/store"
	turnsvc "github.com/selvakn/ipv6diag-server/internal/turn"
)

var version = "dev"

func main() {
	httpPort := envOrDefault("APP_HTTP_PORT", "80")
	httpAddr := flag.String("http-addr", "0.0.0.0:"+httpPort, "IPv4 HTTP listen address")
	http6Addr := flag.String("http6-addr", "[::]:"+httpPort, "IPv6 HTTP listen address")
	httpsAddr := flag.String("https-addr", "0.0.0.0:443", "IPv4 HTTPS listen address")
	https6Addr := flag.String("https6-addr", "[::]:443", "IPv6 HTTPS listen address")
	certFile := flag.String("cert", "", "Path to TLS certificate file (PEM)")
	keyFile := flag.String("key", "", "Path to TLS private key file (PEM)")
	dbPath := flag.String("db", "./reports.db", "Path to SQLite database file")
	certmagicDir := flag.String("certmagic-dir", "./certmagic-data", "Directory for CertMagic certificate storage (used when HTTPS_HOST is set)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	httpsHost := os.Getenv("HTTPS_HOST")
	httpsHosts := splitHosts(httpsHost)

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
	browserDiagPageHandler := &handler.BrowserDiagnosticsPageHandler{}
	browserDiagConfigHandler := &handler.BrowserDiagnosticsConfigHandler{}
	turnCfg := turnsvc.LoadConfigFromEnv()
	turnCredentials := turnsvc.NewCredentialManager(turnCfg.Realm, turnCfg.CredentialTTL)
	turnService := turnsvc.NewService(turnCfg, turnCredentials)
	if err := turnService.Start(); err != nil {
		log.Fatalf("failed to start TURN service: %v", err)
	}
	defer func() {
		if err := turnService.Stop(); err != nil {
			log.Printf("failed to stop TURN service: %v", err)
		}
	}()
	turnHandler := &handler.TurnCredentialsHandler{
		Token:       turnCfg.CredentialsToken,
		Credentials: turnCredentials,
		Service:     turnService,
	}

	// Store the published APK alongside the database on the data volume.
	apkHandler := &handler.APKHandler{
		Dir:         filepath.Dir(*dbPath),
		UploadToken: os.Getenv("APK_UPLOAD_TOKEN"),
	}

	// Build application muxes
	httpMux := http.NewServeMux()
	httpMux.Handle("/", browserDiagPageHandler)
	httpMux.Handle("/diag", &handler.DiagHandler{IsTLS: false})
	httpMux.Handle("/health", &handler.HealthHandler{})
	httpMux.Handle("/api/reports", reportsHandler)
	httpMux.Handle("/api/reports/", reportsHandler)
	httpMux.Handle("/reports", &handler.DashboardHandler{})
	httpMux.Handle("/reports/", &handler.DashboardHandler{})
	httpMux.Handle("/upload-apk", apkHandler)
	httpMux.Handle("/download/apk", apkHandler)
	httpMux.Handle("/apk-info", apkHandler)
	httpMux.Handle("/turn/credentials", turnHandler)
	httpMux.Handle("/browser-diagnostics", browserDiagPageHandler)
	httpMux.Handle("/browser-diagnostics/config", browserDiagConfigHandler)
	httpMux.Handle("/my-ip", &handler.MyIPHandler{})

	tlsMux := http.NewServeMux()
	tlsMux.Handle("/", browserDiagPageHandler)
	tlsMux.Handle("/diag", &handler.DiagHandler{IsTLS: true})
	tlsMux.Handle("/health", &handler.HealthHandler{})
	tlsMux.Handle("/api/reports", reportsHandler)
	tlsMux.Handle("/api/reports/", reportsHandler)
	tlsMux.Handle("/reports", &handler.DashboardHandler{})
	tlsMux.Handle("/reports/", &handler.DashboardHandler{})
	tlsMux.Handle("/upload-apk", apkHandler)
	tlsMux.Handle("/download/apk", apkHandler)
	tlsMux.Handle("/apk-info", apkHandler)
	tlsMux.Handle("/turn/credentials", turnHandler)
	tlsMux.Handle("/browser-diagnostics", browserDiagPageHandler)
	tlsMux.Handle("/browser-diagnostics/config", browserDiagConfigHandler)
	tlsMux.Handle("/my-ip", &handler.MyIPHandler{})

	// Resolve TLS config: CertMagic (HTTPS_HOST) > manual cert/key > none
	var tlsCfg *tls.Config
	var httpHandler http.Handler = httpMux

	if len(httpsHosts) > 0 {
		certmagic.DefaultACME.Agreed = true
		certmagic.Default.Storage = &certmagic.FileStorage{Path: *certmagicDir}
		magic := certmagic.NewDefault()

		// Wrap HTTP handler so the ACME issuer can serve HTTP-01 challenge tokens
		if am, ok := magic.Issuers[0].(*certmagic.ACMEIssuer); ok {
			httpHandler = am.HTTPChallengeHandler(httpMux)
		}

		ctx := context.Background()
		if err := magic.ManageSync(ctx, httpsHosts); err != nil {
			log.Fatalf("CertMagic failed to obtain certificates for %v: %v", httpsHosts, err)
		}
		tlsCfg = magic.TLSConfig()
		log.Printf("CertMagic: managing certificates for %v", httpsHosts)
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
	if turnCfg.Enabled {
		log.Printf("TURN enabled (realm=%s, active_leases=%d)", turnCfg.Realm, turnCredentials.ActiveCount())
	} else {
		log.Println("TURN disabled")
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

func envOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

// splitHosts splits a comma-separated list of hostnames and returns the
// non-empty trimmed entries. Accepts a single hostname for backward compat.
func splitHosts(s string) []string {
	var out []string
	for _, h := range strings.Split(s, ",") {
		h = strings.TrimSpace(h)
		if h != "" {
			out = append(out, h)
		}
	}
	return out
}
