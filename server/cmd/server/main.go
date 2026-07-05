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
	"github.com/selvakn/ipv6diag-server/internal/tlsutil"
	wgsvc "github.com/selvakn/ipv6diag-server/internal/wireguard"
)

var version = "dev"

func main() {
	httpPort := envOrDefault("APP_HTTP_PORT", "80")
	httpAddr := flag.String("http-addr", "0.0.0.0:"+httpPort, "IPv4 HTTP listen address")
	http6Addr := flag.String("http6-addr", "[::]:"+httpPort, "IPv6 HTTP listen address")
	httpsAddr := flag.String("https-addr", "0.0.0.0:443", "IPv4 HTTPS listen address")
	https6Addr := flag.String("https6-addr", "[::]:443", "IPv6 HTTPS listen address")
	certFile := flag.String("cert", "", "Path to TLS certificate file (PEM); enables both HTTPS and TURNS")
	keyFile := flag.String("key", "", "Path to TLS private key file (PEM); enables both HTTPS and TURNS")
	// --tls-certs: multiple cert/key pairs for self-managed HTTPS + TURNS.
	// Right cert selected per-handshake via SNI. Enables both HTTPS (port 443)
	// and TURNS (port 5349).
	tlsCerts := flag.String("tls-certs", "", "Comma-separated cert:key pairs for multi-domain HTTPS+TURNS (SNI selection)")
	// --turn-tls-certs: like --tls-certs but only activates TURNS (port 5349).
	// Use this when a reverse proxy (e.g. Caddy) handles HTTPS so the server
	// must NOT attempt to bind port 443 itself.
	turnTLSCerts := flag.String("turn-tls-certs", "", "Comma-separated cert:key pairs for TURNS only — does not bind HTTPS port 443")
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

	// Store the published APK alongside the database on the data volume.
	apkHandler := &handler.APKHandler{
		Dir:         filepath.Dir(*dbPath),
		UploadToken: os.Getenv("APK_UPLOAD_TOKEN"),
	}

	// Build plain-HTTP mux first so the ACME challenge handler can wrap it.
	httpMux := http.NewServeMux()

	// tlsCfg drives HTTPS listeners on port 443.
	// turnTLSCfg drives TURNS listeners on port 5349.
	// They are the same when the server manages its own TLS (HTTPS_HOST,
	// --cert/--key, --tls-certs). They differ when a reverse proxy owns HTTPS:
	// in that case --turn-tls-certs sets only turnTLSCfg, leaving tlsCfg nil
	// so the server never attempts to bind port 443.
	var tlsCfg *tls.Config     // for HTTPS
	var turnTLSCfg *tls.Config // for TURNS
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
		turnTLSCfg = tlsCfg
		log.Printf("CertMagic: managing certificates for %v", httpsHosts)
	} else if *tlsCerts != "" {
		// Self-managed multi-domain: binds both HTTPS (443) and TURNS (5349).
		pairs, err := parseCertPairs(*tlsCerts)
		if err != nil {
			log.Fatalf("--tls-certs: %v", err)
		}
		cfg, err := tlsutil.NewMultiFileConfig(pairs)
		if err != nil {
			log.Fatalf("Loading TLS certificates: %v", err)
		}
		tlsCfg = cfg
		turnTLSCfg = tlsCfg
		log.Printf("TLS: SNI-based selection across %d certificate(s) for HTTPS+TURNS", len(pairs))
	} else if *certFile != "" && *keyFile != "" {
		// Self-managed single-domain: binds both HTTPS (443) and TURNS (5349).
		cfg, err := tlsutil.NewFileConfig(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("Loading TLS certificate: %v", err)
		}
		tlsCfg = cfg
		turnTLSCfg = tlsCfg
		log.Printf("TLS: watching certificate file for renewal (%s)", *certFile)
	}

	// --turn-tls-certs overrides turnTLSCfg only — for behind-proxy deployments
	// where Caddy owns HTTPS but TURNS needs its own TLS listeners.
	if *turnTLSCerts != "" {
		pairs, err := parseCertPairs(*turnTLSCerts)
		if err != nil {
			log.Fatalf("--turn-tls-certs: %v", err)
		}
		cfg, err := tlsutil.NewMultiFileConfig(pairs)
		if err != nil {
			log.Fatalf("Loading TURNS certificates: %v", err)
		}
		turnTLSCfg = cfg
		log.Printf("TURNS: SNI-based selection across %d certificate(s), watching for renewal", len(pairs))
	}

	turnCfg := turnsvc.LoadConfigFromEnv()
	turnCredentials := turnsvc.NewCredentialManager(turnCfg.Realm, turnCfg.CredentialTTL)
	// turnTLSCfg is nil when no cert is configured; TURNS listeners are then
	// skipped with a "degraded" status rather than failing.
	turnService := turnsvc.NewService(turnCfg, turnCredentials, turnTLSCfg)
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

	// WireGuard service (opt-in via WG_ENABLED=true).
	// The handler is always registered so the route exists; it returns 503 when
	// the service is disabled, which clients interpret as SKIPPED.
	wgCfg := wgsvc.LoadConfigFromEnv()
	wgHandler := &handler.WireGuardCredentialsHandler{Token: os.Getenv("TOKEN")}
	if wgCfg.Enabled {
		wgService, err := wgsvc.NewService(wgCfg)
		if err != nil {
			log.Fatalf("failed to create WireGuard service: %v", err)
		}
		if err := wgService.Start(); err != nil {
			log.Fatalf("failed to start WireGuard service: %v", err)
		}
		defer wgService.Stop()
		wgHandler.Sessions = wgService.Sessions()
		wgHandler.Service = wgService
		log.Printf("WireGuard service enabled on UDP port %d", wgCfg.Port)
	}

	// Populate the plain-HTTP mux now that turnHandler is available.
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
	httpMux.Handle("/wireguard/credentials", wgHandler)

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
	tlsMux.Handle("/wireguard/credentials", wgHandler)

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

// parseCertPairs parses a comma-separated list of "certfile:keyfile" pairs.
// Colons inside Windows absolute paths (C:\...) are handled by splitting on
// the last colon that is preceded by more than one character.
func parseCertPairs(s string) ([]tlsutil.CertKeyPair, error) {
	var pairs []tlsutil.CertKeyPair
	for _, entry := range strings.Split(s, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		// Split on the last colon to handle paths like /a/b.crt:/a/b.key
		idx := strings.LastIndex(entry, ":")
		if idx <= 0 {
			return nil, fmt.Errorf("invalid cert:key pair %q (expected colon separator)", entry)
		}
		pairs = append(pairs, tlsutil.CertKeyPair{
			CertFile: entry[:idx],
			KeyFile:  entry[idx+1:],
		})
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("no cert:key pairs found in %q", s)
	}
	return pairs, nil
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
