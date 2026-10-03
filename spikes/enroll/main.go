// Command enroll is a throwaway spike for ADR-0004. It performs the server-side
// half of device enrollment against the real step-ca: it generates a device key
// and CSR (normally done by the agent), mints a JWK one-time token for the
// configured provisioner and asks step-ca to sign the CSR.
//
// Configuration comes only from the environment (never from reading .env):
// STEP_CA_JWK, STEP_CA_PASSWORD, STEP_CA_PROVISIONER, STEP_CA_PEM and
// STEP_CA_PROVIDER_URL. Secret values are never printed.
//
// Usage:
//
//	go run ./spikes/enroll -env .env -out /path/to/dir -ttl 4320h
package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.step.sm/crypto/jose"
	"go.step.sm/crypto/randutil"
)

// ottClaims are the claims step-ca's JWK provisioner expects in a sign token.
type ottClaims struct {
	jose.Claims
	SANs []string `json:"sans,omitempty"`
}

// signRequest mirrors step-ca's POST /1.0/sign body.
type signRequest struct {
	CSR      string `json:"csr"`
	OTT      string `json:"ott"`
	NotAfter string `json:"notAfter,omitempty"`
}

// signResponse holds the fields of step-ca's sign response used here.
type signResponse struct {
	CertChain []string `json:"certChain"`
}

func main() {
	out := flag.String("out", "", "directory to write device.key, device.crt and ca.crt into (optional)")
	ttl := flag.String("ttl", "24h", "requested certificate lifetime (step-ca notAfter)")
	audience := flag.String("aud", "", "token audience; defaults to <STEP_CA_PROVIDER_URL>/1.0/sign")
	envFile := flag.String("env", "", "optional dotenv file loaded into the environment (values are never printed)")
	flag.Parse()

	if *envFile != "" {
		if err := godotenv.Load(*envFile); err != nil {
			log.Fatalf("load %s: %v", *envFile, err)
		}
	}

	caURL := strings.TrimRight(mustEnv("STEP_CA_PROVIDER_URL"), "/")
	provisioner := mustEnv("STEP_CA_PROVISIONER")
	if *audience == "" {
		*audience = caURL + "/1.0/sign"
	}

	jwk, err := jose.ParseKey([]byte(mustEnv("STEP_CA_JWK")), jose.WithPassword([]byte(mustEnv("STEP_CA_PASSWORD"))))
	if err != nil {
		log.Fatalf("parse provisioner key (STEP_CA_JWK/STEP_CA_PASSWORD): %v", err)
	}
	roots, err := loadRoots(mustEnv("STEP_CA_PEM"))
	if err != nil {
		log.Fatalf("load STEP_CA_PEM: %v", err)
	}

	// Agent side: key + CSR with no subject and no SANs. Identity is decided by the server.
	deviceKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, deviceKey)
	if err != nil {
		log.Fatal(err)
	}

	// Server side: device identity, OTT, sign.
	deviceID, err := randutil.UUIDv4()
	if err != nil {
		log.Fatal(err)
	}
	ott, err := mintOTT(jwk, provisioner, *audience, deviceID)
	if err != nil {
		log.Fatalf("mint OTT: %v", err)
	}
	chain, err := sign(caURL, roots, signRequest{
		CSR:      string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})),
		OTT:      ott,
		NotAfter: *ttl,
	})
	if err != nil {
		log.Fatalf("sign: %v", err)
	}

	leaf, err := parseCert(chain[0])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("device id : %s\n", deviceID)
	fmt.Printf("subject   : %s\n", leaf.Subject)
	fmt.Printf("issuer    : %s\n", leaf.Issuer)
	fmt.Printf("uri sans  : %v\n", leaf.URIs)
	fmt.Printf("dns sans  : %v\n", leaf.DNSNames)
	fmt.Printf("validity  : %s → %s (%s)\n", leaf.NotBefore.Format(time.RFC3339), leaf.NotAfter.Format(time.RFC3339), leaf.NotAfter.Sub(leaf.NotBefore).Round(time.Minute))
	fmt.Printf("ext usage : %v\n", leaf.ExtKeyUsage)

	if *out != "" {
		if err := writeFiles(*out, deviceKey, chain, roots); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("written   : %s/{device.key,device.crt,ca.crt}\n", *out)
	}
}

// mintOTT creates a 5-minute one-time token authorizing a certificate for deviceID.
func mintOTT(jwk *jose.JSONWebKey, provisioner, audience, deviceID string) (string, error) {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.SignatureAlgorithm(jwk.Algorithm), Key: jwk.Key},
		new(jose.SignerOptions).WithType("JWT").WithHeader("kid", jwk.KeyID),
	)
	if err != nil {
		return "", err
	}
	jti, err := randutil.Hex(32)
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims := ottClaims{
		Claims: jose.Claims{
			ID:        jti,
			Issuer:    provisioner,
			Subject:   deviceID,
			Audience:  jose.Audience{audience},
			IssuedAt:  jose.NewNumericDate(now),
			NotBefore: jose.NewNumericDate(now),
			Expiry:    jose.NewNumericDate(now.Add(5 * time.Minute)),
		},
		SANs: []string{"ondota:device:" + deviceID},
	}
	return jose.Signed(signer).Claims(claims).CompactSerialize()
}

// sign posts the request to step-ca and returns the PEM certificate chain.
func sign(caURL string, roots *x509.CertPool, req signRequest) ([]string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}},
	}
	resp, err := client.Post(caURL+"/1.0/sign", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("step-ca returned %s: %s", resp.Status, raw)
	}
	var sr signResponse
	if err := json.Unmarshal(raw, &sr); err != nil {
		return nil, err
	}
	if len(sr.CertChain) == 0 {
		return nil, errors.New("empty certificate chain")
	}
	return sr.CertChain, nil
}

// loadRoots accepts either PEM content or a path to a PEM file.
func loadRoots(v string) (*x509.CertPool, error) {
	v = strings.ReplaceAll(v, `\n`, "\n") // tolerate escaped newlines in single-line env values
	data := []byte(v)
	if !strings.Contains(v, "-----BEGIN") {
		var err error
		if data, err = os.ReadFile(v); err != nil {
			return nil, err
		}
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, errors.New("no certificates found")
	}
	return pool, nil
}

func parseCert(p string) (*x509.Certificate, error) {
	b, _ := pem.Decode([]byte(p))
	if b == nil {
		return nil, errors.New("invalid certificate PEM")
	}
	return x509.ParseCertificate(b.Bytes)
}

func writeFiles(dir string, key *ecdsa.PrivateKey, chain []string, _ *x509.CertPool) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "device.key"), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "device.crt"), []byte(strings.Join(chain, "")), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "ca.crt"), []byte(strings.ReplaceAll(os.Getenv("STEP_CA_PEM"), `\n`, "\n")), 0o644)
}

func mustEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("missing env %s", name)
	}
	return v
}
