package e2e_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const e2eSharedBlueprint = `
actor CustomerActor {
  id: uuid
  role: string
  is_verified: bool
}

enum OrderStatus {
  pending
  confirmed
  cancelled
}

value PlaceOrderInput {
  product_id: uuid
  quantity: int
}

entity Product {
  id: uuid @pk
  name: string
  sku: string @unique
  price: int
  stock: int = 0
}

entity Order {
  id: uuid @pk
  customer_id: uuid
  product_id: uuid
  quantity: int
  total: int
  status: OrderStatus = .pending
}

error ProductNotFound {
  kind: not_found
  message: "product not found"
}

error InsufficientStock {
  kind: conflict
  message: "insufficient stock"
}

error UnauthorizedCustomer {
  kind: forbidden
  message: "customer is not authorized"
}

policy CanPlaceOrder(actor: Actor) {
  actor.role == "customer" && actor.is_verified == true
}

action CreateProduct(name: string, sku: string, price: int, stock: int) -> Product
  @http(method: POST, path: "/v1/products")
{
  let p = create Product {
    name: name,
    sku: sku,
    price: price,
    stock: stock
  }
  return p
}

action PlaceOrder(input: PlaceOrderInput) -> Order
  @http(method: POST, path: "/v1/orders")
{
  require CanPlaceOrder(actor) else UnauthorizedCustomer

  let p = query one Product where id == input.product_id else ProductNotFound
  guard p.stock >= input.quantity else InsufficientStock

  update Product where id == p.id set {
    stock: p.stock - input.quantity
  }

  let total = p.price * input.quantity
  let o = create Order {
    customer_id: actor.id,
    product_id: p.id,
    quantity: input.quantity,
    total: total,
    status: .confirmed
  }
  return o
}

action ListOrders(cust_id: uuid, cursor: string) -> []Order
  @http(method: GET, path: "/v1/orders")
{
  let orders = query many Order
    where customer_id == cust_id
    order_by id asc
    limit 2
    cursor cursor
  return orders
}
`

// jwksFixture mints RS256 tokens and publishes the matching key set over loopback, so the
// generated servers under test verify real signatures instead of trusting token text.
type jwksFixture struct {
	signer *rsa.PrivateKey
	kid    string
	url    string
}

func newJWKSFixture(t *testing.T) *jwksFixture {
	t.Helper()

	signer, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate test signing key: %v", err)
	}
	digest := sha256.Sum256(signer.PublicKey.N.Bytes())
	kid := "e2e-" + base64.RawURLEncoding.EncodeToString(digest[:6])

	doc, err := json.Marshal(map[string]any{
		"keys": []map[string]any{{
			"kty": "RSA",
			"kid": kid,
			"alg": "RS256",
			"use": "sig",
			"n":   base64.RawURLEncoding.EncodeToString(signer.PublicKey.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(signer.PublicKey.E)).Bytes()),
		}},
	})
	if err != nil {
		t.Fatalf("marshal jwks: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(doc)
	}))
	t.Cleanup(srv.Close)

	return &jwksFixture{signer: signer, kid: kid, url: srv.URL}
}

// env is the environment a generated server needs in order to verify against this key set.
func (f *jwksFixture) env() string {
	return "AUTH_JWKS_URL=" + f.url
}

// token signs a claim set the way an identity provider would.
func (f *jwksFixture) token(t *testing.T, sub, role string, verified bool) string {
	t.Helper()

	header := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"alg":"RS256","typ":"JWT","kid":%q}`, f.kid)))
	claims, err := json.Marshal(map[string]any{
		"sub":         sub,
		"role":        role,
		"is_verified": verified,
		"exp":         time.Now().Add(time.Hour).Unix(),
		"iat":         time.Now().Add(-time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}

	signingInput := header + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, f.signer, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

const (
	testValidActorID      = "00000000-0000-0000-0000-000000000001"
	testUnverifiedActorID = "00000000-0000-0000-0000-000000000002"
)

// tokens returns a token whose actor satisfies the suite's policy and one that does not, both
// validly signed so a rejection is attributable to the policy rather than the signature.
func (f *jwksFixture) tokens(t *testing.T) (string, string) {
	t.Helper()
	return f.token(t, testValidActorID, "customer", true),
		f.token(t, testUnverifiedActorID, "customer", false)
}

func getFreePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitForPort(t *testing.T, port int) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server on %s failed to become ready within deadline", addr)
}

func buildAndStartGoServer(t *testing.T, targetDir, jwksEnv string) (string, func()) {
	binPath := filepath.Join(targetDir, "server-bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	buildCmd.Dir = targetDir
	buildCmd.Env = append(os.Environ(), "GOPROXY=off")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build in %s failed: %v\n%s", targetDir, err, string(out))
	}

	port := getFreePort(t)
	srvCmd := exec.Command(binPath)
	srvCmd.Dir = targetDir
	srvCmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port), jwksEnv)
	if err := srvCmd.Start(); err != nil {
		t.Fatalf("failed to start Go server binary: %v", err)
	}

	cleanup := func() {
		if srvCmd.Process != nil {
			_ = srvCmd.Process.Signal(syscall.SIGTERM)
			_ = srvCmd.Wait()
		}
	}

	waitForPort(t, port)
	return fmt.Sprintf("http://127.0.0.1:%d", port), cleanup
}

func startTSServer(t *testing.T, targetDir, jwksEnv string) (string, func()) {
	// Install express/pg dependencies
	npmCmd := exec.Command("npm", "install", "--prefer-offline", "--no-audit", "--no-fund", "--silent")
	npmCmd.Dir = targetDir
	if out, err := npmCmd.CombinedOutput(); err != nil {
		t.Fatalf("npm install in %s failed: %v\n%s", targetDir, err, string(out))
	}

	port := getFreePort(t)
	srvCmd := exec.Command("node", "--experimental-strip-types", "src/main/server.ts")
	srvCmd.Dir = targetDir
	srvCmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port), jwksEnv)
	if err := srvCmd.Start(); err != nil {
		t.Fatalf("failed to start TypeScript server: %v", err)
	}

	cleanup := func() {
		if srvCmd.Process != nil {
			_ = srvCmd.Process.Signal(syscall.SIGTERM)
			_ = srvCmd.Wait()
		}
	}

	waitForPort(t, port)
	return fmt.Sprintf("http://127.0.0.1:%d", port), cleanup
}
