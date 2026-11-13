package storage

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const (
	maxCertFileSize   = 10 << 20
	maxExtraRootCerts = 512
)

// extraCertFiles are well-known CA bundle locations that Go's pure-Go Linux
// loader does not check. Entware keeps its bundle under /opt, which is the
// common case on OpenWrt routers.
var extraCertFiles = []string{
	"/opt/etc/ssl/certs/ca-certificates.crt", // Entware
	"/opt/etc/ssl/cert.pem",                  // Entware (OpenSSL default)
	"/opt/etc/ssl/certs/ca-bundle.crt",       // Entware (curl)
	"/usr/local/etc/ssl/certs/ca-certificates.crt",
	"/usr/local/etc/ssl/cert.pem",
	"/usr/local/share/certs/ca-root-nss.crt", // FreeBSD
	"/etc/ssl/certs/ca-certificates.crt",
	"/etc/pki/tls/certs/ca-bundle.crt",
	"/etc/ssl/ca-bundle.pem",
	"/etc/pki/tls/cacert.pem",
	"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem",
	"/etc/ssl/cert.pem",
	"/etc/ca-certificates/extracted/tls-ca-bundle.pem",
	"/data/data/com.termux/files/usr/etc/tls/cert.pem", // Termux
}

var extraCertDirs = []string{
	"/opt/etc/ssl/certs",
	"/usr/local/etc/ssl/certs",
	"/usr/local/share/certs",
	"/etc/ssl/certs",
	"/etc/pki/tls/certs",
}

var (
	rootsOnce sync.Once
	rootsPool *x509.CertPool
)

// rootCAs returns a pool with the system roots plus well-known extra bundles.
// It returns nil on platforms where Go uses native certificate verification
// (Windows, macOS) so those keep their platform behaviour. An explicit
// SSL_CERT_FILE/SSL_CERT_DIR override is always honoured, on every platform.
func rootCAs() *x509.CertPool {
	file := os.Getenv("SSL_CERT_FILE")
	dirs := os.Getenv("SSL_CERT_DIR")
	if file != "" || dirs != "" {
		return loadOverrideRootCAs(file, dirs)
	}

	if !supportsExtraCertPaths() {
		return nil
	}

	rootsOnce.Do(func() {
		rootsPool = loadRootCAs(extraCertFiles, extraCertDirs)
	})
	return rootsPool
}

func supportsExtraCertPaths() bool {
	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly", "solaris", "aix":
		return true
	default:
		return false
	}
}

func loadRootCAs(files, dirs []string) *x509.CertPool {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}

	seen := make(map[[32]byte]struct{})
	for _, path := range files {
		appendCertFile(pool, path, seen)
	}
	for _, dir := range dirs {
		appendCertDir(pool, dir, seen)
	}

	return pool
}

func loadOverrideRootCAs(file, dirList string) *x509.CertPool {
	pool := x509.NewCertPool()
	seen := make(map[[32]byte]struct{})

	if file != "" {
		appendCertFile(pool, file, seen)
	}
	for _, dir := range filepath.SplitList(dirList) {
		if dir != "" {
			appendCertDir(pool, dir, seen)
		}
	}

	return pool
}

func appendCertDir(pool *x509.CertPool, dir string, seen map[[32]byte]struct{}) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		appendCertFile(pool, filepath.Join(dir, entry.Name()), seen)
	}
}

func appendCertFile(pool *x509.CertPool, path string, seen map[[32]byte]struct{}) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > maxCertFileSize {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	appendPEM(pool, data, seen)
}

func appendPEM(pool *x509.CertPool, data []byte, seen map[[32]byte]struct{}) int {
	added := 0

	for len(seen) < maxExtraRootCerts {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}

		sum := sha256.Sum256(block.Bytes)
		if _, ok := seen[sum]; ok {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}

		pool.AddCert(cert)
		seen[sum] = struct{}{}
		added++
	}

	return added
}
