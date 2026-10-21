// sign creates the release signing key and signs checksums.txt (used by the release workflow).
//
//	go run ./tools/sign -genkey                      # prints a new key pair, once
//	LENS_SIGNING_KEY=<private> go run ./tools/sign dist/checksums.txt   # writes checksums.txt.sig
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/codedb-io/codedb-lens/internal/update"
)

func main() {
	genkey := flag.Bool("genkey", false, "generate a new Ed25519 key pair")
	flag.Parse()
	if *genkey {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			fail(err)
		}
		fmt.Println("public  (paste into internal/update/update.go PublicKey):", base64.StdEncoding.EncodeToString(pub))
		fmt.Println("private (GitHub secret LENS_SIGNING_KEY, never commit):  ", base64.StdEncoding.EncodeToString(priv.Seed()))
		return
	}
	if flag.NArg() != 1 {
		fail(fmt.Errorf("usage: sign <file>"))
	}
	raw := strings.TrimSpace(os.Getenv("LENS_SIGNING_KEY"))
	if i := strings.LastIndex(raw, " "); i >= 0 { // tolerate the whole "private (...): <key>" line being pasted
		raw = raw[i+1:]
	}
	if raw == "" {
		fail(fmt.Errorf("LENS_SIGNING_KEY is empty (not set, or not passed to the sign command)"))
	}
	seed, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(seed) != ed25519.SeedSize {
		// Never print the value; length is enough to tell a wrong paste from a wrong key type.
		fail(fmt.Errorf("LENS_SIGNING_KEY must be a base64 Ed25519 seed (got %d characters, decoded %d bytes, want 32)", len(raw), len(seed)))
	}
	// A key that doesn't match the one built into Lens would publish releases `update` rejects forever.
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	if got := base64.StdEncoding.EncodeToString(pub); got != update.PublicKey {
		fail(fmt.Errorf("LENS_SIGNING_KEY does not match update.PublicKey (its public key is %s)", got))
	}
	data, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fail(err)
	}
	sig := ed25519.Sign(ed25519.NewKeyFromSeed(seed), data)
	if err := os.WriteFile(flag.Arg(0)+".sig", []byte(base64.StdEncoding.EncodeToString(sig)+"\n"), 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "sign:", err)
	os.Exit(1)
}
